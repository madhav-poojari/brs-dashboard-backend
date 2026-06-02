package store

import (
	"context"
	"errors"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
)

// CreateReferralRelationship creates a new referral relationship
func (s *Store) CreateReferralRelationship(ctx context.Context, relationship *models.ReferralRelationship) error {
	var referrer models.User
	if err := s.DB.WithContext(ctx).Where("id = ? AND active = true", relationship.ReferrerID).First(&referrer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("referrer user not found or inactive")
		}
		return err
	}

	var referee models.User
	if err := s.DB.WithContext(ctx).Where("id = ? AND active = true", relationship.RefereeID).First(&referee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("referee user not found or inactive")
		}
		return err
	}

	var existing models.ReferralRelationship
	if err := s.DB.WithContext(ctx).Where("referrer_id = ? AND referee_id = ?", relationship.ReferrerID, relationship.RefereeID).First(&existing).Error; err == nil {
		return errors.New("relationship already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.DB.WithContext(ctx).Create(relationship).Error
}

// GetReferralRelationship gets a single relationship by ID
func (s *Store) GetReferralRelationship(ctx context.Context, relationshipID string) (*models.ReferralRelationship, error) {
	var rel models.ReferralRelationship
	if err := s.DB.WithContext(ctx).Where("id = ?", relationshipID).First(&rel).Error; err != nil {
		return nil, err
	}
	return &rel, nil
}

// GetReferralRelationshipByUsers gets relationship between two specific users
func (s *Store) GetReferralRelationshipByUsers(ctx context.Context, referrerID, refereeID string) (*models.ReferralRelationship, error) {
	var rel models.ReferralRelationship
	if err := s.DB.WithContext(ctx).Where("referrer_id = ? AND referee_id = ?", referrerID, refereeID).First(&rel).Error; err != nil {
		return nil, err
	}
	return &rel, nil
}

// UpdateReferralRelationship updates a relationship
func (s *Store) UpdateReferralRelationship(ctx context.Context, relationshipID string, updates map[string]interface{}) error {
	return s.DB.WithContext(ctx).Model(&models.ReferralRelationship{}).Where("id = ?", relationshipID).Updates(updates).Error
}

// DeleteReferralRelationship deletes a relationship
func (s *Store) DeleteReferralRelationship(ctx context.Context, relationshipID string) error {
	return s.DB.WithContext(ctx).Where("id = ?", relationshipID).Delete(&models.ReferralRelationship{}).Error
}

// GetFullReferralGraph fetches all nodes and edges
func (s *Store) GetFullReferralGraph(ctx context.Context, stateFilter string) (map[string]interface{}, error) {
	var users []struct {
		ID                string
		FirstName         string
		LastName          string
		Role              string
		ProfilePictureURL string
		State             string
		City              string
		Active            bool
	}

	query := s.DB.WithContext(ctx).
		Table("users").
		Select("users.id, users.first_name, users.last_name, users.role, user_details.profile_picture_url, user_details.state, user_details.city, users.active").
		Joins("LEFT JOIN user_details ON users.id = user_details.user_id").
		Where("users.active = true")

	if stateFilter != "" {
		query = query.Where("user_details.state = ?", stateFilter)
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}

	nodes := make([]map[string]interface{}, len(users))
	userMap := make(map[string]string)
	for i, user := range users {
		nodes[i] = map[string]interface{}{
			"id":                  user.ID,
			"name":                user.FirstName + " " + user.LastName,
			"state":               user.State,
			"city":                user.City,
			"role":                user.Role,
			"profile_picture_url": user.ProfilePictureURL,
		}
		userMap[user.ID] = user.State
	}

	var relationships []models.ReferralRelationship
	relQuery := s.DB.WithContext(ctx)
	if stateFilter != "" {
		relQuery = relQuery.Table("referral_relationships").
			Joins("JOIN users u1 ON referral_relationships.referrer_id = u1.id").
			Joins("JOIN user_details ud1 ON u1.id = ud1.user_id").
			Where("ud1.state = ?", stateFilter)
	}

	if err := relQuery.Find(&relationships).Error; err != nil {
		return nil, err
	}

	edges := make([]map[string]interface{}, len(relationships))
	for i, rel := range relationships {
		edges[i] = map[string]interface{}{
			"id":                       rel.ID,
			"source":                   rel.ReferrerID,
			"target":                   rel.RefereeID,
			"relationship_type":        rel.RelationshipType,
			"relationship_description": rel.RelationshipDescription,
			"created_at":               rel.CreatedAt,
		}
	}

	return map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
		"metadata": map[string]interface{}{
			"total_nodes": len(nodes),
			"total_edges": len(edges),
		},
	}, nil
}

// GetNodeDetail fetches detailed info for a node (for hover state)
func (s *Store) GetNodeDetail(ctx context.Context, userID string) (map[string]interface{}, error) {
	var user struct {
		ID                string
		FirstName         string
		LastName          string
		Email             string
		Role              string
		ProfilePictureURL string
		Bio               string
		PersonalMeetLink  string
		State             string
		City              string
		Country           string
		USCFID            string
		LichessUsername   string
		ChesscomUsername  string
	}

	if err := s.DB.WithContext(ctx).
		Table("users").
		Select("users.id, users.first_name, users.last_name, users.email, users.role, user_details.profile_picture_url, user_details.bio, user_details.personal_meet_link, user_details.state, user_details.city, user_details.country, user_details.uscf_id, user_details.lichess_username, user_details.chesscom_username").
		Joins("LEFT JOIN user_details ON users.id = user_details.user_id").
		Where("users.id = ? AND users.active = true", userID).
		First(&user).Error; err != nil {
		return nil, err
	}

	var referredBy []struct {
		UserID                  string
		Name                    string
		RelationshipType        string
		RelationshipDescription *string
	}

	if err := s.DB.WithContext(ctx).
		Table("referral_relationships").
		Select("referral_relationships.referrer_id as user_id, CONCAT(users.first_name, ' ', users.last_name) as name, referral_relationships.relationship_type, referral_relationships.relationship_description").
		Joins("JOIN users ON referral_relationships.referrer_id = users.id").
		Where("referral_relationships.referee_id = ?", userID).
		Find(&referredBy).Error; err != nil {
		return nil, err
	}

	var referredTo []struct {
		UserID                  string
		Name                    string
		RelationshipType        string
		RelationshipDescription *string
	}

	if err := s.DB.WithContext(ctx).
		Table("referral_relationships").
		Select("referral_relationships.referee_id as user_id, CONCAT(users.first_name, ' ', users.last_name) as name, referral_relationships.relationship_type, referral_relationships.relationship_description").
		Joins("JOIN users ON referral_relationships.referee_id = users.id").
		Where("referral_relationships.referrer_id = ?", userID).
		Find(&referredTo).Error; err != nil {
		return nil, err
	}

	referredByList := make([]map[string]interface{}, len(referredBy))
	for i, rb := range referredBy {
		referredByList[i] = map[string]interface{}{
			"user_id":                  rb.UserID,
			"name":                     rb.Name,
			"relationship_type":        rb.RelationshipType,
			"relationship_description": rb.RelationshipDescription,
		}
	}

	referredToList := make([]map[string]interface{}, len(referredTo))
	for i, rt := range referredTo {
		referredToList[i] = map[string]interface{}{
			"user_id":                  rt.UserID,
			"name":                     rt.Name,
			"relationship_type":        rt.RelationshipType,
			"relationship_description": rt.RelationshipDescription,
		}
	}

	return map[string]interface{}{
		"user_id":             user.ID,
		"full_name":           user.FirstName + " " + user.LastName,
		"email":               user.Email,
		"state":               user.State,
		"city":                user.City,
		"country":             user.Country,
		"role":                user.Role,
		"profile_picture_url": user.ProfilePictureURL,
		"bio":                 user.Bio,
		"personal_meet_link":  user.PersonalMeetLink,
		"uscf_id":             user.USCFID,
		"lichess_username":    user.LichessUsername,
		"chesscom_username":   user.ChesscomUsername,
		"referred_by":         referredByList,
		"referred_to":         referredToList,
	}, nil
}
