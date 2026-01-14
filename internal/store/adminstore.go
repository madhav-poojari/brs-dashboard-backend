package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
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

// ListUnapprovedUsers returns users that are not approved, sorted by newest first
func (s *Store) ListUnapprovedUsers(ctx context.Context) ([]*models.User, error) {
	var res []*models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("approved = ?", false).Order("created_at desc").Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// StudentWithAssignment represents a student with their coach/mentor assignment info
type StudentWithAssignment struct {
	*models.User
	CoachID       *string    `json:"coach_id,omitempty"`
	MentorCoachID *string    `json:"mentor_coach_id,omitempty"`
	AssignedAt    *time.Time `json:"assigned_at,omitempty"`
}

// ListStudentsWithAssignments returns all students with their assignment info
func (s *Store) ListStudentsWithAssignments(ctx context.Context) ([]*StudentWithAssignment, error) {
	var students []models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("role = ?", "student").Find(&students).Error; err != nil {
		return nil, err
	}

	var assignments []struct {
		StudentID     string    `gorm:"column:student_id"`
		CoachID       string    `gorm:"column:coach_id"`
		MentorCoachID string    `gorm:"column:mentor_coach_id"`
		CreatedAt     time.Time `gorm:"column:created_at"`
	}
	if err := s.DB.WithContext(ctx).Table("coach_students").Find(&assignments).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Create a map of student_id -> assignment
	assignmentMap := make(map[string]struct {
		CoachID       string
		MentorCoachID string
		CreatedAt     time.Time
	})
	for _, a := range assignments {
		assignmentMap[a.StudentID] = struct {
			CoachID       string
			MentorCoachID string
			CreatedAt     time.Time
		}{
			CoachID:       a.CoachID,
			MentorCoachID: a.MentorCoachID,
			CreatedAt:     a.CreatedAt,
		}
	}

	result := make([]*StudentWithAssignment, 0, len(students))
	unassigned := make([]*StudentWithAssignment, 0)
	assigned := make([]*StudentWithAssignment, 0)

	for i := range students {
		swa := &StudentWithAssignment{User: &students[i]}
		if a, ok := assignmentMap[students[i].ID]; ok {
			swa.CoachID = &a.CoachID
			if a.MentorCoachID != "" {
				swa.MentorCoachID = &a.MentorCoachID
			}
			swa.AssignedAt = &a.CreatedAt
			assigned = append(assigned, swa)
		} else {
			unassigned = append(unassigned, swa)
		}
	}

	// Sort assigned by assigned_at desc (newest first)
	for i := 0; i < len(assigned)-1; i++ {
		for j := i + 1; j < len(assigned); j++ {
			if assigned[i].AssignedAt != nil && assigned[j].AssignedAt != nil {
				if assigned[i].AssignedAt.Before(*assigned[j].AssignedAt) {
					assigned[i], assigned[j] = assigned[j], assigned[i]
				}
			}
		}
	}

	// Unassigned first, then assigned (sorted by newest assignment)
	result = append(result, unassigned...)
	result = append(result, assigned...)

	return result, nil
}

// CoachWithAssignment represents a coach with their student assignment info
type CoachWithAssignment struct {
	*models.User
	StudentID  *string    `json:"student_id,omitempty"`
	IsMentor   bool       `json:"is_mentor"`
	AssignedAt *time.Time `json:"assigned_at,omitempty"`
}

// ListCoachesWithAssignments returns all coaches with their assignment info
func (s *Store) ListCoachesWithAssignments(ctx context.Context) ([]*CoachWithAssignment, error) {
	var coaches []models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("role IN ?", []string{"coach", "mentor"}).Find(&coaches).Error; err != nil {
		return nil, err
	}

	var assignments []struct {
		CoachID       string    `gorm:"column:coach_id"`
		MentorCoachID string    `gorm:"column:mentor_coach_id"`
		StudentID     string    `gorm:"column:student_id"`
		CreatedAt     time.Time `gorm:"column:created_at"`
	}
	if err := s.DB.WithContext(ctx).Table("coach_students").Find(&assignments).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Create a map of coach_id -> assignments (can have multiple)
	assignmentMap := make(map[string][]struct {
		StudentID string
		IsMentor  bool
		CreatedAt time.Time
	})
	for _, a := range assignments {
		// Add as coach assignment
		if a.CoachID != "" {
			assignmentMap[a.CoachID] = append(assignmentMap[a.CoachID], struct {
				StudentID string
				IsMentor  bool
				CreatedAt time.Time
			}{
				StudentID: a.StudentID,
				IsMentor:  false,
				CreatedAt: a.CreatedAt,
			})
		}
		// Add as mentor assignment
		if a.MentorCoachID != "" {
			assignmentMap[a.MentorCoachID] = append(assignmentMap[a.MentorCoachID], struct {
				StudentID string
				IsMentor  bool
				CreatedAt time.Time
			}{
				StudentID: a.StudentID,
				IsMentor:  true,
				CreatedAt: a.CreatedAt,
			})
		}
	}

	result := make([]*CoachWithAssignment, 0)
	unassigned := make([]*CoachWithAssignment, 0)
	assigned := make([]*CoachWithAssignment, 0)

	for i := range coaches {
		assignments := assignmentMap[coaches[i].ID]
		if len(assignments) == 0 {
			cwa := &CoachWithAssignment{User: &coaches[i]}
			unassigned = append(unassigned, cwa)
		} else {
			// Create one entry per assignment, sorted by newest first
			for _, a := range assignments {
				cwa := &CoachWithAssignment{
					User:       &coaches[i],
					StudentID:  &a.StudentID,
					IsMentor:   a.IsMentor,
					AssignedAt: &a.CreatedAt,
				}
				assigned = append(assigned, cwa)
			}
		}
	}

	// Sort assigned by assigned_at desc (newest first)
	for i := 0; i < len(assigned)-1; i++ {
		for j := i + 1; j < len(assigned); j++ {
			if assigned[i].AssignedAt != nil && assigned[j].AssignedAt != nil {
				if assigned[i].AssignedAt.Before(*assigned[j].AssignedAt) {
					assigned[i], assigned[j] = assigned[j], assigned[i]
				}
			}
		}
	}

	// Unassigned first, then assigned (sorted by newest assignment)
	result = append(result, unassigned...)
	result = append(result, assigned...)

	return result, nil
}

// GetPendingApprovals is an alias for ListUnapprovedUsers (for backward compatibility)
func (s *Store) GetPendingApprovals(ctx context.Context) ([]*models.User, error) {
	return s.ListUnapprovedUsers(ctx)
}

// GetUsersByRole fetches users filtered by a specific role
func (s *Store) GetUsersByRole(ctx context.Context, role models.Role) ([]*models.User, error) {
	var users []*models.User
	if err := s.DB.WithContext(ctx).
		Preload("UserDetails").
		Where("role = ?", role).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// GetAllStudents fetches all users with role student
func (s *Store) GetAllStudents(ctx context.Context) ([]*models.User, error) {
	return s.GetUsersByRole(ctx, models.RoleStudent)
}

// GetAllCoaches fetches users with role coach
func (s *Store) GetAllCoaches(ctx context.Context) ([]*models.User, error) {
	return s.GetUsersByRole(ctx, models.RoleCoach)
}

// GetAllMentorCoaches fetches users with role mentor
func (s *Store) GetAllMentorCoaches(ctx context.Context) ([]*models.User, error) {
	return s.GetUsersByRole(ctx, models.RoleMentor)
}

// GetAllUsersGrouped returns all users grouped by role for admin view
func (s *Store) GetAllUsersGrouped(ctx context.Context) (map[string][]*models.User, error) {
	result := make(map[string][]*models.User)

	students, err := s.GetAllStudents(ctx)
	if err != nil {
		return nil, err
	}
	result["students"] = students

	coaches, err := s.GetAllCoaches(ctx)
	if err != nil {
		return nil, err
	}
	result["coaches"] = coaches

	mentors, err := s.GetAllMentorCoaches(ctx)
	if err != nil {
		return nil, err
	}
	result["mentors"] = mentors

	return result, nil
}
