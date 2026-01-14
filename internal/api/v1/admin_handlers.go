package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type AdminHandler struct {
	store serviceStore
}

func NewAdminHandler(store serviceStore) *AdminHandler {
	return &AdminHandler{store: store}
}

func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    *string `json:"email,omitempty"`
		Role     *string `json:"role,omitempty"`
		Approved *bool   `json:"approved,omitempty"`
		Active   *bool   `json:"active,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request ", nil, err)
		return
	}
	userUpdates := map[string]interface{}{}
	if payload.Email != nil {
		userUpdates["email"] = *payload.Email
	}
	if payload.Role != nil {
		userUpdates["role"] = *payload.Role
	}
	if payload.Approved != nil {
		userUpdates["approved"] = *payload.Approved
	}
	if payload.Active != nil {
		userUpdates["active"] = *payload.Active
	}
	// Update user status logic here
	err := h.store.UpdateUserFields(r.Context(), chi.URLParam(r, "id"), userUpdates)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "couldnt process the updates ", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "updated", nil, nil)
}

// GetUnapprovedUsers returns list of unapproved users
func (h *AdminHandler) GetUnapprovedUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUnapprovedUsers(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching unapproved users", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", users, nil)
}

// GetStudentsWithAssignments returns all students with their assignment info
func (h *AdminHandler) GetStudentsWithAssignments(w http.ResponseWriter, r *http.Request) {
	students, err := h.store.ListStudentsWithAssignments(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching students", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", students, nil)
}

// GetCoachesWithAssignments returns all coaches with their assignment info
func (h *AdminHandler) GetCoachesWithAssignments(w http.ResponseWriter, r *http.Request) {
	coaches, err := h.store.ListCoachesWithAssignments(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching coaches", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", coaches, nil)
}

// ApproveUser approves a user
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing user id", nil, nil)
		return
	}

	err := h.store.ApproveUser(r.Context(), userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error approving user", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "user approved", nil, nil)
}

// AssignStudentToCoach assigns a student to a coach
func (h *AdminHandler) AssignStudentToCoach(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CoachID   string `json:"coach_id"`
		StudentID string `json:"student_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err)
		return
	}

	if payload.CoachID == "" || payload.StudentID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "coach_id and student_id are required", nil, nil)
		return
	}

	// Check if assignment already exists
	var existing models.CoachStudent
	if err := h.store.Store.DB.WithContext(r.Context()).Where("coach_id = ? AND student_id = ?", payload.CoachID, payload.StudentID).First(&existing).Error; err == nil {
		utils.WriteJSONResponse(w, http.StatusConflict, false, "assignment already exists", nil, nil)
		return
	}

	err := h.store.AddCoachStudent(r.Context(), payload.CoachID, payload.StudentID, "")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error assigning student", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "student assigned to coach", nil, nil)
}

// AssignCoachAsMentor assigns a coach as a mentor to a student
func (h *AdminHandler) AssignCoachAsMentor(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		MentorCoachID string `json:"mentor_coach_id"`
		StudentID     string `json:"student_id"`
		CoachID       string `json:"coach_id"` // Optional: The existing coach for this student
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err)
		return
	}

	if payload.MentorCoachID == "" || payload.StudentID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "mentor_coach_id and student_id are required", nil, nil)
		return
	}

	// Find existing coach-student relationship for this student
	var existing models.CoachStudent
	err := h.store.Store.DB.WithContext(r.Context()).
		Where("student_id = ?", payload.StudentID).
		First(&existing).Error

	if err == nil {
		// Update existing relationship to add/update mentor
		// Use payload.CoachID if provided, otherwise use existing.CoachID
		coachIDToUse := payload.CoachID
		if coachIDToUse == "" {
			coachIDToUse = existing.CoachID
		}
		updateData := map[string]interface{}{"mentor_coach_id": payload.MentorCoachID}
		err = h.store.Store.DB.WithContext(r.Context()).Model(&models.CoachStudent{}).
			Where("coach_id = ? AND student_id = ?", coachIDToUse, payload.StudentID).
			Updates(updateData).Error
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error assigning mentor", nil, err)
			return
		}
	} else {
		// No existing relationship - if coach_id is provided, create with that coach and mentor
		// Otherwise, create new one with mentor as coach (since coach_id is required)
		if payload.CoachID != "" {
			err = h.store.AddCoachStudent(r.Context(), payload.CoachID, payload.StudentID, payload.MentorCoachID)
		} else {
			// In this case, the mentor coach will also be the coach
			err = h.store.AddCoachStudent(r.Context(), payload.MentorCoachID, payload.StudentID, payload.MentorCoachID)
		}
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error assigning mentor", nil, err)
			return
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "coach assigned as mentor", nil, nil)
}
