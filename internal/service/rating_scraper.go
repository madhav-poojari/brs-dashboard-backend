package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/madhava-poojari/dashboard-api/internal/models"
)

const (
	chesscomUserAgent = "BRSChessAcademy/1.0 (contact: brs.chessacademy@gmail.com)"
	chesscomDelayMin  = 3 * time.Second // minimum delay between Chess.com requests
	chesscomDelayMax  = 6 * time.Second // maximum delay (randomized to avoid rate limiting)
	chesscomRetries   = 2               // max retries per failed archive request
)

// Shared HTTP client with timeout — prevents hung connections from blocking forever.
var scrapeClient = &http.Client{
	Timeout: 30 * time.Second,
}

// randomDelay sleeps for a random duration between chesscomDelayMin and chesscomDelayMax.
func randomDelay() {
	jitter := time.Duration(rand.Int63n(int64(chesscomDelayMax - chesscomDelayMin)))
	time.Sleep(chesscomDelayMin + jitter)
}

// ---- Chess.com Scraper ----

// ChesscomStatsResponse represents the relevant part of the Chess.com stats API response.
type ChesscomStatsResponse struct {
	ChessRapid *struct {
		Last *struct {
			Rating int   `json:"rating"`
			Date   int64 `json:"date"` // unix timestamp
		} `json:"last"`
	} `json:"chess_rapid"`
}

// ScrapeChesscomCurrent fetches the current rapid rating from Chess.com stats API.
func ScrapeChesscomCurrent(username string) (*models.RatingHistory, error) {
	url := fmt.Sprintf("https://api.chess.com/pub/player/%s/stats", strings.ToLower(username))
	body, err := chesscomGet(url)
	if err != nil {
		return nil, fmt.Errorf("chesscom stats fetch: %w", err)
	}
	defer body.Close()

	var stats ChesscomStatsResponse
	if err := json.NewDecoder(body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("chesscom stats decode: %w", err)
	}

	if stats.ChessRapid == nil || stats.ChessRapid.Last == nil {
		return nil, fmt.Errorf("no rapid rating found for %s", username)
	}

	return &models.RatingHistory{
		Platform:   "chesscom",
		RatingType: "rapid",
		Rating:     stats.ChessRapid.Last.Rating,
		RecordedAt: time.Unix(stats.ChessRapid.Last.Date, 0).UTC(),
		CreatedAt:  time.Now(),
	}, nil
}

// ChesscomArchivesResponse represents the archives API response.
type ChesscomArchivesResponse struct {
	Archives []string `json:"archives"`
}

// ChesscomGamesResponse represents a month's games API response.
type ChesscomGamesResponse struct {
	Games []ChesscomGame `json:"games"`
}

// ChesscomGame represents a single game from the Chess.com games API.
type ChesscomGame struct {
	TimeClass string `json:"time_class"`
	EndTime   int64  `json:"end_time"`
	White     struct {
		Rating   int    `json:"rating"`
		Username string `json:"username"`
	} `json:"white"`
	Black struct {
		Rating   int    `json:"rating"`
		Username string `json:"username"`
	} `json:"black"`
}

// ScrapeChesscomHistory fetches all historical rapid games and produces weekly-averaged rating records.
func ScrapeChesscomHistory(username string) ([]models.RatingHistory, error) {
	lowerUser := strings.ToLower(username)

	// Step 1: Get archive list
	archiveURL := fmt.Sprintf("https://api.chess.com/pub/player/%s/games/archives", lowerUser)
	body, err := chesscomGet(archiveURL)
	if err != nil {
		return nil, fmt.Errorf("chesscom archives fetch: %w", err)
	}

	var archives ChesscomArchivesResponse
	if err := json.NewDecoder(body).Decode(&archives); err != nil {
		body.Close()
		return nil, fmt.Errorf("chesscom archives decode: %w", err)
	}
	body.Close()

	if len(archives.Archives) == 0 {
		return nil, nil // no games at all
	}

	// Step 2: Fetch each month's games (with 2s delay between requests)
	type weekKey struct {
		Year int
		Week int
	}
	type weekData struct {
		TotalRating int
		Count       int
		LastTime    time.Time
	}
	weekMap := make(map[weekKey]*weekData)

	for i, archiveMonthURL := range archives.Archives {
		// Randomized delay between requests to avoid rate limiting
		if i > 0 {
			randomDelay()
		}

		// Retry logic for transient failures (connection resets, etc.)
		var monthBody io.ReadCloser
		var fetchErr error
		for attempt := 0; attempt <= chesscomRetries; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(attempt*5) * time.Second
				log.Printf("chesscom retry %d/%d for %s (waiting %s)", attempt, chesscomRetries, archiveMonthURL, backoff)
				time.Sleep(backoff)
			}
			monthBody, fetchErr = chesscomGet(archiveMonthURL)
			if fetchErr == nil {
				break
			}
		}
		if fetchErr != nil {
			log.Printf("chesscom games fetch error for %s (after %d retries): %v", archiveMonthURL, chesscomRetries, fetchErr)
			continue // skip this month, don't fail the whole backfill
		}

		var gamesResp ChesscomGamesResponse
		if err := json.NewDecoder(monthBody).Decode(&gamesResp); err != nil {
			monthBody.Close()
			log.Printf("chesscom games decode error for %s: %v", archiveMonthURL, err)
			continue
		}
		monthBody.Close()

		for _, game := range gamesResp.Games {
			if game.TimeClass != "rapid" {
				continue
			}

			gameTime := time.Unix(game.EndTime, 0).UTC()
			year, week := gameTime.ISOWeek()
			key := weekKey{Year: year, Week: week}

			// Determine the player's rating in this game
			var playerRating int
			if strings.EqualFold(game.White.Username, lowerUser) {
				playerRating = game.White.Rating
			} else {
				playerRating = game.Black.Rating
			}

			if playerRating == 0 {
				continue
			}

			wd, exists := weekMap[key]
			if !exists {
				wd = &weekData{}
				weekMap[key] = wd
			}
			wd.TotalRating += playerRating
			wd.Count++
			if gameTime.After(wd.LastTime) {
				wd.LastTime = gameTime
			}
		}
	}

	// Step 3: Convert to records
	records := make([]models.RatingHistory, 0, len(weekMap))
	for _, wd := range weekMap {
		avgRating := wd.TotalRating / wd.Count
		records = append(records, models.RatingHistory{
			Platform:   "chesscom",
			RatingType: "rapid",
			Rating:     avgRating,
			RecordedAt: wd.LastTime,
			CreatedAt:  time.Now(),
		})
	}

	// Sort by recorded_at
	sort.Slice(records, func(i, j int) bool {
		return records[i].RecordedAt.Before(records[j].RecordedAt)
	})

	return records, nil
}

// chesscomGet performs an HTTP GET with the proper User-Agent header.
func chesscomGet(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chesscomUserAgent)

	resp, err := scrapeClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

// ---- Lichess Scraper ----

// LichessUserResponse represents the relevant part of the Lichess user API response.
type LichessUserResponse struct {
	Perfs struct {
		Rapid *struct {
			Rating int `json:"rating"`
		} `json:"rapid"`
	} `json:"perfs"`
}

// ScrapeLichessCurrent fetches the current rapid rating from the Lichess user API.
func ScrapeLichessCurrent(username string) (*models.RatingHistory, error) {
	url := fmt.Sprintf("https://lichess.org/api/user/%s", username)
	resp, err := scrapeClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("lichess user fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lichess user HTTP %d", resp.StatusCode)
	}

	var user LichessUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("lichess user decode: %w", err)
	}

	if user.Perfs.Rapid == nil {
		return nil, fmt.Errorf("no rapid rating found for %s", username)
	}

	return &models.RatingHistory{
		Platform:   "lichess",
		RatingType: "rapid",
		Rating:     user.Perfs.Rapid.Rating,
		RecordedAt: time.Now().UTC(),
		CreatedAt:  time.Now(),
	}, nil
}

// LichessRatingHistoryEntry represents one time control from the rating-history API.
type LichessRatingHistoryEntry struct {
	Name   string  `json:"name"`
	Points [][]int `json:"points"` // [year, month(0-indexed), day, rating]
}

// ScrapeLichessHistory fetches all historical rapid rating points from Lichess.
// Groups by ISO week, taking the last point per week.
func ScrapeLichessHistory(username string) ([]models.RatingHistory, error) {
	url := fmt.Sprintf("https://lichess.org/api/user/%s/rating-history", username)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("lichess rating-history fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lichess rating-history HTTP %d", resp.StatusCode)
	}

	var entries []LichessRatingHistoryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("lichess rating-history decode: %w", err)
	}

	// Find the "Rapid" entry
	var rapidEntry *LichessRatingHistoryEntry
	for i := range entries {
		if entries[i].Name == "Rapid" {
			rapidEntry = &entries[i]
			break
		}
	}

	if rapidEntry == nil || len(rapidEntry.Points) == 0 {
		return nil, nil // no rapid history
	}

	// Group by ISO week, take the last point per week
	type weekKey struct {
		Year int
		Week int
	}
	weekMap := make(map[weekKey]models.RatingHistory)

	for _, point := range rapidEntry.Points {
		if len(point) < 4 {
			continue
		}
		year := point[0]
		month := point[1] + 1 // 0-indexed → 1-indexed
		day := point[2]
		rating := point[3]

		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		isoYear, isoWeek := t.ISOWeek()
		key := weekKey{Year: isoYear, Week: isoWeek}

		existing, exists := weekMap[key]
		if !exists || t.After(existing.RecordedAt) {
			weekMap[key] = models.RatingHistory{
				Platform:   "lichess",
				RatingType: "rapid",
				Rating:     rating,
				RecordedAt: t,
				CreatedAt:  time.Now(),
			}
		}
	}

	records := make([]models.RatingHistory, 0, len(weekMap))
	for _, rh := range weekMap {
		records = append(records, rh)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].RecordedAt.Before(records[j].RecordedAt)
	})

	return records, nil
}

// ---- FIDE Scraper ----

// ScrapeFIDECurrent fetches the latest STD rating from the FIDE profile page.
func ScrapeFIDECurrent(fideID string) (*models.RatingHistory, error) {
	records, err := scrapeFIDETable(fideID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no FIDE rating data found for %s", fideID)
	}
	// First record is the latest (table is newest-first)
	latest := records[0]
	return &latest, nil
}

// ScrapeFIDEHistory fetches all historical STD ratings from the FIDE profile page.
func ScrapeFIDEHistory(fideID string) ([]models.RatingHistory, error) {
	records, err := scrapeFIDETable(fideID)
	if err != nil {
		return nil, err
	}
	// Reverse to chronological order (table is newest-first)
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	return records, nil
}

// scrapeFIDETable fetches and parses the FIDE profile rating history table.
// Returns records in the order they appear on the page (newest first).
func scrapeFIDETable(fideID string) ([]models.RatingHistory, error) {
	url := fmt.Sprintf("https://ratings.fide.com/profile/%s", fideID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fide profile fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fide profile HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fide html parse: %w", err)
	}

	var records []models.RatingHistory

	doc.Find("table.profile-table_calc tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}

		// First cell: Period (e.g., "2026-Apr")
		periodStr := strings.TrimSpace(cells.Eq(0).Text())
		periodStr = strings.ReplaceAll(periodStr, "\u00a0", "") // strip &nbsp;

		// Second cell: STD. RATING
		ratingStr := strings.TrimSpace(cells.Eq(1).Text())
		ratingStr = strings.ReplaceAll(ratingStr, "\u00a0", "")

		if periodStr == "" || ratingStr == "" {
			return
		}

		rating, err := strconv.Atoi(ratingStr)
		if err != nil || rating == 0 {
			return // skip rows without a valid STD rating
		}

		// Parse period: "YYYY-Mon" format
		recordedAt, err := parseFIDEPeriod(periodStr)
		if err != nil {
			log.Printf("fide period parse error for %q: %v", periodStr, err)
			return
		}

		records = append(records, models.RatingHistory{
			Platform:   "fide",
			RatingType: "classical",
			Rating:     rating,
			RecordedAt: recordedAt,
			CreatedAt:  time.Now(),
		})
	})

	return records, nil
}

// parseFIDEPeriod parses a FIDE period string like "2026-Apr" into a time.Time (1st of that month).
func parseFIDEPeriod(period string) (time.Time, error) {
	// Try standard format "2006-Jan"
	t, err := time.Parse("2006-Jan", period)
	if err == nil {
		return t, nil
	}
	// Fallback: try "2006-January"
	t, err = time.Parse("2006-January", period)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse fide period %q", period)
}
