package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type UserHandler struct {
	store serviceStore
}

func NewUserHandler(store serviceStore) *UserHandler {
	return &UserHandler{store: store}
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
	if !CanAccessStudentData(current, id, coachId, mentorId) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	u, err := h.store.GetUserByID(r.Context(), id)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", u, nil)
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
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", u, nil)
}

// PUT /users/{id} - only allowed to update profile fields (not role, id, approval)
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
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
		LichessUsername   *string                 `json:"lichess_username,omitempty"`
		USCFID            *string                 `json:"uscf_id,omitempty"`
		ChesscomUsername  *string                 `json:"chesscom_username,omitempty"`
		FIDEID            *string                 `json:"fide_id,omitempty"`
		Bio               *string                 `json:"bio,omitempty"`
		ProfilePictureURL *string                 `json:"profile_picture_url,omitempty"`
		AdditionalInfo    *map[string]interface{} `json:"additional_info,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "bad request", nil, err.Error())
		return
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// Coaches/mentors of the *requested* user (id); used to allow coach/mentor to update their student.
	coachId, mentorId, _ := h.store.GetCoachesByStudentID(ctx, id)
	if !CanAccessStudentData(current, id, coachId, mentorId) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// ensure target exists
	_, err := h.store.GetUserByID(ctx, id)
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
	if payload.DOB != nil {
		if t, err := time.Parse(time.RFC3339, *payload.DOB); err == nil {
			detailUpdates["dob"] = t
		}
	}

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

func CanAccessStudentData(current *models.User, targetID string, coachId string, mentorId string) bool {
	if current.ID == targetID || current.Role == "admin" || current.ID == coachId || current.ID == mentorId {
		return true
	}
	return false
}
