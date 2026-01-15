package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/* ------------------ User CRUD ------------------ */

func (s *Store) CreateUser(ctx context.Context, u *models.User, ud *models.UserDetails) error {
	// create user and empty details in a transaction
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		ud.AdditionalInfo = map[string]interface{}{}
		if err := tx.Create(ud).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateUserFields(ctx context.Context, id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Store) UpdateUserDetailsFields(ctx context.Context, id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	tx := s.DB.WithContext(ctx)
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserDetails{UserID: id}).Error; err != nil {
		// ignore if already exists; continue
	}
	return tx.Model(&models.UserDetails{}).Where("user_id = ?", id).Updates(fields).Error
}

func (s *Store) ListUsersAdmin(ctx context.Context) ([]*models.User, error) {
	var res []*models.User
	if err := s.DB.WithContext(ctx).Order("created_at desc").Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Store) ListStudentsForCoachOrMentor(ctx context.Context, userID string) ([]*models.User, error) {
	var students []models.User
	err := s.DB.WithContext(ctx).
		Table("users").
		Select("users.*").
		Joins("JOIN coach_students cs ON cs.student_id = users.id").
		Where("cs.coach_id = ? OR cs.mentor_coach_id = ?", userID, userID).
		Order("users.created_at DESC").
		Find(&students).Error
	if err != nil {
		return nil, err
	}
	out := make([]*models.User, len(students))
	for i := range students {
		out[i] = &students[i]
	}
	return out, nil
}

func (s *Store) GetCoachesByStudentID(ctx context.Context, studentID string) (string, string, error) {
	var result struct {
		CoachID       string `gorm:"column:coach_id"`
		MentorCoachID string `gorm:"column:mentor_coach_id"`
	}

	if err := s.DB.WithContext(ctx).
		Table("coach_students").
		Select("coach_id, mentor_coach_id").
		Where("student_id = ?", studentID).
		First(&result).Error; err != nil {
		return "", "", err
	}

	return result.CoachID, result.MentorCoachID, nil
}
