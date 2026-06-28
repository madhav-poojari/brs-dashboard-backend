package v1

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type PayoutHandler struct {
	store        *store.Store
	imageStorage *utils.R2Storage
	cfg          *config.Config
}

func NewPayoutHandler(s serviceStore, cfg *config.Config) *PayoutHandler {
	return &PayoutHandler{
		store:        s.Store,
		imageStorage: utils.NewR2Storage(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Endpoint, cfg.R2BucketName),
		cfg:          cfg,
	}
}

// GET /payouts/pending — List all pending transactions (admin only)
func (h *PayoutHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	txs, err := h.store.ListPendingTransactions(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch pending transactions", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", txs, nil)
}

// POST /payouts/approve/{id} — Approve a pending transaction (admin only)
// Accepts optional JSON body: { "units": 5.0, "reason": "updated reason" }
func (h *PayoutHandler) ApproveTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid id", nil, err.Error())
		return
	}

	// Parse optional overrides from body
	var body struct {
		Units  *float64 `json:"units"`
		Reason string   `json:"reason"`
	}
	// Ignore decode errors — body is optional
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := h.store.ApproveTransaction(ctx, uint(id), current.ID, body.Units, body.Reason); err != nil {
		if store.IsNotFound(err) {
			utils.WriteJSONResponse(w, http.StatusNotFound, false, "transaction not found", nil, nil)
			return
		}
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to approve transaction", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "transaction approved", nil, nil)
}

// POST /payouts/reject/{id} — Reject a pending transaction (admin only)
func (h *PayoutHandler) RejectTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid id", nil, err.Error())
		return
	}

	if err := h.store.RejectTransaction(ctx, uint(id), current.ID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to reject transaction", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "transaction rejected", nil, nil)
}

// GET /payouts/balances — List all students with their current balance (admin only)
func (h *PayoutHandler) ListBalances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	balances, err := h.store.GetAllStudentBalances(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch balances", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", balances, nil)
}

// POST /payouts/adjust — Admin direct add/deduct units for any student (admin only)
func (h *PayoutHandler) AdminAdjust(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	var req struct {
		UserID string  `json:"user_id"`
		Units  float64 `json:"units"`
		Reason string  `json:"reason"`
		Type   string  `json:"type"` // referral_bonus, admin_credit, admin_debit
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request", nil, err.Error())
		return
	}

	if req.UserID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "user_id is required", nil, nil)
		return
	}
	if req.Units == 0 {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "units must be non-zero", nil, nil)
		return
	}

	// Determine transaction type
	txType := models.UnitTransactionType(req.Type)
	switch txType {
	case models.UnitTxTypeReferralBonus, models.UnitTxTypeAdminCredit, models.UnitTxTypeAdminDebit:
		// valid
	default:
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid type, must be referral_bonus, admin_credit, or admin_debit", nil, nil)
		return
	}

	// Enforce max adjustment units cap (prevents accidental large entries)
	if math.Abs(req.Units) > service.MaxAdjustmentUnits {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false,
			fmt.Sprintf("units cannot exceed %.1f for bonus/adjustment", service.MaxAdjustmentUnits), nil, nil)
		return
	}

	// For admin_debit, units should be negative
	units := req.Units
	if txType == models.UnitTxTypeAdminDebit && units > 0 {
		units = -units
	}
	// For credit types, units should be positive
	if (txType == models.UnitTxTypeAdminCredit || txType == models.UnitTxTypeReferralBonus) && units < 0 {
		units = -units
	}

	tx, err := h.store.AdminDirectAdjustment(ctx, req.UserID, units, req.Reason, txType, current.ID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to adjust units", nil, err.Error())
		return
	}
	utils.WriteJSONResponse(w, http.StatusOK, true, "units adjusted", tx, nil)
}

// GET /payouts/timeline/{userId} — Get transaction timeline for a student (admin only)
func (h *PayoutHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	userID := chi.URLParam(r, "userId")
	if userID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "userId is required", nil, nil)
		return
	}

	// Fetch balance
	bal, err := h.store.GetOrCreateUnitBalance(ctx, userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch balance", nil, err.Error())
		return
	}

	// Fetch timeline
	txs, err := h.store.GetStudentTimeline(ctx, userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to fetch timeline", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "ok", map[string]interface{}{
		"balance":      bal.Balance,
		"transactions": txs,
	}, nil)
}

// POST /payouts/trigger-deduction — Manually trigger monthly deduction (admin only, for testing)
func (h *PayoutHandler) TriggerDeduction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	go service.RunMonthlyPayoutDeduction(h.store)

	utils.WriteJSONResponse(w, http.StatusOK, true, "deduction triggered in background", nil, nil)
}

// POST /payouts/payment-request — Student submits a payment request (student + admin)
// Accepts multipart form: screenshot file + transaction_id field
func (h *PayoutHandler) SubmitPaymentRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	// Parse multipart form (max 5MB)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "file too large or invalid form", nil, err.Error())
		return
	}

	transactionID := r.FormValue("transaction_id")

	// Screenshot is optional on backend (required on frontend)
	var screenshotSuffix string
	file, header, err := r.FormFile("screenshot")
	if err == nil {
		defer file.Close()
		subDir := fmt.Sprintf("payment-screenshots/%s", current.ID)
		suffix, uploadErr := h.imageStorage.SaveFile(subDir, header.Filename, file)
		if uploadErr != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to upload screenshot", nil, uploadErr.Error())
			return
		}
		screenshotSuffix = suffix
	}
	// If no file provided, screenshotSuffix stays empty — that's fine per BE requirements

	tx := &models.UnitTransaction{
		UserID:        current.ID,
		Type:          models.UnitTxTypePayment,
		Units:         0, // admin will set the actual units when approving; or we can accept from form
		Reason:        "Payment submitted",
		ScreenshotURL: screenshotSuffix,
		TransactionID: transactionID,
		Status:        models.UnitTxStatusPending,
		CreatedBy:     current.ID,
	}

	// Optionally accept units from form (admin may want to pre-fill)
	unitsStr := r.FormValue("units")
	if unitsStr != "" {
		if u, err := strconv.ParseFloat(unitsStr, 64); err == nil && u > 0 {
			tx.Units = u
		}
	}

	// Optionally accept reason from form
	reason := r.FormValue("reason")
	if reason != "" {
		tx.Reason = reason
	}

	if err := h.store.CreateUnitTransaction(ctx, tx); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to create payment request", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, true, "payment request submitted", map[string]interface{}{
		"id":             tx.ID,
		"screenshot_url": screenshotSuffix,
	}, nil)
}
