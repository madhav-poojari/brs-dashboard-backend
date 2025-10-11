package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewGormStore(cfg *config.Config) (*Store, error) {
	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormCfg)
	if err != nil {
		return nil, err
	}
	// AutoMigrate (non-destructive: creates tables/columns/indexes)

	if err := db.Set("gorm:DisableForeignKeyConstraintWhenMigrating", true).AutoMigrate(&models.User{}, &models.UserDetails{}, &models.RefreshToken{}, &models.CoachStudent{}); err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// Pooling sensible defaults for small VPS (tune later)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return &Store{DB: db, Cfg: cfg}, nil
}

/* ------------------ User CRUD ------------------ */

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	// create user and empty details in a transaction
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		ud := &models.UserDetails{UserID: u.ID, AdditionalInfo: map[string]interface{}{}}
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

func (s *Store) UpdateUserProfile(ctx context.Context, id, firstName, lastName string) error {
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{"first_name": firstName, "last_name": lastName, "updated_at": time.Now()}).Error
}

func (s *Store) UpdateUserDetails(ctx context.Context, id, city, phone, bio string) error {
	return s.DB.WithContext(ctx).Model(&models.UserDetails{}).Where("user_id = ?", id).
		Updates(map[string]interface{}{"city": city, "phone": phone, "bio": bio, "updated_at": time.Now()}).Error
}

func (s *Store) ListUsersAdmin(ctx context.Context) ([]*models.User, error) {
	var res []*models.User
	if err := s.DB.WithContext(ctx).Order("created_at desc").Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Store) ListStudentsForCoach(ctx context.Context, coachID string) ([]*models.User, error) {
	var students []models.User
	if err := s.DB.WithContext(ctx).
		Joins("JOIN coach_students cs on cs.student_id = users.id").
		Where("cs.coach_id = ?", coachID).
		Order("users.created_at desc").
		Find(&students).Error; err != nil {
		return nil, err
	}
	out := make([]*models.User, len(students))
	for i := range students {
		out[i] = &students[i]
	}
	return out, nil
}

/* ------------------ CoachStudent management ------------------ */

func (s *Store) AddCoachStudent(ctx context.Context, coachID, studentID, mentorID string) error {
	cs := models.CoachStudent{CoachID: coachID, StudentID: studentID, MentorCoachID: mentorID}
	return s.DB.WithContext(ctx).Create(&cs).Error
}
func (s *Store) RemoveCoachStudent(ctx context.Context, coachID, studentID string) error {
	return s.DB.WithContext(ctx).Where("coach_id = ? AND student_id = ?", coachID, studentID).Delete(&models.CoachStudent{}).Error
}

/* ------------------ Refresh token methods ------------------ */

func hashTokenPlain(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// SaveRefreshToken stores a token (hashed) and expiry
func (s *Store) SaveRefreshToken(ctx context.Context, userID, plainToken string, expiresAt time.Time) error {
	rt := models.RefreshToken{
		UserID:    userID,
		TokenHash: hashTokenPlain(plainToken),
		IssuedAt:  time.Now(),
		ExpiresAt: expiresAt,
		Revoked:   false,
	}
	return s.DB.WithContext(ctx).Create(&rt).Error
}

// FindRefreshToken returns the token row (if valid and not revoked)
func (s *Store) FindRefreshToken(ctx context.Context, plainToken string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	if err := s.DB.WithContext(ctx).Where("token_hash = ? AND revoked = false AND expires_at > now()", hashTokenPlain(plainToken)).First(&rt).Error; err != nil {
		return nil, err
	}
	return &rt, nil
}

// RevokeRefreshToken marks token revoked
func (s *Store) RevokeRefreshToken(ctx context.Context, plainToken string) error {
	return s.DB.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("token_hash = ?", hashTokenPlain(plainToken)).Updates(map[string]interface{}{"revoked": true}).Error
}

// RotateRefreshToken: revoke old token, create a new one, return new plain token and expires
func (s *Store) RotateRefreshToken(ctx context.Context, oldPlain string, newPlain string, newExpiry time.Time) (string, error) {
	return newPlain, s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// find old token and ensure it exists
		var old models.RefreshToken
		if err := tx.Where("token_hash = ? AND revoked = false AND expires_at > now()", hashTokenPlain(oldPlain)).First(&old).Error; err != nil {
			return err
		}
		// revoke old
		if err := tx.Model(&models.RefreshToken{}).Where("id = ?", old.ID).Update("revoked", true).Error; err != nil {
			return err
		}
		// create new
		newRT := models.RefreshToken{
			UserID:    old.UserID,
			TokenHash: hashTokenPlain(newPlain),
			IssuedAt:  time.Now(),
			ExpiresAt: newExpiry,
			Revoked:   false,
		}
		if err := tx.Create(&newRT).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) DeleteExpiredTokens(ctx context.Context) error {
	return s.DB.WithContext(ctx).Where("expires_at < now()").Delete(&models.RefreshToken{}).Error
}

/* ------------------ Admin helpers ------------------ */

func (s *Store) ApproveUser(ctx context.Context, userID string) error {
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{"approved": true, "updated_at": time.Now()}).Error
}

func (s *Store) ChangeUserRole(ctx context.Context, userID, role string) error {
	return s.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("role", role).Error
}

/* ------------------ Helpers ------------------ */

func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
