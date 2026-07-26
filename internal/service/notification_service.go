package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const notificationTimeout = 5 * time.Minute

// platformDisplayName returns a human-friendly name for a rating platform.
func platformDisplayName(platform string) string {
	switch platform {
	case "chesscom":
		return "Chess.com"
	case "lichess":
		return "Lichess"
	case "fide":
		return "FIDE"
	case "uscf":
		return "USCF"
	default:
		return platform
	}
}

// CheckRatingMilestones checks all students' max ratings against configured
// milestones and creates notifications for their coaches and mentors.
func CheckRatingMilestones(s *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	log.Println("[Notifications] Checking rating milestones...")

	// 1. Get milestone config
	config, err := s.GetNotificationConfig(ctx, models.NotificationTypeRatingMilestone, "milestones")
	if err != nil {
		log.Printf("[Notifications] Error getting milestone config: %v", err)
		return
	}
	var milestones []int
	if err := json.Unmarshal([]byte(config.Value), &milestones); err != nil {
		log.Printf("[Notifications] Error parsing milestones: %v", err)
		return
	}
	sort.Ints(milestones)

	// 2. Get max rating per student per platform
	maxRatings, err := s.GetMaxRatingPerStudentPlatform(ctx)
	if err != nil {
		log.Printf("[Notifications] Error getting max ratings: %v", err)
		return
	}

	// 3. Get existing dedup keys in one batch query
	existingKeys, err := s.GetExistingDedupKeys(ctx, models.NotificationTypeRatingMilestone)
	if err != nil {
		log.Printf("[Notifications] Error getting existing dedup keys: %v", err)
		return
	}

	// 4. Get name map for notification messages
	nameMap, err := s.GetUserNameMap(ctx)
	if err != nil {
		log.Printf("[Notifications] Error getting name map: %v", err)
		return
	}

	// 5. Build notifications
	var toCreate []models.Notification

	for _, mr := range maxRatings {
		coachID, mentorID, err := s.GetCoachesByStudentID(ctx, mr.UserID)
		if err != nil {
			continue // student has no relation entry
		}

		recipients := collectRecipients(coachID, mentorID)
		if len(recipients) == 0 {
			continue
		}

		studentName := nameMap[mr.UserID]

		for _, milestone := range milestones {
			if mr.MaxRating < milestone {
				continue
			}

			dedupKey := fmt.Sprintf("rm:%s:%s:%d", mr.UserID, mr.Platform, milestone)

			for _, recipientID := range recipients {
				key := recipientID + "|" + dedupKey
				if existingKeys[key] {
					continue
				}

				toCreate = append(toCreate, models.Notification{
					UserID:  recipientID,
					Type:    models.NotificationTypeRatingMilestone,
					Title:   "Rating Milestone! 🏆",
					Message: fmt.Sprintf("%s reached %d on %s!", studentName, milestone, platformDisplayName(mr.Platform)),
					Metadata: map[string]interface{}{
						"student_id":   mr.UserID,
						"student_name": studentName,
						"platform":     mr.Platform,
						"milestone":    milestone,
						"rating":       mr.MaxRating,
					},
					DedupKey: dedupKey,
				})
				existingKeys[key] = true
			}
		}
	}

	// 6. Bulk insert
	if len(toCreate) > 0 {
		if err := s.BulkCreateNotifications(ctx, toCreate); err != nil {
			log.Printf("[Notifications] Error creating milestone notifications: %v", err)
			return
		}
		log.Printf("[Notifications] Created %d rating milestone notifications", len(toCreate))
	} else {
		log.Println("[Notifications] No new rating milestone notifications")
	}
}

// CheckTournamentParticipation detects new USCF/FIDE rating records
// (which indicate tournament play) and notifies coaches/mentors.
func CheckTournamentParticipation(s *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	log.Println("[Notifications] Checking tournament participation...")

	// 1. Get enabled platforms from config
	config, err := s.GetNotificationConfig(ctx, models.NotificationTypeTournamentPlayed, "enabled_platforms")
	if err != nil {
		log.Printf("[Notifications] Error getting tournament config: %v", err)
		return
	}
	var platforms []string
	if err := json.Unmarshal([]byte(config.Value), &platforms); err != nil {
		log.Printf("[Notifications] Error parsing platforms: %v", err)
		return
	}

	// 2. Get rating records created in the last 7 days
	since := time.Now().AddDate(0, 0, -7)
	records, err := s.GetRatingRecordsCreatedSince(ctx, since, platforms)
	if err != nil {
		log.Printf("[Notifications] Error getting recent records: %v", err)
		return
	}

	if len(records) == 0 {
		log.Println("[Notifications] No recent tournament records found")
		return
	}

	// 3. Get existing dedup keys
	existingKeys, err := s.GetExistingDedupKeys(ctx, models.NotificationTypeTournamentPlayed)
	if err != nil {
		log.Printf("[Notifications] Error getting existing dedup keys: %v", err)
		return
	}

	// 4. Get name map
	nameMap, err := s.GetUserNameMap(ctx)
	if err != nil {
		log.Printf("[Notifications] Error getting name map: %v", err)
		return
	}

	// 5. Build notifications
	var toCreate []models.Notification

	for _, rec := range records {
		dateStr := rec.RecordedAt.Format("2006-01-02")
		dedupKey := fmt.Sprintf("tp:%s:%s:%s", rec.UserID, rec.Platform, dateStr)

		coachID, mentorID, err := s.GetCoachesByStudentID(ctx, rec.UserID)
		if err != nil {
			continue
		}

		recipients := collectRecipients(coachID, mentorID)
		if len(recipients) == 0 {
			continue
		}

		studentName := nameMap[rec.UserID]
		platformName := platformDisplayName(rec.Platform)

		for _, recipientID := range recipients {
			key := recipientID + "|" + dedupKey
			if existingKeys[key] {
				continue
			}

			toCreate = append(toCreate, models.Notification{
				UserID:  recipientID,
				Type:    models.NotificationTypeTournamentPlayed,
				Title:   "Tournament Played ♟️",
				Message: fmt.Sprintf("%s played a %s tournament on %s", studentName, platformName, rec.RecordedAt.Format("Jan 2, 2006")),
				Metadata: map[string]interface{}{
					"student_id":      rec.UserID,
					"student_name":    studentName,
					"platform":        rec.Platform,
					"tournament_date": dateStr,
					"rating":          rec.Rating,
				},
				DedupKey: dedupKey,
			})
			existingKeys[key] = true
		}
	}

	// 6. Bulk insert
	if len(toCreate) > 0 {
		if err := s.BulkCreateNotifications(ctx, toCreate); err != nil {
			log.Printf("[Notifications] Error creating tournament notifications: %v", err)
			return
		}
		log.Printf("[Notifications] Created %d tournament participation notifications", len(toCreate))
	} else {
		log.Println("[Notifications] No new tournament participation notifications")
	}
}

// CheckJoiningAnniversaries creates self-notifications for coaches/mentors
// who have reached configured month milestones since joining.
func CheckJoiningAnniversaries(s *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	log.Println("[Notifications] Checking joining anniversaries...")

	// 1. Get anniversary milestone months from config
	config, err := s.GetNotificationConfig(ctx, models.NotificationTypeJoiningAnniversary, "milestones_months")
	if err != nil {
		log.Printf("[Notifications] Error getting anniversary config: %v", err)
		return
	}
	var months []int
	if err := json.Unmarshal([]byte(config.Value), &months); err != nil {
		log.Printf("[Notifications] Error parsing anniversary months: %v", err)
		return
	}
	sort.Ints(months)

	// 2. Get active coaches and mentors
	users, err := s.GetActiveCoachesAndMentors(ctx)
	if err != nil {
		log.Printf("[Notifications] Error getting coaches/mentors: %v", err)
		return
	}

	// 3. Get existing dedup keys
	existingKeys, err := s.GetExistingDedupKeys(ctx, models.NotificationTypeJoiningAnniversary)
	if err != nil {
		log.Printf("[Notifications] Error getting existing dedup keys: %v", err)
		return
	}

	// 4. Check each user against milestones
	now := time.Now()
	var toCreate []models.Notification

	for _, u := range users {
		elapsed := monthsBetween(u.CreatedAt, now)

		for _, m := range months {
			if elapsed < m {
				break // months is sorted; skip remaining larger milestones
			}

			dedupKey := fmt.Sprintf("ja:%s:%d", u.ID, m)
			key := u.ID + "|" + dedupKey
			if existingKeys[key] {
				continue
			}

			duration := formatDuration(m)

			toCreate = append(toCreate, models.Notification{
				UserID:  u.ID,
				Type:    models.NotificationTypeJoiningAnniversary,
				Title:   "Joining Anniversary! 🎉",
				Message: fmt.Sprintf("Congratulations! You've been with BRS Chess Academy for %s!", duration),
				Metadata: map[string]interface{}{
					"months": m,
				},
				DedupKey: dedupKey,
			})
			existingKeys[key] = true
		}
	}

	// 5. Bulk insert
	if len(toCreate) > 0 {
		if err := s.BulkCreateNotifications(ctx, toCreate); err != nil {
			log.Printf("[Notifications] Error creating anniversary notifications: %v", err)
			return
		}
		log.Printf("[Notifications] Created %d joining anniversary notifications", len(toCreate))
	} else {
		log.Println("[Notifications] No new joining anniversary notifications")
	}
}

/* -------------------- Helpers -------------------- */

// collectRecipients returns a de-duplicated list of non-empty recipient IDs.
func collectRecipients(ids ...string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// monthsBetween calculates the number of complete months between two dates.
func monthsBetween(start, end time.Time) int {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		months--
	}
	return years*12 + months
}

// formatDuration returns a human-friendly duration string from months.
func formatDuration(months int) string {
	if months < 12 {
		return fmt.Sprintf("%d months", months)
	}
	years := months / 12
	remaining := months % 12
	if remaining == 0 {
		if years == 1 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", years)
	}
	if years == 1 {
		return fmt.Sprintf("1 year and %d months", remaining)
	}
	return fmt.Sprintf("%d years and %d months", years, remaining)
}

// BackfillHistoricNotifications runs on startup to populate dedup_keys for all past events
// (rating milestones, tournaments, anniversaries) as read notifications.
// This prevents a notification storm when deploying to a database with existing data.
func BackfillHistoricNotifications(s *store.Store) error {
	ctx := context.Background()

	// 1. Transaction Guard: Check if backfill has already run
	var count int64
	err := s.DB.WithContext(ctx).
		Model(&models.NotificationConfig{}).
		Where("type = ? AND key = ? AND value = ?", "system", "backfill_completed", "true").
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("checking backfill status: %w", err)
	}
	if count > 0 {
		log.Println("[Startup] Silent backfill already completed, skipping.")
		return nil
	}

	log.Println("[Startup] Starting historic notifications backfill...")

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var notificationsToInsert []models.Notification

		// Get name map for proper notification messages
		var allUsers []models.User
		if err := tx.Select("id, first_name, last_name").Find(&allUsers).Error; err != nil {
			return fmt.Errorf("getting user names: %w", err)
		}
		nameMap := make(map[string]string, len(allUsers))
		for _, u := range allUsers {
			nameMap[u.ID] = u.FirstName + " " + u.LastName
		}

		// --- A. Backfill Rating Milestones ---
		var milestones []int
		var milConfig models.NotificationConfig
		if err := tx.Where("type = ? AND key = ?", models.NotificationTypeRatingMilestone, "milestones").First(&milConfig).Error; err == nil {
			_ = json.Unmarshal([]byte(milConfig.Value), &milestones)
		}
		sort.Ints(milestones)

		type StudentMaxRating struct {
			UserID    string
			Platform  string
			MaxRating int
		}
		var maxRatings []StudentMaxRating
		if err := tx.Model(&models.RatingHistory{}).
			Select("user_id, platform, MAX(rating) as max_rating").
			Group("user_id, platform").
			Scan(&maxRatings).Error; err == nil {

			for _, mr := range maxRatings {
				var rel models.Relation
				if err := tx.Where("user_id = ?", mr.UserID).First(&rel).Error; err == nil {
					recipients := collectRecipients(rel.CoachID, rel.MentorID)
					for _, milestone := range milestones {
						if mr.MaxRating >= milestone {
							dedupKey := fmt.Sprintf("rm:%s:%s:%d", mr.UserID, mr.Platform, milestone)
							for _, recipientID := range recipients {
								studentName := nameMap[mr.UserID]
								notificationsToInsert = append(notificationsToInsert, models.Notification{
									UserID:  recipientID,
									Type:    models.NotificationTypeRatingMilestone,
									Title:   "Rating Milestone! 🏆",
									Message: fmt.Sprintf("%s reached %d on %s!", studentName, milestone, platformDisplayName(mr.Platform)),
									Metadata: map[string]interface{}{
										"student_id":   mr.UserID,
										"student_name": studentName,
										"platform":     mr.Platform,
										"milestone":    milestone,
										"rating":       mr.MaxRating,
									},
									DedupKey:  dedupKey,
									IsRead:    true,
									ReadAt:    &now,
									CreatedAt: now,
								})
							}
						}
					}
				}
			}
		}

		// --- B. Backfill Tournaments ---
		var tournamentPlatforms []string
		var tpConfig models.NotificationConfig
		if err := tx.Where("type = ? AND key = ?", models.NotificationTypeTournamentPlayed, "enabled_platforms").First(&tpConfig).Error; err == nil {
			_ = json.Unmarshal([]byte(tpConfig.Value), &tournamentPlatforms)
		}

		if len(tournamentPlatforms) > 0 {
			var ratingRecords []models.RatingHistory
			if err := tx.Where("platform IN ?", tournamentPlatforms).Find(&ratingRecords).Error; err == nil {
				for _, rec := range ratingRecords {
					var rel models.Relation
					if err := tx.Where("user_id = ?", rec.UserID).First(&rel).Error; err == nil {
						recipients := collectRecipients(rel.CoachID, rel.MentorID)
						dateStr := rec.RecordedAt.Format("2006-01-02")
						dedupKey := fmt.Sprintf("tp:%s:%s:%s", rec.UserID, rec.Platform, dateStr)
						studentName := nameMap[rec.UserID]
						platformName := platformDisplayName(rec.Platform)
						for _, recipientID := range recipients {
							notificationsToInsert = append(notificationsToInsert, models.Notification{
								UserID:  recipientID,
								Type:    models.NotificationTypeTournamentPlayed,
								Title:   "Tournament Played ♟️",
								Message: fmt.Sprintf("%s played a %s tournament on %s", studentName, platformName, rec.RecordedAt.Format("Jan 2, 2006")),
								Metadata: map[string]interface{}{
									"student_id":      rec.UserID,
									"student_name":    studentName,
									"platform":        rec.Platform,
									"tournament_date": dateStr,
									"rating":          rec.Rating,
								},
								DedupKey:  dedupKey,
								IsRead:    true,
								ReadAt:    &now,
								CreatedAt: now,
							})
						}
					}
				}
			}
		}

		// --- C. Backfill Coach/Mentor Anniversaries ---
		var months []int
		var annConfig models.NotificationConfig
		if err := tx.Where("type = ? AND key = ?", models.NotificationTypeJoiningAnniversary, "milestones_months").First(&annConfig).Error; err == nil {
			_ = json.Unmarshal([]byte(annConfig.Value), &months)
		}
		sort.Ints(months)

		var users []models.User
		if err := tx.Where("role IN ? AND active = ? AND approved = ?", []string{string(models.RoleCoach), string(models.RoleMentor)}, true, true).Find(&users).Error; err == nil {
			for _, u := range users {
				elapsed := monthsBetween(u.CreatedAt, now)
				for _, m := range months {
					if elapsed >= m {
						dedupKey := fmt.Sprintf("ja:%s:%d", u.ID, m)
						notificationsToInsert = append(notificationsToInsert, models.Notification{
							UserID:    u.ID,
							Type:      models.NotificationTypeJoiningAnniversary,
							Title:     "Joining Anniversary! 🎉",
							Message:   fmt.Sprintf("Congratulations! You've been with BRS Chess Academy for %s!", formatDuration(m)),
							Metadata:  map[string]interface{}{"months": m},
							DedupKey:  dedupKey,
							IsRead:    true,
							ReadAt:    &now,
							CreatedAt: now,
						})
					}
				}
			}
		}

		// Insert notifications in batches of 100 with ON CONFLICT DO NOTHING
		if len(notificationsToInsert) > 0 {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "dedup_key"}},
				DoNothing: true,
			}).CreateInBatches(notificationsToInsert, 100).Error
			if err != nil {
				return fmt.Errorf("inserting backfill notifications: %w", err)
			}
			log.Printf("[Startup] Successfully backfilled %d historical notifications.", len(notificationsToInsert))
		}

		// Save completion marker config
		marker := models.NotificationConfig{
			Type:        "system",
			Key:         "backfill_completed",
			Value:       "true",
			Description: "Flags whether the startup silent notifications backfill has finished",
			UpdatedAt:   now,
		}
		if err := tx.Create(&marker).Error; err != nil {
			return fmt.Errorf("saving completion marker config: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("transaction execution: %w", err)
	}

	log.Println("[Startup] Historic notifications backfill complete.")
	return nil
}

