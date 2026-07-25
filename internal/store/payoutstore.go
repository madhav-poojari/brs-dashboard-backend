package store

import (
	"context"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/* ─────────────── Unit Balance helpers ─────────────── */

// GetOrCreateUnitBalance returns the balance row for a user, creating one (balance=0) if it doesn't exist.
func (s *Store) GetOrCreateUnitBalance(ctx context.Context, userID string) (*models.UnitBalance, error) {
	var bal models.UnitBalance
	err := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Attrs(models.UnitBalance{UserID: userID, Balance: 0, UpdatedAt: time.Now()}).
		FirstOrCreate(&bal).Error
	return &bal, err
}

// StudentWithBalance is a projection for the admin list view.
type StudentWithBalance struct {
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Balance   float64 `json:"balance"`
}

// GetAllStudentBalances returns all active students with their current balance.
func (s *Store) GetAllStudentBalances(ctx context.Context) ([]*StudentWithBalance, error) {
	var results []*StudentWithBalance
	err := s.DB.WithContext(ctx).
		Table("users").
		Select("users.id, users.first_name, users.last_name, users.email, COALESCE(unit_balances.balance, 0) as balance").
		Joins("LEFT JOIN unit_balances ON unit_balances.user_id = users.id").
		Where("users.role = ? AND users.active = ? AND users.approved = ?", "student", true, true).
		Order("users.first_name, users.last_name").
		Scan(&results).Error
	return results, err
}

/* ─────────────── Unit Transaction CRUD ─────────────── */

// CreateUnitTransaction inserts a new transaction record.
func (s *Store) CreateUnitTransaction(ctx context.Context, tx *models.UnitTransaction) error {
	tx.CreatedAt = time.Now()
	return s.DB.WithContext(ctx).Create(tx).Error
}

// GetUnitTransactionByID fetches a single transaction, preloading the user.
func (s *Store) GetUnitTransactionByID(ctx context.Context, id uint) (*models.UnitTransaction, error) {
	var tx models.UnitTransaction
	if err := s.DB.WithContext(ctx).Preload("User").First(&tx, id).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

// ListPendingTransactions returns all transactions with status=pending, ordered by created_at desc.
func (s *Store) ListPendingTransactions(ctx context.Context) ([]*models.UnitTransaction, error) {
	var txs []*models.UnitTransaction
	err := s.DB.WithContext(ctx).
		Preload("User").
		Where("status = ?", models.UnitTxStatusPending).
		Order("created_at desc").
		Find(&txs).Error
	return txs, err
}

// ApproveTransaction sets status=approved, updates balance, and sets last_transaction_id — all atomically.
// If overrideUnits is non-nil, it replaces the transaction's units before approval.
// If overrideReason is non-empty, it replaces the transaction's reason before approval.
func (s *Store) ApproveTransaction(ctx context.Context, txID uint, adminID string, overrideUnits *float64, overrideReason string) error {
	return s.DB.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		// 1. Fetch the transaction
		var unitTx models.UnitTransaction
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&unitTx, txID).Error; err != nil {
			return err
		}
		if unitTx.Status != models.UnitTxStatusPending {
			return gorm.ErrInvalidData // already processed
		}

		// 2. Apply overrides if provided
		if overrideUnits != nil {
			unitTx.Units = *overrideUnits
		}
		if overrideReason != "" {
			unitTx.Reason = overrideReason
		}

		// 3. Update transaction status (and possibly units/reason)
		now := time.Now()
		if err := db.Model(&unitTx).Updates(map[string]interface{}{
			"status":      models.UnitTxStatusApproved,
			"approved_by": adminID,
			"approved_at": now,
			"units":       unitTx.Units,
			"reason":      unitTx.Reason,
		}).Error; err != nil {
			return err
		}

		// 4. Upsert balance: create if not exists, add units
		var bal models.UnitBalance
		err := db.Where("user_id = ?", unitTx.UserID).First(&bal).Error
		if err == gorm.ErrRecordNotFound {
			bal = models.UnitBalance{
				UserID:            unitTx.UserID,
				Balance:           unitTx.Units,
				UpdatedAt:         now,
				LastTransactionID: &txID,
			}
			return db.Create(&bal).Error
		}
		if err != nil {
			return err
		}

		return db.Model(&bal).Updates(map[string]interface{}{
			"balance":             bal.Balance + unitTx.Units,
			"updated_at":          now,
			"last_transaction_id": txID,
		}).Error
	})
}

// RejectTransaction sets status=rejected without touching the balance.
func (s *Store) RejectTransaction(ctx context.Context, txID uint, adminID string) error {
	now := time.Now()
	return s.DB.WithContext(ctx).
		Model(&models.UnitTransaction{}).
		Where("id = ? AND status = ?", txID, models.UnitTxStatusPending).
		Updates(map[string]interface{}{
			"status":      models.UnitTxStatusRejected,
			"approved_by": adminID,
			"approved_at": now,
		}).Error
}

// AdminDirectAdjustment creates an auto-approved transaction and updates the balance atomically.
// screenshotURL is optional — pass "" if no screenshot is attached.
func (s *Store) AdminDirectAdjustment(ctx context.Context, userID string, units float64, reason string, txType models.UnitTransactionType, adminID string, screenshotURL string) (*models.UnitTransaction, error) {
	var created models.UnitTransaction
	err := s.DB.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		now := time.Now()
		created = models.UnitTransaction{
			UserID:        userID,
			Type:          txType,
			Units:         units,
			Reason:        reason,
			ScreenshotURL: screenshotURL,
			Status:        models.UnitTxStatusApproved,
			ApprovedBy:    adminID,
			ApprovedAt:    &now,
			CreatedBy:     adminID,
			CreatedAt:     now,
		}
		if err := db.Create(&created).Error; err != nil {
			return err
		}

		// Upsert balance
		var bal models.UnitBalance
		err := db.Where("user_id = ?", userID).First(&bal).Error
		if err == gorm.ErrRecordNotFound {
			bal = models.UnitBalance{
				UserID:            userID,
				Balance:           units,
				UpdatedAt:         now,
				LastTransactionID: &created.ID,
			}
			return db.Create(&bal).Error
		}
		if err != nil {
			return err
		}

		return db.Model(&bal).Updates(map[string]interface{}{
			"balance":             bal.Balance + units,
			"updated_at":          now,
			"last_transaction_id": created.ID,
		}).Error
	})
	return &created, err
}

/* ─────────────── Timeline ─────────────── */

// GetStudentTimeline returns all transactions for a student, ordered by created_at desc.
func (s *Store) GetStudentTimeline(ctx context.Context, userID string) ([]*models.UnitTransaction, error) {
	var txs []*models.UnitTransaction
	err := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&txs).Error
	return txs, err
}

/* ─────────────── Cron helpers ─────────────── */

// HasCronRunForMonth checks if a class_deduction transaction already exists for the given billing period.
func (s *Store) HasCronRunForMonth(ctx context.Context, year, month int) (bool, error) {
	var count int64
	err := s.DB.WithContext(ctx).
		Model(&models.UnitTransaction{}).
		Where("type = ? AND period_year = ? AND period_month = ?",
			models.UnitTxTypeClassDeduction, year, month).
		Count(&count).Error
	return count > 0, err
}

// AttendanceCountByType holds the count of classes per class type for a student in a month.
type AttendanceCountByType struct {
	ClassType string `json:"class_type"`
	Count     int    `json:"count"`
}

// CountAttendancesByTypeForMonth queries the attendances table for a student in a given month,
// grouping by class_type. Returns a slice of (class_type, count) pairs.
func (s *Store) CountAttendancesByTypeForMonth(ctx context.Context, studentID string, year, month int) ([]AttendanceCountByType, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var results []AttendanceCountByType
	err := s.DB.WithContext(ctx).
		Table("attendances").
		Select("class_type, COUNT(*) as count").
		Where("student_id = ? AND date >= ? AND date < ? AND deleted_at IS NULL", studentID, start, end).
		Group("class_type").
		Scan(&results).Error
	return results, err
}

// GetActiveStudentIDs returns the IDs of all active, approved students.
func (s *Store) GetActiveStudentIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := s.DB.WithContext(ctx).
		Model(&models.User{}).
		Where("role = ? AND active = ? AND approved = ?", "student", true, true).
		Pluck("id", &ids).Error
	return ids, err
}
