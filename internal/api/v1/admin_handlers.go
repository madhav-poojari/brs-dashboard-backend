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

// GetPendingApprovals returns users where approved = false, sorted by created_at DESC
func (h *AdminHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.GetPendingApprovals(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch pending approvals", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "pending approvals fetched", users, nil)
}

// ApproveUser sets approved = true for the user
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "user id is required", nil, nil)
		return
	}

	err := h.store.ApproveUser(r.Context(), userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to approve user", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "user approved successfully", nil, nil)
}

// GetStudentsWithAssignments returns all students with their coach assignment info
func (h *AdminHandler) GetStudentsWithAssignments(w http.ResponseWriter, r *http.Request) {
	students, err := h.store.GetStudentsWithAssignments(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch students", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "students fetched", students, nil)
}

// GetCoachesWithAssignments returns all coaches with their mentor assignment info
func (h *AdminHandler) GetCoachesWithAssignments(w http.ResponseWriter, r *http.Request) {
	coaches, err := h.store.GetCoachesWithAssignments(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch coaches", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "coaches fetched", coaches, nil)
}

// GetMentorCoaches returns all mentor coaches
func (h *AdminHandler) GetMentorCoaches(w http.ResponseWriter, r *http.Request) {
	mentors, err := h.store.GetAllMentorCoaches(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch mentor coaches", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "mentor coaches fetched", mentors, nil)
}

// GetAllCoaches returns all coaches (for assignment dropdown)
func (h *AdminHandler) GetAllCoaches(w http.ResponseWriter, r *http.Request) {
	coaches, err := h.store.GetAllCoaches(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch coaches", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "coaches fetched", coaches, nil)
}

// AssignStudentToCoach assigns a student to a coach and optionally a mentor coach
func (h *AdminHandler) AssignStudentToCoach(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		StudentID     string `json:"student_id"`
		CoachID       string `json:"coach_id"`
		MentorCoachID string `json:"mentor_coach_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err)
		return
	}

	if payload.StudentID == "" || payload.CoachID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "student_id and coach_id are required", nil, nil)
		return
	}

	err := h.store.AssignStudentToCoach(r.Context(), payload.StudentID, payload.CoachID, payload.MentorCoachID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to assign student to coach", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "student assigned to coach successfully", nil, nil)
}

// AssignCoachToMentor assigns a coach to a mentor coach
func (h *AdminHandler) AssignCoachToMentor(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CoachID       string `json:"coach_id"`
		MentorCoachID string `json:"mentor_coach_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err)
		return
	}

	if payload.CoachID == "" || payload.MentorCoachID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "coach_id and mentor_coach_id are required", nil, nil)
		return
	}

	err := h.store.AssignCoachToMentor(r.Context(), payload.CoachID, payload.MentorCoachID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to assign coach to mentor", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "coach assigned to mentor successfully", nil, nil)
}

// ListAllUsers returns all users grouped by role
func (h *AdminHandler) ListAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.GetAllUsersGrouped(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch users", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "users fetched", users, nil)
}

// AdminDashboardData represents the data structure for the admin dashboard
type AdminDashboardData struct {
	PendingApprovals interface{} `json:"pending_approvals,omitempty"`
	Students         interface{} `json:"students"`
	Coaches          interface{} `json:"coaches"`
	MentorCoaches    interface{} `json:"mentor_coaches"`
}

// GetAdminDashboard returns all data needed for the admin dashboard in one call
func (h *AdminHandler) GetAdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pending approvals
	pendingUsers, err := h.store.GetPendingApprovals(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch pending approvals", nil, err)
		return
	}

	// Get students with assignments
	students, err := h.store.GetStudentsWithAssignments(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch students", nil, err)
		return
	}

	// Get coaches with assignments
	coaches, err := h.store.GetCoachesWithAssignments(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch coaches", nil, err)
		return
	}

	// Get mentor coaches
	mentors, err := h.store.GetAllMentorCoaches(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch mentor coaches", nil, err)
		return
	}

	data := map[string]interface{}{
		"pending_approvals": pendingUsers,
		"students":          students,
		"coaches":           coaches,
		"mentor_coaches":    mentors,
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "admin dashboard data fetched", data, nil)
}
