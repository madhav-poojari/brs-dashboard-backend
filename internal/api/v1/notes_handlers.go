package v1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

// NotesHandler holds store pointer
type NotesHandler struct {
	store *store.Store
}

// NewNotesHandler matches your AuthHandler style
func NewNotesHandler(s serviceStore) *NotesHandler {
	return &NotesHandler{store: s.Store}
}

// POST /api/v1/notes
func (h *NotesHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID         string                 `json:"user_id"`
		Title          string                 `json:"title"`
		Description    string                 `json:"description"`
		PrimaryTag     string                 `json:"primary_tag"`
		Tags           []string               `json:"tags"`
		Visibility     int                    `json:"visibility"`
		AdditionalInfo map[string]interface{} `json:"additional_info"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}
	noteUser, err := h.store.GetUserByID(ctx, req.UserID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "No such user", nil, err.Error())
		return
	}
	if noteUser.Role == models.RoleCoach {
		if !(current.Role == models.RoleAdmin || current.Role == models.RoleMentor) {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
			return
		}
	}
	if noteUser.Role == models.RoleStudent {
		// only related (student themselves, their coach/mentor, or admin) may create note for that user
		ok, err := h.store.IsRelatedStudent(ctx, current.ID, req.UserID)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		if !ok {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
			return
		}
	}

	// tag restrictions
	if !store.TagAllowedForRole(req.PrimaryTag, current.Role) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "tag restricted", nil, nil)
		return
	}

	n := &models.Note{
		UserID:         req.UserID,
		Title:          req.Title,
		Description:    req.Description,
		PrimaryTag:     req.PrimaryTag,
		Tags:           utils.DatatypesJSONFromStrings(req.Tags),
		IsStarred:      false,
		AdditionalInfo: utils.DatatypesJSONFromMap(req.AdditionalInfo),
		Visibility:     req.Visibility,
		CreatedBy:      current.ID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.store.CreateNote(ctx, n); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "create note failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusCreated, true, "created", n, nil)
}

// POST /api/v1/lesson-plans
func (h *NotesHandler) CreateLessonPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      string   `json:"user_id"`
		Title       string   `json:"title"`
		Description []string `json:"description"`
		StartDate   string   `json:"start_date"`
		EndDate     string   `json:"end_date"`
		Result      string   `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// only mentor (for that user) or admin can create lesson plan
	if current.Role != "admin" {
		isMentor, err := h.store.IsMentorOf(ctx, current.ID, req.UserID)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		if !isMentor {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "only mentor or admin can create lesson plan", nil, nil)
			return
		}
	}

	lp := &models.LessonPlan{
		UserID:      req.UserID,
		Title:       req.Title,
		Description: utils.DatatypesJSONFromStrings(req.Description),
		Result:      req.Result,
		Active:      true,
		CreatedBy:   current.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
		lp.StartDate = t
	}
	if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
		lp.EndDate = t
	}

	if err := h.store.CreateLessonPlan(ctx, lp); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "create lesson plan failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusCreated, true, "lesson plan created", lp, nil)
}

// PATCH /api/v1/notes/lesson-plans/{id}
func (h *NotesHandler) UpdateLessonPlan(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "id")
	var req struct {
		Title       *string  `json:"title"`
		Description []string `json:"description"`
		StartDate   *string  `json:"start_date"`
		EndDate     *string  `json:"end_date"`
		Result      *string  `json:"result"`
		Active      *bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// Fetch the lesson plan to check ownership/permissions
	var lp models.LessonPlan
	if err := h.store.DB.WithContext(ctx).First(&lp, "id = ?", planID).Error; err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "lesson plan not found", nil, err.Error())
		return
	}

	// Permission Check: Admin or Mentor Coach of the student
	// Only Admin and Mentor Coach can update the lesson plan
	allowed := false
	if current.Role == "admin" {
		allowed = true
	} else {
		isMentor, err := h.store.IsMentorOf(ctx, current.ID, lp.UserID)
		if err == nil && isMentor {
			allowed = true
		}
	}

	if !allowed {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// Build updates map
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = utils.DatatypesJSONFromStrings(req.Description)
	}
	if req.Result != nil {
		updates["result"] = *req.Result
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.StartDate != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartDate); err == nil {
			updates["start_date"] = t
		}
	}
	if req.EndDate != nil {
		if t, err := time.Parse(time.RFC3339, *req.EndDate); err == nil {
			updates["end_date"] = t
		}
	}

	if len(updates) > 0 {
		if err := h.store.UpdateLessonPlanFields(ctx, planID, updates); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update failed", nil, err.Error())
			return
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "lesson plan updated", nil, nil)
}

// GET /api/v1/notes/
func (h *NotesHandler) GetNotesByUser(w http.ResponseWriter, r *http.Request) {
	// userID := chi.URLParam(r, "id")
	userID := r.URL.Query().Get("user_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	noteUser, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "No such user", nil, err.Error())
		return
	}
	if noteUser.Role == models.RoleCoach {
		if !(current.Role == models.RoleAdmin || current.Role == models.RoleMentor) {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
			return
		}
	}
	if noteUser.Role == models.RoleStudent {
		// only related (student themselves, their coach/mentor, or admin) may create note for that user
		ok, err := h.store.IsRelatedStudent(ctx, current.ID, userID)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		if !ok {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
			return
		}
	}

	notes, lp, err := h.store.GetNotesByStudent(ctx, userID, limit, offset)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching notes", nil, err.Error())
		return
	}

	// filter by visibility
	filtered := []*models.Note{}
	for _, n := range notes {
		if h.store.CanAccessNoteForRequester(ctx, current, n) {
			filtered = append(filtered, n)
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", map[string]interface{}{"lesson_plan": lp, "notes": filtered}, nil)
}

// PATCH /api/v1/notes/{id}
func (h *NotesHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	noteID := chi.URLParam(r, "id")
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	var note models.Note
	if err := h.store.DB.WithContext(ctx).First(&note, "id = ?", noteID).Error; err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, err.Error())
		return
	}

	if !h.store.CanAccessNoteForRequester(ctx, current, &note) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	if err := h.store.UpdateNoteFields(ctx, noteID, payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "updated", nil, nil)
}

// DELETE /api/v1/notes/{id}
func (h *NotesHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	noteID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	var note models.Note
	if err := h.store.DB.WithContext(ctx).First(&note, "id = ?", noteID).Error; err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, err.Error())
		return
	}
	if !h.store.CanAccessNoteForRequester(ctx, current, &note) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}
	if err := h.store.DeleteNoteSoft(ctx, noteID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "delete failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "deleted", nil, nil)
}
