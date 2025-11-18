package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
)
func (s *Store) ApproveUser(ctx context.Context, userID string) error {
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{"approved": true, "updated_at": time.Now()}).Error
}

func (s *Store) ChangeUserRole(ctx context.Context, userID, role string) error {
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("role", role).Error
}

func (s *Store) AddCoachStudent(ctx context.Context, coachID, studentID, mentorID string) error {
	cs := models.CoachStudent{CoachID: coachID, StudentID: studentID, MentorCoachID: mentorID}
	return s.DB.WithContext(ctx).Create(&cs).Error
}
func (s *Store) RemoveCoachStudent(ctx context.Context, coachID, studentID string) error {
	return s.DB.WithContext(ctx).Where("coach_id = ? AND student_id = ?", coachID, studentID).Delete(&models.CoachStudent{}).Error
}