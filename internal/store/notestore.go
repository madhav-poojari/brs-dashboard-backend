package store

import (
	"context"
	"encoding/json"
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

// StudentNoteSummary is a lightweight summary for the students-page cards.
type StudentNoteSummary struct {
	LessonPlanTitle string   `json:"lesson_plan_title,omitempty"`
	LessonPlanDesc  []string `json:"lesson_plan_description,omitempty"`
	LatestNoteTitle string   `json:"latest_note_title,omitempty"`
}

// GetBulkNotesSummary returns a map of studentID -> summary containing
// the active lesson-plan title/description and the latest visible note title.
// minVisibility controls which notes are considered (1=all, 2=mentor+, 3=coach+).
// Uses 2 DB queries regardless of student count.
func (s *Store) GetBulkNotesSummary(ctx context.Context, studentIDs []string, minVisibility int) (map[string]*StudentNoteSummary, error) {
	result := make(map[string]*StudentNoteSummary, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}

	// 1. Active lesson plans for all students (one query)
	var lessonPlans []models.LessonPlan
	if err := s.DB.WithContext(ctx).
		Where("user_id IN ? AND active = true", studentIDs).
		Find(&lessonPlans).Error; err != nil {
		return nil, err
	}
	for i := range lessonPlans {
		lp := &lessonPlans[i]
		summary := &StudentNoteSummary{
			LessonPlanTitle: lp.Title,
		}
		// Parse the JSON description array
		var desc []string
		if err := json.Unmarshal([]byte(lp.Description), &desc); err == nil {
			summary.LessonPlanDesc = desc
		}
		result[lp.UserID] = summary
	}

	// 2. Latest visible note per student (one query using Postgres DISTINCT ON)
	type latestNote struct {
		UserID string `gorm:"column:user_id"`
		Title  string `gorm:"column:title"`
	}
	var notes []latestNote
	if err := s.DB.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (user_id) user_id, title
			 FROM notes
			 WHERE user_id IN ?
			   AND deleted_at IS NULL
			   AND visibility >= ?
			 ORDER BY user_id, created_at DESC`,
			studentIDs, minVisibility).
		Scan(&notes).Error; err != nil {
		return nil, err
	}
	for _, n := range notes {
		if _, ok := result[n.UserID]; !ok {
			result[n.UserID] = &StudentNoteSummary{}
		}
		result[n.UserID].LatestNoteTitle = n.Title
	}

	return result, nil
}
