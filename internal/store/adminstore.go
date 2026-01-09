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

// GetPendingApprovals fetches users where approved = false, ordered by created_at DESC
func (s *Store) GetPendingApprovals(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	if err := s.DB.WithContext(ctx).
		Preload("UserDetails").
		Where("approved = ?", false).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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

// StudentWithAssignment represents a student with their coach assignment info
type StudentWithAssignment struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Approved  bool   `json:"approved"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	CoachID   *string `json:"coach_id"`
	CoachName *string `json:"coach_name"`
	MentorCoachID *string `json:"mentor_coach_id"`
	MentorName    *string `json:"mentor_name"`
}

// GetStudentsWithAssignments fetches all students with their coach assignment info
// Unassigned students come first, then assigned students sorted by user created_at
func (s *Store) GetStudentsWithAssignments(ctx context.Context) ([]StudentWithAssignment, error) {
	var results []StudentWithAssignment

	err := s.DB.WithContext(ctx).
		Table("users").
		Select(`
			users.id,
			users.email,
			users.first_name,
			users.last_name,
			users.role,
			users.approved,
			users.active,
			users.created_at,
			users.updated_at,
			cs.coach_id,
			coach.first_name || ' ' || coach.last_name as coach_name,
			cs.mentor_coach_id,
			mentor.first_name || ' ' || mentor.last_name as mentor_name
		`).
		Joins("LEFT JOIN coach_students cs ON cs.student_id = users.id").
		Joins("LEFT JOIN users coach ON coach.id = cs.coach_id").
		Joins("LEFT JOIN users mentor ON mentor.id = cs.mentor_coach_id").
		Where("users.role = ?", models.RoleStudent).
		Order("CASE WHEN cs.coach_id IS NULL THEN 0 ELSE 1 END, users.created_at DESC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

// CoachWithAssignment represents a coach with their mentor assignment info
type CoachWithAssignment struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Approved  bool   `json:"approved"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	MentorCoachID *string `json:"mentor_coach_id"`
	MentorName    *string `json:"mentor_name"`
	StudentCount  int     `json:"student_count"`
}

// GetCoachesWithAssignments fetches all coaches with their mentor assignment info
// Unassigned coaches come first, then assigned coaches sorted by user created_at
func (s *Store) GetCoachesWithAssignments(ctx context.Context) ([]CoachWithAssignment, error) {
	var results []CoachWithAssignment

	err := s.DB.WithContext(ctx).
		Table("users").
		Select(`
			users.id,
			users.email,
			users.first_name,
			users.last_name,
			users.role,
			users.approved,
			users.active,
			users.created_at,
			users.updated_at,
			(SELECT cs.mentor_coach_id FROM coach_students cs WHERE cs.coach_id = users.id LIMIT 1) as mentor_coach_id,
			(SELECT m.first_name || ' ' || m.last_name FROM users m WHERE m.id = (SELECT cs2.mentor_coach_id FROM coach_students cs2 WHERE cs2.coach_id = users.id LIMIT 1)) as mentor_name,
			(SELECT COUNT(*) FROM coach_students cs3 WHERE cs3.coach_id = users.id) as student_count
		`).
		Where("users.role = ?", models.RoleCoach).
		Order("users.created_at DESC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

// AssignStudentToCoach creates or updates the student-coach relationship
func (s *Store) AssignStudentToCoach(ctx context.Context, studentID, coachID, mentorCoachID string) error {
	// First, remove any existing assignment for this student
	s.DB.WithContext(ctx).Where("student_id = ?", studentID).Delete(&models.CoachStudent{})

	// Create new assignment
	cs := models.CoachStudent{
		CoachID:       coachID,
		StudentID:     studentID,
		MentorCoachID: mentorCoachID,
	}
	return s.DB.WithContext(ctx).Create(&cs).Error
}

// AssignCoachToMentor updates all students of a coach to have the specified mentor
func (s *Store) AssignCoachToMentor(ctx context.Context, coachID, mentorCoachID string) error {
	return s.DB.WithContext(ctx).
		Model(&models.CoachStudent{}).
		Where("coach_id = ?", coachID).
		Update("mentor_coach_id", mentorCoachID).Error
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
