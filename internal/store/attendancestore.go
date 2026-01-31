package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
)

type AttendanceListFilter struct {
	StartDate  time.Time
	EndDate    time.Time
	StudentID  *string
	CoachID    *string
	ClassType  *models.AttendanceClassType
	SessionID  *string
	IsVerified *bool
	MentorID   *string // mentor-scoped listing (via relations)
}

func (s *Store) CreateAttendance(ctx context.Context, a *models.Attendance) error {
	return s.DB.WithContext(ctx).Create(a).Error
}

func (s *Store) GetAttendanceByID(ctx context.Context, id uint) (*models.Attendance, error) {
	var a models.Attendance
	if err := s.DB.WithContext(ctx).
		Preload("Student").Preload("Coach").
		First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) UpdateAttendanceByID(ctx context.Context, id uint, updates map[string]interface{}) (*models.Attendance, error) {
	updates["updated_at"] = time.Now()
	if err := s.DB.WithContext(ctx).Model(&models.Attendance{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetAttendanceByID(ctx, id)
}

func (s *Store) DeleteAttendanceByID(ctx context.Context, id uint) error {
	return s.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.Attendance{}).Error
}

func (s *Store) ListAttendances(ctx context.Context, f AttendanceListFilter) ([]*models.Attendance, error) {
	q := s.DB.WithContext(ctx).Model(&models.Attendance{}).
		Preload("Student").Preload("Coach").
		Where("date >= ? AND date < ?", f.StartDate, f.EndDate)

	if f.StudentID != nil && *f.StudentID != "" {
		q = q.Where("student_id = ?", *f.StudentID)
	}
	if f.CoachID != nil && *f.CoachID != "" {
		q = q.Where("coach_id = ?", *f.CoachID)
	}
	if f.ClassType != nil && *f.ClassType != "" {
		q = q.Where("class_type = ?", *f.ClassType)
	}
	if f.SessionID != nil && *f.SessionID != "" {
		q = q.Where("session_id = ?", *f.SessionID)
	}
	if f.IsVerified != nil {
		q = q.Where("is_verified = ?", *f.IsVerified)
	}

	// Mentor scoping:
	// - records for students assigned to mentor (relations.user_id = attendances.student_id)
	// - OR records taught by a coach assigned to mentor (relations.coach_id = attendances.coach_id)
	if f.MentorID != nil && *f.MentorID != "" {
		mentorID := *f.MentorID
		q = q.Where(
			`(
				EXISTS (SELECT 1 FROM relations r WHERE r.user_id = attendances.student_id AND r.mentor_id = ?)
				OR EXISTS (SELECT 1 FROM relations r2 WHERE r2.coach_id = attendances.coach_id AND r2.mentor_id = ?)
			)`,
			mentorID, mentorID,
		)
	}

	var out []*models.Attendance
	if err := q.Order("date desc, id desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) IsMentorOfCoach(ctx context.Context, mentorID string, coachID string) (bool, error) {
	var cnt int64
	err := s.DB.WithContext(ctx).Table("relations").
		Where("coach_id = ? AND mentor_id = ?", coachID, mentorID).
		Count(&cnt).Error
	return cnt > 0, err
}

// helper: used by handlers to detect not-found vs other errors
func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
