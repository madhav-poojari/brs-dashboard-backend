package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
)

type ReferralService struct {
	store *store.Store
}

func NewReferralService(s *store.Store) *ReferralService {
	return &ReferralService{store: s}
}

// ValidateRelationshipType checks if relationship type is valid
func (rs *ReferralService) ValidateRelationshipType(relType string) error {
	validTypes := map[string]bool{
		"vendor":            true,
		"classmate_college": true,
		"classmate_school":  true,
		"coworker":          true,
		"family":            true,
		"friend":            true,
		"coach":             true,
		"student":           true,
		"other":             true,
	}
	if !validTypes[relType] {
		return errors.New("invalid relationship type")
	}
	return nil
}

// CreateRelationship creates a new referral relationship with validation
func (rs *ReferralService) CreateRelationship(ctx context.Context, referrerID, refereeID, relationshipType string, description *string) (map[string]interface{}, error) {
	// Prevent self-referral
	if referrerID == refereeID {
		return nil, errors.New("cannot create self-referral")
	}

	// Validate relationship type
	if err := rs.ValidateRelationshipType(relationshipType); err != nil {
		return nil, err
	}

	// Create the relationship
	rel := &models.ReferralRelationship{
		ReferrerID:              referrerID,
		RefereeID:               refereeID,
		RelationshipType:        relationshipType,
		RelationshipDescription: description,
	}

	if err := rs.store.CreateReferralRelationship(ctx, rel); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":                       rel.ID,
		"referrer_id":              rel.ReferrerID,
		"referee_id":               rel.RefereeID,
		"relationship_type":        rel.RelationshipType,
		"relationship_description": rel.RelationshipDescription,
		"created_at":               rel.CreatedAt,
	}, nil
}

// UpdateRelationship updates a relationship's metadata
func (rs *ReferralService) UpdateRelationship(ctx context.Context, relationshipID string, relationshipType *string, description *string) error {
	// Validate relationship exists
	_, err := rs.store.GetReferralRelationship(ctx, relationshipID)
	if err != nil {
		fmt.Println("relationship not found")
		return err
	}

	updates := make(map[string]interface{})

	if relationshipType != nil {
		if err := rs.ValidateRelationshipType(*relationshipType); err != nil {
			return err
		}
		updates["relationship_type"] = *relationshipType
	}

	if description != nil {
		updates["relationship_description"] = *description
	}

	if len(updates) == 0 {
		return errors.New("no fields to update")
	}

	if err := rs.store.UpdateReferralRelationship(ctx, relationshipID, updates); err != nil {
		return err
	}

	// Refresh the relationship
	_, err = rs.store.GetReferralRelationship(ctx, relationshipID)
	return err
}

// DeleteRelationship removes a relationship
func (rs *ReferralService) DeleteRelationship(ctx context.Context, relationshipID string) error {
	// Validate relationship exists
	_, err := rs.store.GetReferralRelationship(ctx, relationshipID)
	if err != nil {
		return err
	}

	return rs.store.DeleteReferralRelationship(ctx, relationshipID)
}

// GetNetworkGraph fetches complete graph data for visualization
func (rs *ReferralService) GetNetworkGraph(ctx context.Context, stateFilter string) (map[string]interface{}, error) {
	return rs.store.GetFullReferralGraph(ctx, stateFilter)
}

// GetNodeDetail fetches all details about a specific node for hover/popover
func (rs *ReferralService) GetNodeDetail(ctx context.Context, userID string) (map[string]interface{}, error) {
	return rs.store.GetNodeDetail(ctx, userID)
}
