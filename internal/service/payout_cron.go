package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/robfig/cron/v3"
)

/* ─────────────── Configurable Constants ─────────────── */
// Change these values to adjust unit costs and cron schedule.
// No need to modify any other code.

const (
	// PayoutCronDay is the day of the month the deduction cron runs (processes previous month).
	PayoutCronDay = 5

	// Class unit costs — how many units each class type costs.
	RegularClassUnits      = 1.0
	GameSessionClassUnits  = 0.75
	DualClassUnits         = 0.5
	SubstitutionClassUnits = 0.75

	// MaxAdjustmentUnits is the max units an admin can add in a single bonus/adjustment.
	// This prevents accidental large credit entries.
	MaxAdjustmentUnits = 3.0

	// Timeout for the entire monthly payout deduction run.
	payoutCronTimeout = 10 * time.Minute
)

// ClassTypeUnitCost maps each attendance class type to its unit cost.
var ClassTypeUnitCost = map[models.AttendanceClassType]float64{
	models.AttendanceClassTypeRegular:      RegularClassUnits,
	models.AttendanceClassTypeGameSession:  GameSessionClassUnits,
	models.AttendanceClassTypeDual:         DualClassUnits,
	models.AttendanceClassTypeSubstitution: SubstitutionClassUnits,
}

/* ─────────────── Cron Setup ─────────────── */

// StartPayoutCron creates and starts the cron scheduler for monthly payout deductions.
// Returns the cron instance so the caller can stop it on shutdown.
func StartPayoutCron(s *store.Store) *cron.Cron {
	c := cron.New(cron.WithLocation(time.UTC))

	// Run on the PayoutCronDay-th of each month at 2:00 AM UTC
	schedule := fmt.Sprintf("0 2 %d * *", PayoutCronDay)
	_, err := c.AddFunc(schedule, func() {
		log.Println("[PayoutCron] Starting monthly payout deduction...")
		RunMonthlyPayoutDeduction(s)
		log.Println("[PayoutCron] Monthly payout deduction completed")
	})
	if err != nil {
		log.Printf("[PayoutCron] Failed to schedule payout cron: %v", err)
	}

	c.Start()
	log.Printf("[PayoutCron] Cron scheduler started (day %d of month, 2AM UTC)", PayoutCronDay)
	return c
}

/* ─────────────── Monthly Deduction Logic ─────────────── */

// RunMonthlyPayoutDeduction processes the previous month's classes for all active students
// and creates pending deduction transactions for admin approval.
func RunMonthlyPayoutDeduction(s *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), payoutCronTimeout)
	defer cancel()

	// Determine the previous month
	now := time.Now().UTC()
	prevMonth := now.AddDate(0, -1, 0)
	year := prevMonth.Year()
	month := int(prevMonth.Month())

	// Check if already processed
	alreadyRun, err := s.HasCronRunForMonth(ctx, year, month)
	if err != nil {
		log.Printf("[PayoutCron] Error checking cron history: %v", err)
		return
	}
	if alreadyRun {
		log.Printf("[PayoutCron] Already processed %d-%02d, skipping", year, month)
		return
	}

	// Get all active students
	studentIDs, err := s.GetActiveStudentIDs(ctx)
	if err != nil {
		log.Printf("[PayoutCron] Error fetching students: %v", err)
		return
	}

	created := 0
	skipped := 0

	for _, studentID := range studentIDs {
		if ctx.Err() != nil {
			log.Println("[PayoutCron] Timeout reached, stopping")
			break
		}

		// Count attendances by type for this student in the billing month
		counts, err := s.CountAttendancesByTypeForMonth(ctx, studentID, year, month)
		if err != nil {
			log.Printf("[PayoutCron] Error counting attendances for %s: %v", studentID, err)
			continue
		}

		if len(counts) == 0 {
			skipped++
			continue
		}

		// Calculate total units and build breakdown in reason
		totalUnits := 0.0
		reason := ""

		for _, c := range counts {
			classType := models.AttendanceClassType(c.ClassType)
			unitCost, ok := ClassTypeUnitCost[classType]
			if !ok {
				log.Printf("[PayoutCron] Unknown class type %q for student %s, using 1.0", c.ClassType, studentID)
				unitCost = 1.0
			}
			classTotal := unitCost * float64(c.Count)
			totalUnits += classTotal

			if reason != "" {
				reason += ", "
			}
			reason += fmt.Sprintf("%d %s × %.2f", c.Count, c.ClassType, unitCost)
		}

		if totalUnits == 0 {
			skipped++
			continue
		}

		// Create pending deduction transaction (units is negative for deductions)
		tx := &models.UnitTransaction{
			UserID:      studentID,
			Type:        models.UnitTxTypeClassDeduction,
			Units:       -totalUnits, // negative = deduction
			Reason:      reason,
			Status:      models.UnitTxStatusPending,
			PeriodYear:  year,
			PeriodMonth: month,
			CreatedBy:   "system",
		}

		if err := s.CreateUnitTransaction(ctx, tx); err != nil {
			log.Printf("[PayoutCron] Error creating transaction for %s: %v", studentID, err)
			continue
		}
		created++
	}

	log.Printf("[PayoutCron] Processed %d-%02d: %d deductions created, %d students skipped (no classes)",
		year, month, created, skipped)
}
