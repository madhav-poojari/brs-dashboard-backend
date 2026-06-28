package service

import (
	"log"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/robfig/cron/v3"
)

// StartNotificationCrons creates and starts cron jobs for notification generation.
// Returns the cron instance so the caller can stop it on shutdown.
// Daily at 6:00 AM UTC: check joining anniversaries, tournament participation, and milestones.
func StartNotificationCrons(s *store.Store) *cron.Cron {
	c := cron.New(cron.WithLocation(time.UTC))

	// Daily at 6:00 AM UTC
	_, err := c.AddFunc("0 6 * * *", func() {
		log.Println("[NotificationCron] Starting daily notification checks...")
		CheckJoiningAnniversaries(s)
		CheckTournamentParticipation(s)
		CheckRatingMilestones(s)
		log.Println("[NotificationCron] Daily notification checks completed")
	})
	if err != nil {
		log.Printf("[NotificationCron] Failed to schedule daily notifications: %v", err)
	}

	c.Start()
	log.Println("[NotificationCron] Cron scheduler started (daily: 6AM UTC)")
	return c
}
