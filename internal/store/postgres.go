package store

// import (
// 	"context"
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"time"

// 	"github.com/jackc/pgx/v5/pgxpool"
// 	"github.com/madhava-poojari/dashboard-api/internal/models"
// )

// type Store struct {
// 	pool *pgxpool.Pool
// }

// func NewPgPool(ctx context.Context, dsn string) (*Store, error) {
// 	cfg, err := pgxpool.ParseConfig(dsn)
// 	if err != nil {
// 		return nil, err
// 	}
// 	p, err := pgxpool.NewWithConfig(ctx, cfg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &Store{pool: p}, nil
// }

// func (p *Store) Close() {
// 	p.pool.Close()
// }

// // AutoMigrate created tables if not exists (simple approach for MVP)
// func (p *Store) AutoMigrate(ctx context.Context) error {
// 	queries := []string{
// 		`CREATE TABLE IF NOT EXISTS users (
// 			id TEXT PRIMARY KEY,
// 			email TEXT UNIQUE NOT NULL,
// 			password_hash TEXT,
// 			first_name TEXT,
// 			last_name TEXT,
// 			role TEXT NOT NULL,
// 			approved BOOLEAN DEFAULT FALSE,
// 			active BOOLEAN DEFAULT TRUE,
// 			created_at TIMESTAMPTZ DEFAULT now(),
// 			updated_at TIMESTAMPTZ DEFAULT now()
// 		)`,
// 		`CREATE TABLE IF NOT EXISTS user_details (
// 			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
// 			city TEXT,
// 			state TEXT,
// 			country TEXT,
// 			zipcode TEXT,
// 			phone TEXT,
// 			dob DATE,
// 			lichess_username TEXT,
// 			uscf_id TEXT,
// 			chesscom_username TEXT,
// 			fide_id TEXT,
// 			bio TEXT,
// 			profile_picture_url TEXT,
// 			additional_info JSONB DEFAULT '{}'::jsonb,
// 			updated_at TIMESTAMPTZ DEFAULT now()
// 		)`,
// 		`CREATE TABLE IF NOT EXISTS refresh_tokens (
// 			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
// 			user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
// 			token_hash TEXT NOT NULL,
// 			issued_at TIMESTAMPTZ DEFAULT now(),
// 			expires_at TIMESTAMPTZ,
// 			revoked BOOLEAN DEFAULT FALSE
// 		)`,
// 		`CREATE TABLE IF NOT EXISTS coach_students (
// 			coach_id TEXT REFERENCES users(id) ON DELETE CASCADE,
// 			student_id TEXT REFERENCES users(id) ON DELETE CASCADE,
// 			PRIMARY KEY (coach_id, student_id)
// 		)`,
// 	}
// 	for _, q := range queries {
// 		if _, err := p.pool.Exec(ctx, q); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// // CreateUser inserts into users and an empty user_details
// func (p *Store) CreateUser(ctx context.Context, u *models.User) error {
// 	_, err := p.pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,first_name,last_name,role,approved,active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
// 		u.ID, u.Email, u.PasswordHash, u.FirstName, u.LastName, string(u.Role), u.Approved, u.Active)
// 	if err != nil {
// 		return err
// 	}
// 	_, _ = p.pool.Exec(ctx, `INSERT INTO user_details(user_id,additional_info) VALUES ($1,'{}')`, u.ID)
// 	return nil
// }

// func (p *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
// 	row := p.pool.QueryRow(ctx, `SELECT id,email,password_hash,first_name,last_name,role,approved,active,created_at,updated_at FROM users WHERE email=$1`, email)
// 	u := &models.User{}
// 	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.Approved, &u.Active, &u.CreatedAt, &u.UpdatedAt)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return u, nil
// }

// func (p *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
// 	row := p.pool.QueryRow(ctx, `SELECT id,email,password_hash,first_name,last_name,role,approved,active,created_at,updated_at FROM users WHERE id=$1`, id)
// 	u := &models.User{}
// 	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.Approved, &u.Active, &u.CreatedAt, &u.UpdatedAt)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return u, nil
// }

// func (p *Store) ListUsersAdmin(ctx context.Context) ([]*models.User, error) {
// 	rows, err := p.pool.Query(ctx, `SELECT id,email,first_name,last_name,role,approved,active,created_at,updated_at FROM users ORDER BY created_at DESC`)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()
// 	var res []*models.User
// 	for rows.Next() {
// 		u := &models.User{}
// 		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.Approved, &u.Active, &u.CreatedAt, &u.UpdatedAt); err != nil {
// 			return nil, err
// 		}
// 		res = append(res, u)
// 	}
// 	return res, nil
// }

// // ListStudentsForCoach
// func (p *Store) ListStudentsForCoach(ctx context.Context, coachID string) ([]*models.User, error) {
// 	rows, err := p.pool.Query(ctx, `
// 		SELECT u.id,u.email,u.first_name,u.last_name,u.role,u.approved,u.active,u.created_at,u.updated_at
// 		FROM users u
// 		JOIN coach_students cs ON cs.student_id = u.id
// 		WHERE cs.coach_id = $1
// 		ORDER BY u.created_at DESC
// 	`, coachID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()
// 	var res []*models.User
// 	for rows.Next() {
// 		u := &models.User{}
// 		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.Approved, &u.Active, &u.CreatedAt, &u.UpdatedAt); err != nil {
// 			return nil, err
// 		}
// 		res = append(res, u)
// 	}
// 	return res, nil
// }

// // SaveRefreshToken saves a hashed token
// func (p *Store) SaveRefreshToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
// 	h := sha256.Sum256([]byte(token))
// 	_, err := p.pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`, userID, hex.EncodeToString(h[:]), expiresAt)
// 	return err
// }

// // FindRefreshToken verifies token exists and not revoked and returns token row id
// func (p *Store) FindRefreshToken(ctx context.Context, token string) (bool, error) {
// 	h := sha256.Sum256([]byte(token))
// 	var exists bool
// 	err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE token_hash=$1 AND revoked=false AND expires_at > now())`, hex.EncodeToString(h[:])).Scan(&exists)
// 	return exists, err
// }

// // RevokeRefreshToken deletes or marks revoked
// func (p *Store) RevokeRefreshToken(ctx context.Context, token string) error {
// 	h := sha256.Sum256([]byte(token))
// 	_, err := p.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1`, hex.EncodeToString(h[:]))
// 	return err
// }

// // DeleteExpiredTokens - optional maintenance
// func (p *Store) DeleteExpiredTokens(ctx context.Context) error {
// 	_, err := p.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now()`)
// 	return err
// }
