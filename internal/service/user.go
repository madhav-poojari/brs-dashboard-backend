package service

import (
	"context"
	"errors"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type UserService struct {
	store *store.Store
}

func NewUserService(s *store.Store) *UserService {
	return &UserService{store: s}
}

func (u *UserService) CreateUser(ctx context.Context, email, password, firstName, lastName string, role models.Role, picture string) (*models.User, error) {
	uid, err := utils.GenerateUserID()
	if err != nil {
		return nil, err
	}
	if password == "" {
		// generate random password if not provided (e.g. for OAuth users)
		p := utils.GenerateRandomString(12)
		password = p
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		ID:           uid,
		Email:        email,
		PasswordHash: hash,
		FirstName:    firstName,
		LastName:     lastName,
		Role:         role,
		Approved:     false, // default to not approved
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	ud := &models.UserDetails{
		UserID:            uid,
		ProfilePictureURL: picture,
	}
	// try create; if conflict on ID (rare), regenerate few times
	for i := 0; i < 5; i++ {
		err = u.store.CreateUser(ctx, user, ud)
		if err == nil {
			return user, nil
		}
		// if unique violation on id/email, try regenerate id
		uid, err2 := utils.GenerateUserID()
		if err2 != nil {
			return nil, err2
		}
		user.ID = uid
	}
	return nil, errors.New("could not create unique user id")
}
