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
