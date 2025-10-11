package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
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
	u, err := h.store.GetUserByID(r.Context(), id)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "not found", nil, nil)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", u, nil)
}

// PUT /users/{id} - only allowed to update profile fields (not role, id, approval)
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
		City      string `json:"city,omitempty"`
		Phone     string `json:"phone,omitempty"`
		Bio       string `json:"bio,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "bad request", nil, nil)
		return
	}

	// Uncomment and implement the update logic as needed
	// if err := h.store.UpdateUserProfile(r.Context(), id, payload.FirstName, payload.LastName); err != nil {
	// 	utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update failed", nil, err)
	// 	return
	// }
	// if err := h.store.UpdateUserDetails(r.Context(), id, payload.City, payload.Phone, payload.Bio); err != nil {
	// 	utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "update details failed", nil, err)
	// 	return
	// }
	utils.WriteJSONResponse(w, http.StatusOK, true, "updated", nil, nil)
}

// GET /users - list users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsersAdmin(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error", nil, err)
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "success", users, nil)
}
