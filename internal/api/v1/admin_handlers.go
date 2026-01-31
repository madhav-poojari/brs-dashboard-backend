package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
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
		Password *string `json:"password,omitempty"` // plaintext; will be hashed server-side
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
	if payload.Password != nil {
		if *payload.Password == "" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "password cannot be empty", nil, nil)
			return
		}
		hash, err := utils.HashPassword(*payload.Password)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error hashing password", nil, err)
			return
		}
		userUpdates["password_hash"] = hash
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

// UpdateAssignments is the single endpoint for assignment updates.
//
// Supported payloads:
// - assignment_type=student_coach: {student_id, coach_id} (coach_id can be empty to remove)
// - assignment_type=coach_mentor:  {coach_id, mentor_coach_id} (mentor_coach_id can be empty to remove)
func (h *AdminHandler) UpdateAssignments(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AssignmentType string `json:"assignment_type"`
		StudentID      string `json:"student_id,omitempty"`
		CoachID        string `json:"coach_id,omitempty"`
		MentorCoachID  string `json:"mentor_coach_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err)
		return
	}

	switch payload.AssignmentType {
	case "student_coach":
		if payload.StudentID == "" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "student_id is required", nil, nil)
			return
		}
		if err := h.store.SetStudentCoachAssignment(r.Context(), payload.StudentID, payload.CoachID); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error updating student assignment", nil, err)
			return
		}
		utils.WriteJSONResponse(w, http.StatusOK, true, "student assignment updated", nil, nil)
		return

	case "coach_mentor":
		if payload.CoachID == "" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "coach_id is required", nil, nil)
			return
		}
		if err := h.store.SetCoachMentorAssignment(r.Context(), payload.CoachID, payload.MentorCoachID); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error updating mentor assignment", nil, err)
			return
		}
		utils.WriteJSONResponse(w, http.StatusOK, true, "mentor assignment updated", nil, nil)
		return

	default:
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid assignment_type", nil, nil)
		return
	}
}
