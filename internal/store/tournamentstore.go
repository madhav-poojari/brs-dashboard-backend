package store

import (
	"context"
	"fmt"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Hardcoded distances (miles) used for scraping
var ScrapingDistances = []int{10, 20, 30}

// SyncZipcodesFromUserDetails ensures the zipcode_scrape_scopes table
// contains 3 rows per active student zipcode (one per distance).
func (s *Store) SyncZipcodesFromUserDetails(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, dist := range ScrapingDistances {
			if err := tx.Exec(`
				INSERT INTO zipcode_scrape_scopes (zipcode, distance)
				SELECT DISTINCT ud.zipcode, $1::INT
				FROM user_details ud
				JOIN users u ON u.id = ud.user_id
				WHERE u.active = true AND u.approved = true
				  AND u.role = 'student'
				  AND ud.zipcode IS NOT NULL AND ud.zipcode != ''
				ON CONFLICT (zipcode, distance) DO NOTHING
			`, dist).Error; err != nil {
				return err
			}
		}

		// Remove scopes for zipcodes with no active students
		if err := tx.Exec(`
			DELETE FROM zipcode_scrape_scopes
			WHERE zipcode NOT IN (
				SELECT DISTINCT ud.zipcode
				FROM user_details ud
				JOIN users u ON u.id = ud.user_id
				WHERE u.active = true AND u.approved = true
				  AND u.role = 'student'
				  AND ud.zipcode IS NOT NULL AND ud.zipcode != ''
			)
		`).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetZipcodesForScraping returns (zipcode, distance) pairs that need scraping:
// last_scraped is NULL or older than 24 hours.
func (s *Store) GetZipcodesForScraping(ctx context.Context) ([]models.ZipcodeScrapeScope, error) {
	var scopes []models.ZipcodeScrapeScope
	cutoff := time.Now().Add(-24 * time.Hour)

	if err := s.DB.WithContext(ctx).
		Where("last_scraped IS NULL OR last_scraped < ?", cutoff).
		Order("zipcode ASC, distance ASC").
		Find(&scopes).Error; err != nil {
		return nil, err
	}
	return scopes, nil
}

// UpsertTournaments upserts tournament records and links them to a (zipcode, distance).
// Uses ON CONFLICT (url_path) DO UPDATE to deduplicate tournaments.
func (s *Store) UpsertTournaments(ctx context.Context, zipcode string, distance int, tournaments []models.Tournament) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing junction rows for this (zipcode, distance)
		if err := tx.Where("zipcode = ? AND distance = ?", zipcode, distance).
			Delete(&models.TournamentWithinRadius{}).Error; err != nil {
			return err
		}

		// Upsert each tournament and create junction rows
		for _, t := range tournaments {
			t.ID = 0
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "url_path"}},
				DoUpdates: clause.AssignmentColumns([]string{"title", "city", "state", "dates", "start_date", "organizer", "description"}),
			}).Create(&t).Error; err != nil {
				return err
			}

			junc := models.TournamentWithinRadius{
				Zipcode:      zipcode,
				Distance:     distance,
				TournamentID: t.ID,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&junc).Error; err != nil {
				return err
			}
		}

		// Update last_scraped
		now := time.Now()
		if err := tx.Model(&models.ZipcodeScrapeScope{}).
			Where("zipcode = ? AND distance = ?", zipcode, distance).
			Update("last_scraped", now).Error; err != nil {
			return err
		}

		return nil
	})
}

// TournamentsByDistance is the response shape for grouped tournaments.
type TournamentsByDistance struct {
	Distance    int                 `json:"distance"`
	Tournaments []models.Tournament `json:"tournaments"`
}

// GetTournamentsByUserID returns tournaments grouped by distance with exclusive grouping.
func (s *Store) GetTournamentsByUserID(ctx context.Context, userID string) ([]TournamentsByDistance, error) {
	var ud models.UserDetails
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&ud).Error; err != nil {
		return nil, err
	}
	if ud.Zipcode == "" {
		return []TournamentsByDistance{}, nil
	}

	seen := make(map[uint]bool)
	result := make([]TournamentsByDistance, 0, len(ScrapingDistances))

	for _, dist := range ScrapingDistances {
		var junctions []models.TournamentWithinRadius
		if err := s.DB.WithContext(ctx).
			Where("zipcode = ? AND distance = ?", ud.Zipcode, dist).
			Find(&junctions).Error; err != nil {
			return nil, err
		}

		var ids []uint
		for _, j := range junctions {
			if !seen[j.TournamentID] {
				ids = append(ids, j.TournamentID)
			}
		}

		var tournaments []models.Tournament
		if len(ids) > 0 {
			if err := s.DB.WithContext(ctx).
				Where("id IN ?", ids).
				Order("start_date ASC NULLS LAST").
				Find(&tournaments).Error; err != nil {
				return nil, fmt.Errorf("fetching tournaments for distance %d: %w", dist, err)
			}
			for _, t := range tournaments {
				seen[t.ID] = true
			}
		}

		result = append(result, TournamentsByDistance{
			Distance:    dist,
			Tournaments: tournaments,
		})
	}

	return result, nil
}
