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
	print("Updating user details for user ID:", id, "with fields:", fields)
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

func (s *Store) ListActiveStudents(ctx context.Context, activeOnly bool) ([]*models.User, error) {
	var res []*models.User
	query := s.DB.WithContext(ctx).
		Where("role = ? ", models.RoleStudent).
		Order("created_at desc")
	if activeOnly {
		query.Where("active = ?", true)
	}
	if err := query.
		Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Store) ListStudentsForCoachOrMentor(ctx context.Context, userID string) ([]*models.User, error) {
	var students []models.User
	err := s.DB.WithContext(ctx).
		Table("users").
		Select("users.*").
		Joins("JOIN relations r ON r.user_id = users.id").
		Where("(r.coach_id = ? OR r.mentor_id = ?) AND users.role = 'student' AND users.active = true", userID, userID).
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
func (s *Store) GetCoachesByStudentID(ctx context.Context, studentID string) (string, string, error) {
	var r models.Relation
	if err := s.DB.WithContext(ctx).Where("user_id = ?", studentID).First(&r).Error; err != nil {
		return "", "", err
	}
	return r.CoachID, r.MentorID, nil
}

// ListCoachesForMentor returns all coaches assigned to a mentor (via relations), plus the mentor themselves.
func (s *Store) ListCoachesForMentor(ctx context.Context, mentorID string) ([]*models.User, error) {
	// With the new schema, coach-mentor rows have user_id=coach_id, coach_id='', mentor_id=mentorID
	var coachIDs []string
	if err := s.DB.WithContext(ctx).
		Table("relations").
		Joins("JOIN users u ON u.id = relations.user_id").
		Where("relations.mentor_id = ? AND u.role = 'coach'", mentorID).
		Distinct("relations.user_id").
		Pluck("relations.user_id", &coachIDs).Error; err != nil {
		return nil, err
	}

	// Build a unique set including the mentor themselves
	idSet := map[string]struct{}{mentorID: {}}
	for _, id := range coachIDs {
		if id != "" {
			idSet[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	var users []*models.User
	if err := s.DB.WithContext(ctx).
		Where("id IN ? AND active = true", ids).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func GetMentorCoachDetails(db *gorm.DB, studentID string) (string, string, string, string, error) {
	var relation models.Relation

	if err := db.Where("user_id = ?", studentID).First(&relation).Error; err != nil {
		return "", "", "", "", err
	}

	var coach models.User
	var mentor models.User

	var coachDetails models.UserDetails
	var mentorDetails models.UserDetails

	if relation.CoachID != "" {
		db.First(&coach, "id = ?", relation.CoachID)
		db.First(&coachDetails, "user_id = ?", relation.CoachID)
	}

	if relation.MentorID != "" {
		db.First(&mentor, "id = ?", relation.MentorID)
		db.First(&mentorDetails, "user_id = ?", relation.MentorID)
	}

	coachName := coach.FirstName + " " + coach.LastName
	mentorName := mentor.FirstName + " " + mentor.LastName

	return coachName, coachDetails.Phone,
		mentorName, mentorDetails.Phone, nil
}
