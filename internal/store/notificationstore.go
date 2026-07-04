package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/* -------------------- Notification CRUD -------------------- */

// CreateNotification inserts a single notification.
// Uses ON CONFLICT DO NOTHING on the dedup index as a safety net.
func (s *Store) CreateNotification(ctx context.Context, n *models.Notification) error {
	return s.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "dedup_key"}},
			DoNothing: true,
		}).
		Create(n).Error
}

// BulkCreateNotifications inserts multiple notifications in batches of 100.
func (s *Store) BulkCreateNotifications(ctx context.Context, notifications []models.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	return s.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "dedup_key"}},
			DoNothing: true,
		}).
		CreateInBatches(notifications, 100).Error
}

// GetNotificationsByUserID returns notifications for a user, unread first then by newest.
func (s *Store) GetNotificationsByUserID(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error) {
	var notifications []models.Notification
	query := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_read ASC, created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

// GetUnreadNotificationCount returns the number of unread notifications for a user.
func (s *Store) GetUnreadNotificationCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := s.DB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Count(&count).Error
	return count, err
}

// MarkNotificationAsRead marks a single notification as read.
func (s *Store) MarkNotificationAsRead(ctx context.Context, notificationID, userID string) error {
	now := time.Now()
	result := s.DB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkAllNotificationsAsRead marks all unread notifications as read for a user.
func (s *Store) MarkAllNotificationsAsRead(ctx context.Context, userID string) error {
	now := time.Now()
	return s.DB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false", userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		}).Error
}

/* -------------------- Dedup helpers -------------------- */

// GetExistingDedupKeys returns a set of "userID|dedupKey" strings for all
// notifications of a given type. Used for efficient batch dedup checks.
func (s *Store) GetExistingDedupKeys(ctx context.Context, notificationType string) (map[string]bool, error) {
	var results []struct {
		UserID   string
		DedupKey string
	}
	if err := s.DB.WithContext(ctx).
		Model(&models.Notification{}).
		Select("user_id, dedup_key").
		Where("type = ?", notificationType).
		Scan(&results).Error; err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(results))
	for _, r := range results {
		m[r.UserID+"|"+r.DedupKey] = true
	}
	return m, nil
}

/* -------------------- Notification Config CRUD -------------------- */

// GetNotificationConfig returns a single config entry by type and key.
func (s *Store) GetNotificationConfig(ctx context.Context, notificationType, key string) (*models.NotificationConfig, error) {
	var config models.NotificationConfig
	if err := s.DB.WithContext(ctx).
		Where("type = ? AND key = ?", notificationType, key).
		First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// ListNotificationConfigs returns all notification config entries.
func (s *Store) ListNotificationConfigs(ctx context.Context) ([]models.NotificationConfig, error) {
	var configs []models.NotificationConfig
	if err := s.DB.WithContext(ctx).
		Order("type ASC, key ASC").
		Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// CreateNotificationConfig inserts a new config entry.
func (s *Store) CreateNotificationConfig(ctx context.Context, config *models.NotificationConfig) error {
	config.UpdatedAt = time.Now()
	return s.DB.WithContext(ctx).Create(config).Error
}

// UpdateNotificationConfig updates specific fields of a config entry.
func (s *Store) UpdateNotificationConfig(ctx context.Context, id uint, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	result := s.DB.WithContext(ctx).
		Model(&models.NotificationConfig{}).
		Where("id = ?", id).
		Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteNotificationConfig deletes a config entry by ID.
func (s *Store) DeleteNotificationConfig(ctx context.Context, id uint) error {
	result := s.DB.WithContext(ctx).Delete(&models.NotificationConfig{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

/* -------------------- Helpers for notification generation -------------------- */

// StudentMaxRating holds the maximum rating for a student on a specific platform.
type StudentMaxRating struct {
	UserID    string
	Platform  string
	MaxRating int
}

// GetMaxRatingPerStudentPlatform returns the highest rating per student per platform.
func (s *Store) GetMaxRatingPerStudentPlatform(ctx context.Context) ([]StudentMaxRating, error) {
	var results []StudentMaxRating
	err := s.DB.WithContext(ctx).
		Model(&models.RatingHistory{}).
		Select("user_id, platform, MAX(rating) as max_rating").
		Group("user_id, platform").
		Scan(&results).Error
	return results, err
}

// GetRatingRecordsCreatedSince returns rating records created after a given time for specific platforms.
func (s *Store) GetRatingRecordsCreatedSince(ctx context.Context, since time.Time, platforms []string) ([]models.RatingHistory, error) {
	var records []models.RatingHistory
	err := s.DB.WithContext(ctx).
		Where("created_at >= ? AND platform IN ?", since, platforms).
		Order("user_id, platform, recorded_at").
		Find(&records).Error
	return records, err
}

// GetActiveCoachesAndMentors returns all active, approved coaches and mentors.
func (s *Store) GetActiveCoachesAndMentors(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := s.DB.WithContext(ctx).
		Where("role IN ? AND active = ? AND approved = ?",
			[]string{string(models.RoleCoach), string(models.RoleMentor)}, true, true).
		Find(&users).Error
	return users, err
}

// GetUserNameMap returns a map of user ID → "FirstName LastName" for all users.
func (s *Store) GetUserNameMap(ctx context.Context) (map[string]string, error) {
	var users []models.User
	if err := s.DB.WithContext(ctx).Select("id, first_name, last_name").Find(&users).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.ID] = u.FirstName + " " + u.LastName
	}
	return m, nil
}

// SeedDefaultNotificationConfigs inserts default notification configs if they don't exist.
func (s *Store) SeedDefaultNotificationConfigs(ctx context.Context) error {
	defaults := []models.NotificationConfig{
		{
			Type:        models.NotificationTypeRatingMilestone,
			Key:         "milestones",
			Value:       "[500,1000,1500,2000,2500]",
			Description: "Rating thresholds that trigger milestone notifications",
		},
		{
			Type:        models.NotificationTypeJoiningAnniversary,
			Key:         "milestones_months",
			Value:       "[6,12,24,36,48,60]",
			Description: "Months after joining to trigger anniversary notifications",
		},
		{
			Type:        models.NotificationTypeTournamentPlayed,
			Key:         "enabled_platforms",
			Value:       "[\"uscf\",\"fide\"]",
			Description: "Platforms to check for tournament participation (new rating records indicate tournament play)",
		},
	}

	for i := range defaults {
		var count int64
		s.DB.WithContext(ctx).
			Model(&models.NotificationConfig{}).
			Where("type = ? AND key = ?", defaults[i].Type, defaults[i].Key).
			Count(&count)
		if count == 0 {
			defaults[i].UpdatedAt = time.Now()
			if err := s.DB.WithContext(ctx).Create(&defaults[i]).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
