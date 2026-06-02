package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	// Migrate relations table from composite PK to single PK (if needed)
	if err := migrateRelationsSchema(db); err != nil {
		return nil, fmt.Errorf("relations migration failed: %w", err)
	}

	// AutoMigrate (non-destructive: creates tables/columns/indexes)

	if err := db.Set("gorm:DisableForeignKeyConstraintWhenMigrating", true).AutoMigrate(
		&models.User{},
		&models.UserDetails{},
		&models.RefreshToken{},
		&models.Relation{},
		&models.LessonPlan{},
		&models.Note{},
		&models.Attendance{},
		&models.Image{},
		&models.ZipcodeScrapeScope{},
		&models.Tournament{},
		&models.TournamentWithinRadius{},
		&models.ClassSchedule{},
		&models.ReferralRelationship{},
		&models.RatingHistory{},
	); err != nil {
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

/* ------------------ Relation (relations table) management ------------------ */

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

/* ------------------ Helpers ------------------ */

func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// migrateRelationsSchema migrates the relations table from composite PK (coach_id, user_id)
// to single PK (user_id). This is a one-time migration that:
// 1. Detects if old schema exists (coach_id is part of PK)
// 2. Copies student rows (skipping T- tracking rows)
// 3. Creates coach-mentor rows from tracking data
// 4. Replaces old table with new schema
func migrateRelationsSchema(db *gorm.DB) error {
	// Check if relations table exists
	if !db.Migrator().HasTable("relations") {
		return nil // table doesn't exist yet, AutoMigrate will create it
	}

	// Check if the old composite PK still exists by seeing if coach_id is part of the PK.
	var count int64
	err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = 'relations' AND tc.constraint_type = 'PRIMARY KEY' AND kcu.column_name = 'coach_id'
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("checking relations schema: %w", err)
	}
	if count == 0 {
		return nil // already migrated
	}

	fmt.Println("[migration] Migrating relations table from composite PK to single PK (user_id)...")

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Create temp table with new schema
		if err := tx.Exec(`
			CREATE TABLE relations_new (
				user_id  TEXT PRIMARY KEY,
				coach_id TEXT DEFAULT '',
				mentor_id TEXT DEFAULT ''
			)
		`).Error; err != nil {
			return fmt.Errorf("creating relations_new: %w", err)
		}

		// 2. Copy student rows (skip T- tracking rows)
		if err := tx.Exec(`
			INSERT INTO relations_new (user_id, coach_id, mentor_id)
			SELECT DISTINCT ON (user_id) user_id, coach_id, COALESCE(mentor_id, '')
			FROM relations
			WHERE user_id NOT LIKE 'T-%%'
			ORDER BY user_id
		`).Error; err != nil {
			return fmt.Errorf("copying student rows: %w", err)
		}

		// 3. Create coach-mentor rows from tracking rows and existing mentor assignments
		if err := tx.Exec(`
			INSERT INTO relations_new (user_id, coach_id, mentor_id)
			SELECT DISTINCT ON (coach_id) coach_id, '', COALESCE(mentor_id, '')
			FROM relations
			WHERE mentor_id IS NOT NULL AND mentor_id != ''
			  AND coach_id NOT IN (SELECT user_id FROM relations_new)
			ORDER BY coach_id
			ON CONFLICT (user_id) DO NOTHING
		`).Error; err != nil {
			return fmt.Errorf("creating coach-mentor rows: %w", err)
		}

		// 4. Drop old table and rename new
		if err := tx.Exec(`DROP TABLE relations`).Error; err != nil {
			return fmt.Errorf("dropping old relations: %w", err)
		}
		if err := tx.Exec(`ALTER TABLE relations_new RENAME TO relations`).Error; err != nil {
			return fmt.Errorf("renaming relations_new: %w", err)
		}

		// 5. Create indexes
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_relations_coach_id ON relations (coach_id)`).Error; err != nil {
			return fmt.Errorf("creating coach_id index: %w", err)
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_relations_mentor_id ON relations (mentor_id)`).Error; err != nil {
			return fmt.Errorf("creating mentor_id index: %w", err)
		}

		fmt.Println("[migration] Relations table migration complete.")
		return nil
	})
}
