package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type RatingHandler struct {
	store *store.Store
}

func NewRatingHandler(s serviceStore) *RatingHandler {
	return &RatingHandler{store: s.Store}
}

// GET /ratings/{studentId}/{platform}
// Returns rating history for a specific student on a specific platform.
// Allowed platforms: chesscom, lichess, fide, uscf
func (h *RatingHandler) GetStudentPlatformRatings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	studentID := chi.URLParam(r, "studentId")
	platform := chi.URLParam(r, "platform")

	if studentID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing studentId", nil, nil)
		return
	}

	// Validate platform
	validPlatforms := map[string]bool{"chesscom": true, "lichess": true, "fide": true, "uscf": true}
	if !validPlatforms[platform] {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid platform, must be one of: chesscom, lichess, fide, uscf", nil, nil)
		return
	}

	// Access check: student viewing self, coach/mentor of student, or admin
	if !CanAccessStudentData(ctx, h.store, current, studentID) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	records, err := h.store.GetRatingHistory(ctx, studentID, platform)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching ratings", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", records, nil)
}

// POST /ratings/trigger-scrape
// Admin-only endpoint to manually trigger a full scrape cycle.
func (h *RatingHandler) TriggerScrape(w http.ResponseWriter, r *http.Request) {
	// Run scrapes in background goroutines
	go func() {
		service.RunWeeklyRatingScrape(h.store)
		service.RunMonthlyFIDEScrape(h.store)
	}()

	utils.WriteJSONResponse(w, http.StatusOK, true, "scrape triggered in background", nil, nil)
}

// ---- USCF Extension Endpoints (API-key protected via ScraperHandler.APIKeyAuth) ----

// GET /scraper/uscf/students
// Returns active students who have a USCF ID set.
func (h *RatingHandler) GetUSCFStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	students, err := h.store.GetStudentsWithPlatformUsername(ctx, "uscf")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching USCF students", nil, err.Error())
		return
	}

	// Map to a clean response shape
	type uscfStudent struct {
		UserID string `json:"user_id"`
		USCFID string `json:"uscf_id"`
	}
	out := make([]uscfStudent, len(students))
	for i, s := range students {
		out[i] = uscfStudent{UserID: s.UserID, USCFID: s.Username}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", out, nil)
}

// uploadUSCFRequest is the expected JSON body for POST /scraper/uscf/upload.
type uploadUSCFRequest struct {
	UserID  string       `json:"user_id"`
	Records []uscfRecord `json:"records"`
}

type uscfRecord struct {
	Date   string `json:"date"`   // "YYYY-MM-DD"
	Rating int    `json:"rating"` // classical "after" rating
}

// POST /scraper/uscf/upload
// Receives scraped USCF records from the Chrome extension.
// Performs date-based deduplication: skips records where recorded_at date already exists.
func (h *RatingHandler) UploadUSCFRatings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req uploadUSCFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err.Error())
		return
	}

	if req.UserID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "user_id is required", nil, nil)
		return
	}
	if len(req.Records) == 0 {
		utils.WriteJSONResponse(w, http.StatusOK, true, "no records to insert", map[string]int{"inserted": 0, "skipped": 0}, nil)
		return
	}

	// Get existing dates for this user's USCF records
	existingDates, err := h.store.GetExistingRatingDates(ctx, req.UserID, "uscf")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error checking existing records", nil, err.Error())
		return
	}

	// Filter out duplicates and build insert batch
	var toInsert []models.RatingHistory
	skipped := 0
	for _, rec := range req.Records {
		if existingDates[rec.Date] {
			skipped++
			continue
		}

		recordedAt, err := time.Parse("2006-01-02", rec.Date)
		if err != nil {
			skipped++
			continue
		}

		toInsert = append(toInsert, models.RatingHistory{
			UserID:     req.UserID,
			Platform:   "uscf",
			RatingType: "classical",
			Rating:     rec.Rating,
			RecordedAt: recordedAt,
			CreatedAt:  time.Now(),
		})
	}

	if len(toInsert) > 0 {
		if err := h.store.BulkCreateRatingRecords(ctx, toInsert); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error inserting records", nil, err.Error())
			return
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, fmt.Sprintf("inserted %d, skipped %d", len(toInsert), skipped), map[string]int{
		"inserted": len(toInsert),
		"skipped":  skipped,
	}, nil)
}

