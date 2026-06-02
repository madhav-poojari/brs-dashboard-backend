package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type ReferralHandler struct {
	referralService *service.ReferralService
	store           *store.Store
}

func NewReferralHandler(ss serviceStore) *ReferralHandler {
	return &ReferralHandler{
		referralService: service.NewReferralService(ss.Store),
		store:           ss.Store,
	}
}

// GetGraph returns the complete referral network graph
// GET /referral-network/graph
func (rh *ReferralHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	stateFilter := r.URL.Query().Get("state")

	data, err := rh.referralService.GetNetworkGraph(r.Context(), stateFilter)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "Failed to fetch network graph", nil, err)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "Network graph fetched successfully", data, nil)
}

// GetNodeDetail returns detailed information about a specific node
// GET /referral-network/node/{user_id}
func (rh *ReferralHandler) GetNodeDetail(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "User ID is required", nil, nil)
		return
	}

	data, err := rh.referralService.GetNodeDetail(r.Context(), userID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "User not found", nil, err)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "Node detail fetched successfully", data, nil)
}

// CreateRelationship creates a new referral relationship
// POST /referral-network/relationship
func (rh *ReferralHandler) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid request body", nil, err)
		return
	}

	// Validate required fields
	if req.ReferrerID == "" || req.RefereeID == "" || req.RelationshipType == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "referrer_id, referee_id, and relationship_type are required", nil, nil)
		return
	}

	data, err := rh.referralService.CreateRelationship(r.Context(), req.ReferrerID, req.RefereeID, req.RelationshipType, req.RelationshipDescription)
	if err != nil {
		// Check for specific error types
		if err.Error() == "cannot create self-referral" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Cannot create self-referral", nil, err)
		} else if err.Error() == "invalid relationship type" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid relationship type", nil, err)
		} else if err.Error() == "relationship already exists" {
			utils.WriteJSONResponse(w, http.StatusConflict, false, "Relationship already exists", nil, err)
		} else {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "Failed to create relationship", nil, err)
		}
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, true, "Relationship created successfully", data, nil)
}

// UpdateRelationship updates a relationship
// PUT /referral-network/relationship/{relationship_id}
func (rh *ReferralHandler) UpdateRelationship(w http.ResponseWriter, r *http.Request) {
	relationshipID := chi.URLParam(r, "relationship_id")
	if relationshipID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Relationship ID is required", nil, nil)
		return
	}

	var req models.UpdateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid request body", nil, err)
		return
	}

	if err := rh.referralService.UpdateRelationship(r.Context(), relationshipID, req.RelationshipType, req.RelationshipDescription); err != nil {
		if err.Error() == "invalid relationship type" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid relationship type", nil, err)
		} else if err.Error() == "no fields to update" {
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "No fields to update", nil, err)
		} else {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "Failed to update relationship", nil, err)
		}
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "Relationship updated successfully", nil, nil)
}

// DeleteRelationship deletes a relationship
// DELETE /referral-network/relationship/{relationship_id}
func (rh *ReferralHandler) DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	relationshipID := chi.URLParam(r, "relationship_id")
	if relationshipID == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Relationship ID is required", nil, nil)
		return
	}

	if err := rh.referralService.DeleteRelationship(r.Context(), relationshipID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "Failed to delete relationship", nil, err)
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "Relationship deleted successfully", nil, nil)
}
