package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type UserHandler struct {
	store serviceStore
}

func NewUserHandler(store serviceStore) *UserHandler {
	return &UserHandler{
		store: store,
	}
}

// GET /users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing id", nil, nil)
		return
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// Coaches/mentors of the *requested* user (id); used to allow coach/mentor to view their student.
	coachId, mentorId, _ := h.store.GetCoachesByStudentID(ctx, id)

	if !CanAccessStudentData(ctx, h.store.Store, current, id) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	u, err := h.store.GetUserByID(ctx, id)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
		return
	}

	var coachInfo *models.PersonInfo
	var mentorInfo *models.PersonInfo

	mentorInfo, err = h.getPersonInfoByID(ctx, mentorId)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch mentor", nil, nil)
		return
	}

	coachInfo, err = h.getPersonInfoByID(ctx, coachId)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch coach", nil, nil)
		return
	}

	resp := models.UserResponse{
		User:   u,
		Coach:  coachInfo,
		Mentor: mentorInfo,
	}

	// Embed schedule for student profiles
	if u.Role == models.RoleStudent {
		schedule, err := h.store.ListSchedulesForStudents(ctx, []string{u.ID})
		if err == nil {
			resp.Schedule = schedule
		}
	}

	fmt.Println("Sending response: ", resp)
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", resp, nil)
}

// GET /users/self - get the profile of the requesting user
func (h *UserHandler) GetSelfProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	u, err := h.store.GetUserByID(ctx, current.ID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
		return
	}

	var coachInfo *models.PersonInfo
	var mentorInfo *models.PersonInfo

	// fetch assigned coach & mentor
	coachId, mentorId, _ := h.store.GetCoachesByStudentID(ctx, current.ID)

	mentorInfo, err = h.getPersonInfoByID(ctx, mentorId)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch mentor", nil, nil)
		return
	}

	coachInfo, err = h.getPersonInfoByID(ctx, coachId)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch coach", nil, nil)
		return
	}

	resp := models.UserResponse{
		User:   u,
		Coach:  coachInfo,
		Mentor: mentorInfo,
	}

	// Embed schedule for student profiles
	if u.Role == models.RoleStudent {
		schedule, err := h.store.ListSchedulesForStudents(ctx, []string{u.ID})
		if err == nil {
			resp.Schedule = schedule
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", resp, nil)
}

// PUT /users/{id} - only allowed to update profile fields (not role, id, approval)
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	print("UpdateUser called")
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing id", nil, nil)
		return
	}

	// payload with pointers to detect omitted fields
	var payload struct {
		FirstName         *string                 `json:"first_name,omitempty"`
		LastName          *string                 `json:"last_name,omitempty"`
		Password          *string                 `json:"password,omitempty"`
		City              *string                 `json:"city,omitempty"`
		State             *string                 `json:"state,omitempty"`
		Country           *string                 `json:"country,omitempty"`
		Zipcode           *string                 `json:"zipcode,omitempty"`
		Phone             *string                 `json:"phone,omitempty"`
		DOB               *string                 `json:"dob,omitempty"` // ISO date string, optional
		Age               *int                    `json:"age,omitempty"` // age in years, alternative to DOB
		LichessUsername   *string                 `json:"lichess_username,omitempty"`
		USCFID            *string                 `json:"uscf_id,omitempty"`
		ChesscomUsername  *string                 `json:"chesscom_username,omitempty"`
		FIDEID            *string                 `json:"fide_id,omitempty"`
		Bio               *string                 `json:"bio,omitempty"`
		ProfilePictureURL *string                 `json:"profile_picture_url,omitempty"`
		AdditionalInfo    *map[string]interface{} `json:"additional_info,omitempty"`
		SyllabusURL       *string                 `json:"syllabus_url,omitempty"`
		PersonalMeetLink  *string                 `json:"personal_meet_link,omitempty"`
		AddedInWhatsapp   *bool                   `json:"added_in_whatsapp,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "bad request", nil, err.Error())
		return
	}
	if payload.AddedInWhatsapp != nil {
		fmt.Println("whatsapp in payload:", *payload.AddedInWhatsapp)
	} else {
		fmt.Println("whatsapp in payload: nil")
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// Coaches/mentors of the *requested* user (id); used to allow coach/mentor to update their student.
	if !CanAccessStudentData(ctx, h.store.Store, current, id) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// ensure target exists
	existingUser, err := h.store.GetUserByID(ctx, id)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "user not found", nil, err.Error())
		return
	}

	userUpdates := map[string]interface{}{}
	if payload.FirstName != nil {
		userUpdates["first_name"] = *payload.FirstName
	}
	if payload.LastName != nil {
		userUpdates["last_name"] = *payload.LastName
	}

	// apply user updates if any
	if len(userUpdates) > 0 {
		if err := h.store.UpdateUserFields(ctx, id, userUpdates); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update failed", nil, err.Error())
			return
		}
	}

	detailUpdates := map[string]interface{}{}
	if payload.City != nil {
		detailUpdates["city"] = *payload.City
	}
	if payload.State != nil {
		detailUpdates["state"] = *payload.State
	}
	if payload.Country != nil {
		detailUpdates["country"] = *payload.Country
	}
	if payload.Zipcode != nil {
		detailUpdates["zipcode"] = *payload.Zipcode
	}
	if payload.Phone != nil {
		detailUpdates["phone"] = *payload.Phone
	}
	if payload.LichessUsername != nil {
		detailUpdates["lichess_username"] = *payload.LichessUsername
	}
	if payload.USCFID != nil {
		detailUpdates["uscf_id"] = *payload.USCFID
	}
	if payload.ChesscomUsername != nil {
		detailUpdates["chesscom_username"] = *payload.ChesscomUsername
	}
	if payload.FIDEID != nil {
		detailUpdates["fide_id"] = *payload.FIDEID
	}
	if payload.Bio != nil {
		detailUpdates["bio"] = *payload.Bio
	}
	if payload.ProfilePictureURL != nil {
		detailUpdates["profile_picture_url"] = *payload.ProfilePictureURL
	}
	if payload.AdditionalInfo != nil {
		detailUpdates["additional_info"] = *payload.AdditionalInfo
	}
	if payload.SyllabusURL != nil {
		detailUpdates["syllabus_url"] = *payload.SyllabusURL
	}
	if payload.PersonalMeetLink != nil {
		detailUpdates["personal_meet_link"] = *payload.PersonalMeetLink
	}
	if payload.AddedInWhatsapp != nil {
		detailUpdates["added_in_whatsapp"] = *payload.AddedInWhatsapp
	}
	// DOB and Age are mutually exclusive.
	// If DOB is provided, store it and remove age from additional_info.
	// If Age is provided (without DOB), store age in additional_info and clear DOB.
	if payload.DOB != nil && *payload.DOB != "" {
		if t, err := time.Parse(time.RFC3339, *payload.DOB); err == nil {
			detailUpdates["dob"] = t
		}
		// Store DOB and clear age and age_recorded_at from additional_info
		ai := copyAdditionalInfo(existingUser.UserDetails.AdditionalInfo)
		delete(ai, "age")
		delete(ai, "age_recorded_at")
		detailUpdates["additional_info"] = ai
	} else if payload.Age != nil {
		// Store age in additional_info and clear DOB
		ai := copyAdditionalInfo(existingUser.UserDetails.AdditionalInfo)
		ai["age"] = *payload.Age
		ai["age_recorded_at"] = time.Now().Format("2006-01-02")
		detailUpdates["additional_info"] = ai
		// Clear DOB
		detailUpdates["dob"] = nil
	}

	print("detailUpdates length: ", len(detailUpdates))
	if len(detailUpdates) > 0 {
		if err := h.store.UpdateUserDetailsFields(ctx, id, detailUpdates); err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update details failed", nil, err.Error())
			return
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "updated", nil, nil)
}

// GET /users - list users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	if current.Role == "admin" {
		users, err := h.store.ListUsersAdmin(ctx)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		utils.WriteJSONResponse(w, http.StatusOK, true, "success", users, nil)
		return
	}

	if current.Role == "student" {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// coach or mentor (or both) -> single DB query with OR
	users, err := h.store.ListStudentsForCoachOrMentor(ctx, current.ID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching students", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", users, nil)
}

// GET /users/students - returns id, full name, and email of all active students
func (h *UserHandler) GetStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}
	q := r.URL.Query()
	showInactive := q.Get("show-inactive")
	activeOnly := true
	if showInactive == "true" {
		activeOnly = false
	}

	if current.Role != models.RoleAdmin && current.Role != models.RoleCoach && current.Role != models.RoleMentor {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	users, err := h.store.ListActiveStudents(ctx, activeOnly)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching students", nil, err.Error())
		return
	}

	type studentSummary struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	result := make([]studentSummary, len(users))
	for i, u := range users {
		result[i] = studentSummary{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", result, nil)
}

// POST /users/reset-password - allows users to reset their own password
func (h *UserHandler) ResetOwnPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	var payload struct {
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err)
		return
	}

	if payload.NewPassword == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "new_password is required", nil, nil)
		return
	}

	if len(payload.NewPassword) < 6 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "password must be at least 6 characters long", nil, nil)
		return
	}

	// Hash the new password
	hash, err := utils.HashPassword(payload.NewPassword)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error hashing password", nil, err)
		return
	}

	// Update user's password (only their own)
	err = h.store.UpdateUserFields(ctx, current.ID, map[string]interface{}{
		"password_hash": hash,
	})
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error updating password", nil, err)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "password reset successfully", nil, nil)
}

// GET /users/{id}/tournaments - get upcoming tournaments near the user
func (h *UserHandler) GetTournaments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing id", nil, nil)
		return
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	if !CanAccessStudentData(ctx, h.store.Store, current, id) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	tournaments, err := h.store.GetTournamentsByUserID(ctx, id)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching tournaments", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", tournaments, nil)
}

func CanAccessStudentData(ctx context.Context, s *store.Store, current *models.User, targetID string) bool {
	if current.ID == targetID || current.Role == "admin" {
		return true
	}
	coachID, mentorID, _ := s.GetCoachesByStudentID(ctx, targetID)
	return current.ID == coachID || current.ID == mentorID
}

func (h *UserHandler) getPersonInfoByID(
	ctx context.Context,
	id string,
) (*models.PersonInfo, error) {

	if id == "" {
		return nil, nil
	}

	user, err := h.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, nil
	}

	return &models.PersonInfo{
		Name:              user.FirstName + " " + user.LastName,
		ProfilePictureURL: user.UserDetails.ProfilePictureURL,
		FIDEID:            user.UserDetails.FIDEID,
		Bio:               user.UserDetails.Bio,
		PersonalMeetLink:  user.UserDetails.PersonalMeetLink,
	}, nil
}

// GET /users/coaches - returns coaches/mentors for the attendance coach-overview dropdown.
// Admin: all coach + mentor users. Mentor: coaches under them + themselves.
func (h *UserHandler) GetCoachesForAttendance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	switch current.Role {
	case models.RoleAdmin:
		coaches, err := h.store.GetAllCoaches(ctx)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		mentors, err := h.store.GetAllMentorCoaches(ctx)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		all := append(coaches, mentors...)
		utils.WriteJSONResponse(w, http.StatusOK, true, "success", all, nil)

	case models.RoleMentor:
		users, err := h.store.ListCoachesForMentor(ctx, current.ID)
		if err != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err.Error())
			return
		}
		utils.WriteJSONResponse(w, http.StatusOK, true, "success", users, nil)

	default:
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
	}
}

// copyAdditionalInfo returns a shallow copy of the additional_info map
// so that modifications don't mutate the original GORM-loaded data.
func copyAdditionalInfo(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
