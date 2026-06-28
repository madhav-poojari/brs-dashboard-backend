package v1

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

// NotificationHandler handles notification-related REST endpoints.
type NotificationHandler struct {
	store *store.Store
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(s serviceStore) *NotificationHandler {
	return &NotificationHandler{store: s.Store}
}

// ListNotifications returns paginated notifications for the current user.
// GET /notifications?limit=20&offset=0
func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromCtx(ctx)
	if user == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	notifications, err := h.store.GetNotificationsByUserID(ctx, user.ID, limit, offset)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching notifications", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", notifications, nil)
}

// GetUnreadCount returns the number of unread notifications for the current user.
// GET /notifications/unread-count
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromCtx(ctx)
	if user == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	count, err := h.store.GetUnreadNotificationCount(ctx, user.ID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error fetching unread count", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", map[string]int64{"count": count}, nil)
}

// MarkAsRead marks a single notification as read.
// PATCH /notifications/{id}/read
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromCtx(ctx)
	if user == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	notificationID := chi.URLParam(r, "id")
	if notificationID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing notification id", nil, nil)
		return
	}

	if err := h.store.MarkNotificationAsRead(ctx, notificationID, user.ID); err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "notification not found", nil, nil)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "marked as read", nil, nil)
}

// MarkAllAsRead marks all unread notifications as read for the current user.
// PATCH /notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromCtx(ctx)
	if user == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	if err := h.store.MarkAllNotificationsAsRead(ctx, user.ID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error marking notifications as read", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "all notifications marked as read", nil, nil)
}

// TriggerNotificationsTest manually triggers all notification check tasks.
// POST /admin/trigger-notifications-test
func (h *NotificationHandler) TriggerNotificationsTest(w http.ResponseWriter, r *http.Request) {
	// Trigger notification checks synchronously so we can return the result immediately
	service.CheckJoiningAnniversaries(h.store)
	service.CheckTournamentParticipation(h.store)
	service.CheckRatingMilestones(h.store)

	utils.WriteJSONResponse(w, http.StatusOK, true, "notification checks triggered successfully", nil, nil)
}

