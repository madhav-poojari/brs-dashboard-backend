package store

import (
	"context"
	"errors"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	utils "github.com/madhava-poojari/dashboard-api/internal/utils"

	"gorm.io/gorm"
)

// Tag restriction map (sensible defaults)
var tagRestrictedTo = map[string][]models.Role{
	"StudentAssessment": {"mentor", "coach", "admin"},
	"CoachAssessment":   {"mentor", "admin"},
	"ParentFeedback":    {"mentor", "coach", "admin"},
	"LessonPlanArchive": {models.RoleMentor, models.RoleAdmin},
}

func TagAllowedForRole(tag string, role models.Role) bool {
	allowed, ok := tagRestrictedTo[tag]
	if !ok {
		return true // not restricted
	}
	for _, r := range allowed {
		if r == role {
			return true
		}
	}
	return false
}

// IsCoachOf / IsMentorOf / IsRelatedStudent
func (s *Store) IsCoachOf(ctx context.Context, coachID, studentID string) (bool, error) {
	var cnt int64
	err := s.DB.WithContext(ctx).Table("relations").
		Where("user_id = ? AND coach_id = ?", studentID, coachID).
		Count(&cnt).Error
	return cnt > 0, err
}
func (s *Store) IsMentorOf(ctx context.Context, mentorID, studentID string) (bool, error) {
	var cnt int64
	err := s.DB.WithContext(ctx).Table("relations").
		Where("user_id = ? AND mentor_id = ?", studentID, mentorID).
		Count(&cnt).Error
	return cnt > 0, err
}
func (s *Store) IsRelatedStudent(ctx context.Context, requesterID string, studentID string) (bool, error) {
	// admin quick-check
	u, err := s.GetUserByID(ctx, requesterID)
	if err == nil && u.Role == "admin" {
		return true, nil
	}
	if requesterID == studentID {
		return true, nil
	}
	// TODO get coach and mentor from a single SQL call
	c, err := s.IsCoachOf(ctx, requesterID, studentID)
	if err != nil {
		return false, err
	}
	if c {
		return true, nil
	}
	m, err := s.IsMentorOf(ctx, requesterID, studentID)
	if err != nil {
		return false, err
	}
	return m, nil
}

// CreateNote
func (s *Store) CreateNote(ctx context.Context, n *models.Note) error {
	n.CreatedAt = time.Now()
	n.UpdatedAt = time.Now()
	return s.DB.WithContext(ctx).Create(n).Error
}

// CreateLessonPlan (archives existing active plan into a Note)
func (s *Store) CreateLessonPlan(ctx context.Context, lp *models.LessonPlan) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// find active plan
		var old models.LessonPlan
		if err := tx.Where("user_id = ? AND active = true", lp.UserID).First(&old).Error; err == nil {
			// convert old -> Note
			note := models.Note{
				UserID:      old.UserID,
				Title:       old.Title + " (archived)",
				Description: utils.StringifyJSON(old.Description), // helper to convert JSON array to text
				PrimaryTag:  "LessonPlanArchive",
				Tags:        utils.DatatypesJSONFromStrings([]string{"LessonPlanArchive"}),
				IsStarred:   false,
				Visibility:  4, // default visibility; adjust if needed
				CreatedBy:   old.CreatedBy,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := tx.Create(&note).Error; err != nil {
				return err
			}
			// deactivate old
			if err := tx.Model(&models.LessonPlan{}).Where("id = ?", old.ID).Updates(map[string]interface{}{"active": false, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// create new plan
		lp.CreatedAt = time.Now()
		lp.UpdatedAt = time.Now()
		lp.Active = true
		return tx.Create(lp).Error
	})
}

// GetNotesByStudent returns active lesson plan + notes (paged)
func (s *Store) GetNotesByStudent(ctx context.Context, userId string, limit, offset int) ([]*models.Note, *models.LessonPlan, error) {
	var notes []*models.Note
	if limit == 0 {
		limit = 50
	}
	if err := s.DB.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("created_at desc").
		Limit(limit).Offset(offset).
		Find(&notes).Error; err != nil {
		return nil, nil, err
	}
	var lp models.LessonPlan
	if err := s.DB.WithContext(ctx).Where("user_id = ? AND active = true", userId).First(&lp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return notes, nil, nil
		}
		return nil, nil, err
	}
	return notes, &lp, nil
}

func (s *Store) UpdateNoteFields(ctx context.Context, noteID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return s.DB.WithContext(ctx).Model(&models.Note{}).Where("id = ?", noteID).Updates(updates).Error
}

func (s *Store) UpdateLessonPlanFields(ctx context.Context, planID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	// LessonPlan ID is uint
	return s.DB.WithContext(ctx).Model(&models.LessonPlan{}).Where("id = ?", planID).Updates(updates).Error
}

func (s *Store) DeleteNoteSoft(ctx context.Context, noteID string) error {
	return s.DB.WithContext(ctx).Where("id = ?", noteID).Delete(&models.Note{}).Error
}

func (s *Store) CanAccessNoteForRequester(ctx context.Context, requester *models.User, n *models.Note) bool {
	// visibility rules:
	// 1 = admin only
	// 2 = admin + mentor
	// 3 = admin + mentor + coach
	// 4 = student + their coach/mentor + admin
	switch n.Visibility {
	case 1:
		return requester.Role == "admin"
	case 2:
		if requester.Role == "admin" {
			return true
		}
		ok, _ := s.IsMentorOf(ctx, requester.ID, n.UserID)
		return ok
	case 3:
		if requester.Role == "admin" {
			return true
		}
		isMentor, _ := s.IsMentorOf(ctx, requester.ID, n.UserID)
		if isMentor {
			return true
		}
		isCoach, _ := s.IsCoachOf(ctx, requester.ID, n.UserID)
		return isCoach
	case 4:
		// any related (student themself or coach/mentor or admin)
		ok, _ := s.IsRelatedStudent(ctx, requester.ID, n.UserID)
		return ok
	default:
		return false
	}
}
