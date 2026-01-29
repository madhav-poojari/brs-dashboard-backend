package v1

import (
	"context"
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

type AttendanceHandler struct {
	store *store.Store
}

func NewAttendanceHandler(s serviceStore) *AttendanceHandler {
	return &AttendanceHandler{store: s.Store}
}

func parseDateFlexible(s string) (time.Time, error) {
	// Prefer YYYY-MM-DD for "date" fields; allow RFC3339 as fallback.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func classTypeValid(ct models.AttendanceClassType) bool {
	switch ct {
	case models.AttendanceClassTypeRegular,
		models.AttendanceClassTypeDual,
		models.AttendanceClassTypeGameSession,
		models.AttendanceClassTypeSubstitution:
		return true
	default:
		return false
	}
}

// POST /attendances
func (h *AttendanceHandler) CreateAttendance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StudentID  string                     `json:"student_id"`
		StudentIDs []string                   `json:"student_ids"`
		CoachID    string                     `json:"coach_id"`
		ClassType  models.AttendanceClassType `json:"class_type"`
		Date       string                     `json:"date"`
		SessionID  string                     `json:"session_id"`

		ClassHighlights string `json:"class_highlights"`
		Homework        string `json:"homework"`
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
	if !classTypeValid(req.ClassType) {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid class_type", nil, nil)
		return
	}
	if req.Date == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "date is required", nil, nil)
		return
	}
	date, err := parseDateFlexible(req.Date)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid date", nil, err.Error())
		return
	}

	studentIDs := []string{}
	if len(req.StudentIDs) > 0 {
		for _, sid := range req.StudentIDs {
			if sid != "" {
				studentIDs = append(studentIDs, sid)
			}
		}
	} else if req.StudentID != "" {
		studentIDs = append(studentIDs, req.StudentID)
	}
	if len(studentIDs) == 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "student_id is required", nil, nil)
		return
	}

	coachID := req.CoachID
	if current.Role == models.RoleCoach {
		coachID = current.ID
	}
	if coachID == "" {
		coachID = current.ID
	}

	// Permission checks
	switch current.Role {
	case models.RoleAdmin:
		// ok
	case models.RoleCoach:
		// for regular/dual, ensure student belongs to coach
		if req.ClassType == models.AttendanceClassTypeRegular || req.ClassType == models.AttendanceClassTypeDual {
			for _, sid := range studentIDs {
				ok, err := h.store.IsCoachOf(ctx, current.ID, sid)
				if err != nil {
					utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
					return
				}
				if !ok {
					utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
					return
				}
			}
		}
	case models.RoleMentor:
		// Mentor can create for their coaches/students.
		// For substitution/game_session with free-text student_id, rely on coach relationship.
		if req.ClassType == models.AttendanceClassTypeSubstitution || req.ClassType == models.AttendanceClassTypeGameSession {
			ok, err := h.store.IsMentorOfCoach(ctx, current.ID, coachID)
			if err != nil {
				utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
				return
			}
			if !(ok || coachID == current.ID) {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
				return
			}
		} else {
			for _, sid := range studentIDs {
				ok, err := h.store.IsRelatedStudent(ctx, current.ID, sid)
				if err != nil {
					utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
					return
				}
				if !ok {
					// allow if mentor owns the coach even if student relation not present
					isMentorCoach, err := h.store.IsMentorOfCoach(ctx, current.ID, coachID)
					if err != nil {
						utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
						return
					}
					if !(isMentorCoach || coachID == current.ID) {
						utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
						return
					}
				}
			}
		}
	default:
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	created := []*models.Attendance{}
	for _, sid := range studentIDs {
		a := &models.Attendance{
			StudentID:       sid,
			CoachID:         coachID,
			ClassType:       req.ClassType,
			Date:            date,
			SessionID:       req.SessionID,
			IsVerified:      false, // always default false on create
			ClassHighlights: req.ClassHighlights,
			Homework:        req.Homework,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := h.store.CreateAttendance(ctx, a); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "create attendance failed", nil, err.Error())
			return
		}
		created = append(created, a)
	}
	if len(created) == 1 {
		utils.WriteJSONResponse(w, http.StatusCreated, true, "created", created[0], nil)
		return
	}
	utils.WriteJSONResponse(w, http.StatusCreated, true, "created", created, nil)
}

// GET /attendances?month=&year=&student_id=&coach_id=&class_type=&session_id=&is_verified=
func (h *AttendanceHandler) ListAttendances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	q := r.URL.Query()
	monthStr := q.Get("month")
	yearStr := q.Get("year")
	if monthStr == "" || yearStr == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "month and year are required", nil, nil)
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid month", nil, nil)
		return
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 1970 || year > 2100 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid year", nil, nil)
		return
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	studentID := q.Get("student_id")
	coachID := q.Get("coach_id")
	sessionID := q.Get("session_id")
	classTypeStr := q.Get("class_type")
	isVerifiedStr := q.Get("is_verified")

	var classType *models.AttendanceClassType
	if classTypeStr != "" {
		ct := models.AttendanceClassType(classTypeStr)
		if !classTypeValid(ct) {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid class_type", nil, nil)
			return
		}
		classType = &ct
	}

	var isVerified *bool
	if isVerifiedStr != "" {
		b, err := strconv.ParseBool(isVerifiedStr)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid is_verified", nil, err.Error())
			return
		}
		isVerified = &b
	}

	f := store.AttendanceListFilter{
		StartDate:  start,
		EndDate:    end,
		ClassType:  classType,
		IsVerified: isVerified,
	}
	if studentID != "" {
		f.StudentID = &studentID
	}
	if coachID != "" {
		f.CoachID = &coachID
	}
	if sessionID != "" {
		f.SessionID = &sessionID
	}

	// Role scoping
	switch current.Role {
	case models.RoleAdmin:
		// no extra scoping
	case models.RoleCoach:
		// force coach_id to self
		self := current.ID
		f.CoachID = &self
	case models.RoleMentor:
		self := current.ID
		f.MentorID = &self

		// If mentor explicitly filters by coach_id, ensure it's one of their coaches (or themselves).
		if coachID != "" && coachID != current.ID {
			ok, err := h.store.IsMentorOfCoach(ctx, current.ID, coachID)
			if err != nil {
				utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
				return
			}
			if !ok {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
				return
			}
		}
		// If mentor explicitly filters by student_id, ensure they are related to that student.
		if studentID != "" {
			ok, err := h.store.IsRelatedStudent(ctx, current.ID, studentID)
			if err != nil {
				utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
				return
			}
			if !ok {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
				return
			}
		}
	default:
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	out, err := h.store.ListAttendances(ctx, f)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching attendances", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", out, nil)
}

// GET /attendances/{id}
func (h *AttendanceHandler) GetAttendance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}
	idStr := chi.URLParam(r, "id")
	idU64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid id", nil, err.Error())
		return
	}
	a, err := h.store.GetAttendanceByID(ctx, uint(idU64))
	if err != nil {
		if store.IsNotFound(err) {
			utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
			return
		}
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
		return
	}

	if !h.canAccessAttendance(ctx, current, a) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", a, nil)
}

// PATCH /attendances/{id}
func (h *AttendanceHandler) UpdateAttendance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	idU64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid id", nil, err.Error())
		return
	}

	existing, err := h.store.GetAttendanceByID(ctx, uint(idU64))
	if err != nil {
		if store.IsNotFound(err) {
			utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
			return
		}
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
		return
	}
	if !h.canAccessAttendance(ctx, current, existing) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	var req struct {
		ClassType       *models.AttendanceClassType `json:"class_type"`
		Date            *string                     `json:"date"`
		SessionID       *string                     `json:"session_id"`
		IsVerified      *bool                       `json:"is_verified"`
		ClassHighlights *string                     `json:"class_highlights"`
		Homework        *string                     `json:"homework"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}

	updates := map[string]interface{}{}

	if req.ClassType != nil {
		if !classTypeValid(*req.ClassType) {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid class_type", nil, nil)
			return
		}
		updates["class_type"] = *req.ClassType
	}
	if req.Date != nil {
		t, err := parseDateFlexible(*req.Date)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid date", nil, err.Error())
			return
		}
		updates["date"] = t
	}
	if req.SessionID != nil {
		updates["session_id"] = *req.SessionID
	}
	if req.ClassHighlights != nil {
		updates["class_highlights"] = *req.ClassHighlights
	}
	if req.Homework != nil {
		updates["homework"] = *req.Homework
	}
	if req.IsVerified != nil {
		// only mentor/admin can verify
		if current.Role == models.RoleCoach {
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
			return
		}
		updates["is_verified"] = *req.IsVerified
	}

	if len(updates) == 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "no updates provided", nil, nil)
		return
	}

	updated, err := h.store.UpdateAttendanceByID(ctx, uint(idU64), updates)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "updated", updated, nil)
}

// DELETE /attendances/{id}
func (h *AttendanceHandler) DeleteAttendance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	idU64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid id", nil, err.Error())
		return
	}

	existing, err := h.store.GetAttendanceByID(ctx, uint(idU64))
	if err != nil {
		if store.IsNotFound(err) {
			utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
			return
		}
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
		return
	}
	if !h.canAccessAttendance(ctx, current, existing) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	if err := h.store.DeleteAttendanceByID(ctx, uint(idU64)); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "delete failed", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "deleted", nil, nil)
}

func (h *AttendanceHandler) canAccessAttendance(ctx context.Context, current *models.User, a *models.Attendance) bool {
	switch current.Role {
	case models.RoleAdmin:
		return true
	case models.RoleCoach:
		return a.CoachID == current.ID
	case models.RoleMentor:
		if a.CoachID == current.ID {
			return true
		}
		ok, _ := h.store.IsMentorOfCoach(ctx, current.ID, a.CoachID)
		if ok {
			return true
		}
		ok2, _ := h.store.IsMentorOf(ctx, current.ID, a.StudentID)
		return ok2
	default:
		return false
	}
}
