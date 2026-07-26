package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

// NotificationConfigHandler handles admin CRUD for notification_configs.
type NotificationConfigHandler struct {
	store *store.Store
}

// NewNotificationConfigHandler creates a new NotificationConfigHandler.
func NewNotificationConfigHandler(s serviceStore) *NotificationConfigHandler {
	return &NotificationConfigHandler{store: s.Store}
}

// ListConfigs returns all notification config entries.
// GET /admin/notification-configs
func (h *NotificationConfigHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListNotificationConfigs(r.Context())
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching configs", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", configs, nil)
}

// CreateConfig creates a new notification config entry.
// POST /admin/notification-configs
func (h *NotificationConfigHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string `json:"type"`
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err.Error())
		return
	}
	if req.Type == "" || req.Key == "" || req.Value == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "type, key, and value are required", nil, nil)
		return
	}

	config := &models.NotificationConfig{
		Type:        req.Type,
		Key:         req.Key,
		Value:       req.Value,
		Description: req.Description,
	}

	if err := h.store.CreateNotificationConfig(r.Context(), config); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error creating config", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, true, "config created", config, nil)
}

// UpdateConfig updates a notification config entry by ID.
// PUT /admin/notification-configs/{id}
func (h *NotificationConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid config id", nil, nil)
		return
	}

	var req struct {
		Value       *string `json:"value"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err.Error())
		return
	}

	fields := map[string]interface{}{}
	if req.Value != nil {
		fields["value"] = *req.Value
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if len(fields) == 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "no fields to update", nil, nil)
		return
	}

	if err := h.store.UpdateNotificationConfig(r.Context(), uint(id), fields); err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "config not found", nil, nil)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "config updated", nil, nil)
}

// DeleteConfig deletes a notification config entry by ID.
// DELETE /admin/notification-configs/{id}
func (h *NotificationConfigHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid config id", nil, nil)
		return
	}

	if err := h.store.DeleteNotificationConfig(r.Context(), uint(id)); err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "config not found", nil, nil)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "config deleted", nil, nil)
}
