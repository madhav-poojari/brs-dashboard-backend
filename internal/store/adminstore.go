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
	CoachName     string     `json:"coach_name,omitempty"`
	MentorCoachID *string    `json:"mentor_coach_id,omitempty"`
	MentorName    string     `json:"mentor_name,omitempty"`
	AssignedAt    *time.Time `json:"assigned_at,omitempty"`
}

// ListStudentsWithAssignments returns all students with their assignment info
func (s *Store) ListStudentsWithAssignments(ctx context.Context) ([]*StudentWithAssignment, error) {
	var students []models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("role = ? AND active = ?", "student", true).Find(&students).Error; err != nil {
		return nil, err
	}

	// Build a map of user_id -> relation
	var relations []models.Relation
	if err := s.DB.WithContext(ctx).Find(&relations).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	relMap := make(map[string]models.Relation)
	for _, r := range relations {
		relMap[r.UserID] = r
	}

	// Build a name lookup for coaches/mentors
	nameMap, err := s.buildUserNameMap(ctx)
	if err != nil {
		return nil, err
	}

	unassigned := make([]*StudentWithAssignment, 0)
	assigned := make([]*StudentWithAssignment, 0)

	for i := range students {
		swa := &StudentWithAssignment{User: &students[i]}
		if r, ok := relMap[students[i].ID]; ok {
			if r.CoachID != "" {
				swa.CoachID = &r.CoachID
				swa.CoachName = nameMap[r.CoachID]
			}
			if r.MentorID != "" {
				swa.MentorCoachID = &r.MentorID
				swa.MentorName = nameMap[r.MentorID]
			}
		}
		if swa.CoachID != nil || swa.MentorCoachID != nil {
			assigned = append(assigned, swa)
		} else {
			unassigned = append(unassigned, swa)
		}
	}

	result := make([]*StudentWithAssignment, 0, len(students))
	result = append(result, unassigned...)
	result = append(result, assigned...)
	return result, nil
}

type CoachPickerItem struct {
	ID                  string `json:"id"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	CurrentStudentCount int    `json:"current_student_count"`
}

// ListCoachesForPicker returns all active coaches with their real student count.
func (s *Store) ListCoachesForPicker(ctx context.Context) ([]*CoachPickerItem, error) {
	var coaches []models.User
	if err := s.DB.WithContext(ctx).
		Where("role = ? AND active = ?", "coach", true).
		Find(&coaches).Error; err != nil {
		return nil, err
	}

	// Count students per coach from relations
	var relations []models.Relation
	if err := s.DB.WithContext(ctx).Where("coach_id != ''").Find(&relations).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	studentCountMap := make(map[string]int)
	for _, r := range relations {
		if r.CoachID != "" {
			studentCountMap[r.CoachID]++
		}
	}

	result := make([]*CoachPickerItem, 0, len(coaches))
	for _, c := range coaches {
		result = append(result, &CoachPickerItem{
			ID:                  c.ID,
			FirstName:           c.FirstName,
			LastName:            c.LastName,
			CurrentStudentCount: studentCountMap[c.ID],
		})
	}
	return result, nil
}

// ListMentorsForPicker returns all active mentors with their assignee count.
func (s *Store) ListMentorsForPicker(ctx context.Context) ([]*CoachPickerItem, error) {
	var mentors []models.User
	if err := s.DB.WithContext(ctx).
		Where("role = ? AND active = ?", "mentor", true).
		Find(&mentors).Error; err != nil {
		return nil, err
	}

	// Count assignees per mentor from relations
	var relations []models.Relation
	if err := s.DB.WithContext(ctx).Where("mentor_id != ''").Find(&relations).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	countMap := make(map[string]int)
	for _, r := range relations {
		if r.MentorID != "" {
			countMap[r.MentorID]++
		}
	}

	result := make([]*CoachPickerItem, 0, len(mentors))
	for _, m := range mentors {
		result = append(result, &CoachPickerItem{
			ID:                  m.ID,
			FirstName:           m.FirstName,
			LastName:            m.LastName,
			CurrentStudentCount: countMap[m.ID],
		})
	}
	return result, nil
}

// CoachWithAssignment represents a coach with their mentor assignment info
type CoachWithAssignment struct {
	*models.User
	MentorCoachID *string `json:"mentor_coach_id,omitempty"`
	MentorName    string  `json:"mentor_name,omitempty"`
}

// ListCoachesWithAssignments returns all coaches (role=coach) with their mentor assignment
func (s *Store) ListCoachesWithAssignments(ctx context.Context) ([]*CoachWithAssignment, error) {
	var coaches []models.User
	if err := s.DB.WithContext(ctx).Preload("UserDetails").Where("role = ? AND active = ?", "coach", true).Find(&coaches).Error; err != nil {
		return nil, err
	}

	// Build relation map
	var relations []models.Relation
	if err := s.DB.WithContext(ctx).Find(&relations).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	relMap := make(map[string]models.Relation)
	for _, r := range relations {
		relMap[r.UserID] = r
	}

	nameMap, err := s.buildUserNameMap(ctx)
	if err != nil {
		return nil, err
	}

	unassigned := make([]*CoachWithAssignment, 0)
	assigned := make([]*CoachWithAssignment, 0)

	for i := range coaches {
		cwa := &CoachWithAssignment{User: &coaches[i]}
		if r, ok := relMap[coaches[i].ID]; ok && r.MentorID != "" {
			cwa.MentorCoachID = &r.MentorID
			cwa.MentorName = nameMap[r.MentorID]
			assigned = append(assigned, cwa)
		} else {
			unassigned = append(unassigned, cwa)
		}
	}

	result := make([]*CoachWithAssignment, 0, len(coaches))
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

// SetStudentCoachAssignment assigns/moves/unassigns a coach for a student.
func (s *Store) SetStudentCoachAssignment(ctx context.Context, studentID, coachID string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Relation
		err := tx.Where("user_id = ?", studentID).First(&existing).Error
		found := (err == nil)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if coachID == "" {
			// Remove coach assignment
			if found {
				return tx.Model(&models.Relation{}).Where("user_id = ?", studentID).Update("coach_id", "").Error
			}
			return nil
		}

		if found {
			// Update existing relation
			return tx.Model(&models.Relation{}).Where("user_id = ?", studentID).Update("coach_id", coachID).Error
		}

		// Create new relation
		return tx.Create(&models.Relation{UserID: studentID, CoachID: coachID}).Error
	})
}

// SetStudentMentor assigns/removes a mentor for a student.
func (s *Store) SetStudentMentor(ctx context.Context, studentID, mentorID string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Relation
		err := tx.Where("user_id = ?", studentID).First(&existing).Error
		found := (err == nil)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if found {
			return tx.Model(&models.Relation{}).Where("user_id = ?", studentID).Update("mentor_id", mentorID).Error
		}

		// Create new relation (mentor can be assigned even without a coach)
		return tx.Create(&models.Relation{UserID: studentID, MentorID: mentorID}).Error
	})
}

// SetCoachMentorAssignment assigns/removes a mentor for a coach.
func (s *Store) SetCoachMentorAssignment(ctx context.Context, coachID, mentorCoachID string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.Relation
		err := tx.Where("user_id = ?", coachID).First(&existing).Error
		found := (err == nil)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if found {
			return tx.Model(&models.Relation{}).Where("user_id = ?", coachID).Update("mentor_id", mentorCoachID).Error
		}

		if mentorCoachID == "" {
			return nil // nothing to do
		}

		// Create new relation for coach (coach_id stays empty — this is a coach-mentor row)
		return tx.Create(&models.Relation{UserID: coachID, MentorID: mentorCoachID}).Error
	})
}

// buildUserNameMap returns a map of user ID -> "FirstName LastName" for all users.
func (s *Store) buildUserNameMap(ctx context.Context) (map[string]string, error) {
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
