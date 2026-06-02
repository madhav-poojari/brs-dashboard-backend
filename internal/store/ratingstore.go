package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
)

// CreateRatingRecord inserts a single rating history record.
func (s *Store) CreateRatingRecord(ctx context.Context, rh *models.RatingHistory) error {
	return s.DB.WithContext(ctx).Create(rh).Error
}

// BulkCreateRatingRecords inserts multiple rating history records in batches of 100.
func (s *Store) BulkCreateRatingRecords(ctx context.Context, records []models.RatingHistory) error {
	if len(records) == 0 {
		return nil
	}
	return s.DB.WithContext(ctx).CreateInBatches(records, 100).Error
}

// GetLatestRating returns the most recent rating record for a user on a given platform.
func (s *Store) GetLatestRating(ctx context.Context, userID, platform string) (*models.RatingHistory, error) {
	var rh models.RatingHistory
	err := s.DB.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		Order("recorded_at DESC").
		First(&rh).Error
	if err != nil {
		return nil, err
	}
	return &rh, nil
}

// HasRatingHistory checks if any rating records exist for a user on a given platform.
func (s *Store) HasRatingHistory(ctx context.Context, userID, platform string) (bool, error) {
	var count int64
	err := s.DB.WithContext(ctx).
		Model(&models.RatingHistory{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetRatingHistory returns all rating records for a user on a given platform, ordered by recorded_at.
func (s *Store) GetRatingHistory(ctx context.Context, userID, platform string) ([]models.RatingHistory, error) {
	var records []models.RatingHistory
	err := s.DB.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		Order("recorded_at ASC").
		Find(&records).Error
	return records, err
}

// StudentPlatformInfo holds user ID and the relevant platform username/ID.
type StudentPlatformInfo struct {
	UserID   string
	Username string // chesscom_username, lichess_username, or fide_id
}

// GetStudentsWithPlatformUsername returns active students who have a non-empty
// username/ID for the given platform. Platform must be "chesscom", "lichess", or "fide".
func (s *Store) GetStudentsWithPlatformUsername(ctx context.Context, platform string) ([]StudentPlatformInfo, error) {
	var column string
	switch platform {
	case "chesscom":
		column = "chesscom_username"
	case "lichess":
		column = "lichess_username"
	case "fide":
		column = "fide_id"
	case "uscf":
		column = "uscf_id"
	default:
		return nil, nil
	}

	var results []StudentPlatformInfo
	err := s.DB.WithContext(ctx).
		Table("users").
		Select("users.id as user_id, user_details."+column+" as username").
		Joins("JOIN user_details ON user_details.user_id = users.id").
		Where("users.role = ? AND users.active = ? AND users.approved = ? AND user_details."+column+" != ''",
			models.RoleStudent, true, true).
		Scan(&results).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return results, nil
}

// GetExistingRatingDates returns a set of "YYYY-MM-DD" date strings for all
// existing rating records for a user on a given platform.
// Used by the USCF extension upload to skip duplicate tournament dates.
func (s *Store) GetExistingRatingDates(ctx context.Context, userID, platform string) (map[string]bool, error) {
	var dates []time.Time
	err := s.DB.WithContext(ctx).
		Model(&models.RatingHistory{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Pluck("recorded_at", &dates).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		set[d.Format("2006-01-02")] = true
	}
	return set, nil
}
