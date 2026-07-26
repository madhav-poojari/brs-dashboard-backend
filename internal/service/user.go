package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/apperrors"
	"github.com/madhava-poojari/dashboard-api/internal/cache"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type UserService struct {
	store *store.Store
	rc    cache.Client
}

func NewUserService(s *store.Store, r cache.Client) *UserService {
	return &UserService{store: s, rc: r}
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

func (u *UserService) GetPersonInfoByID(
	ctx context.Context,
	id string,
) (*models.PersonInfo, error) {

	if id == "" {
		return nil, nil
	}

	user, err := u.store.GetUserByID(ctx, id)
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

func (u *UserService) GetSelfProfile(ctx context.Context, currId string) (*models.UserResponse, error) {
	key := fmt.Sprintf("User:Profile:%s", currId)
	userString, err := u.rc.Get(ctx, key)
	if err == nil {
		var resp *models.UserResponse
		err := json.Unmarshal([]byte(userString), &resp)
		if err != nil {
			return nil, err
		}
		print("returning from cache")
		return resp, nil
	}
	print("its a cache miss")
	user, err := u.store.GetUserByID(ctx, currId)
	if err != nil {
		errString := fmt.Sprintf("user info not found: %w", err)
		return nil, errors.New(errString)
	}

	var coachInfo *models.PersonInfo
	var mentorInfo *models.PersonInfo

	// fetch assigned coach & mentor
	coachId, mentorId, _ := u.store.GetCoachesByStudentID(ctx, currId)

	mentorInfo, err = u.GetPersonInfoByID(ctx, mentorId)
	if err != nil {
		errString := fmt.Sprintf("failed to fetch mentor: %w", err)
		return nil, errors.New(errString)
	}

	coachInfo, err = u.GetPersonInfoByID(ctx, coachId)
	if err != nil {
		errString := fmt.Sprintf("failed to fetch coach: %w", err)
		return nil, errors.New(errString)
	}

	resp := models.UserResponse{
		User:   user,
		Coach:  coachInfo,
		Mentor: mentorInfo,
	}

	// Embed schedule for student profiles
	if user.Role == models.RoleStudent {
		schedule, err := u.store.ListSchedulesForStudents(ctx, []string{user.ID})
		if err == nil {
			resp.Schedule = schedule
		}
	}

	value, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	dur, _ := time.ParseDuration("4h30m")
	print("writing into cache")
	err = u.rc.Set(ctx, key, string(value), dur)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (u *UserService) ListUsers(ctx context.Context, role models.Role, currId string) ([]*models.User, error) {
	key := fmt.Sprintf("UsersList:%s", currId)
	userListString, err := u.rc.Get(ctx, key)
	if err == nil {
		var users []*models.User
		err := json.Unmarshal([]byte(userListString), &users)
		if err == nil {
			fmt.Println("returning from cache")
			return users, nil
		}
	}
	fmt.Println("cache miss...")
	if role == "admin" {
		users, err := u.store.ListUsersAdmin(ctx)
		if err != nil {
			return nil, apperrors.NewAppError(http.StatusInternalServerError, err.Error())
		}
		value, _ := json.Marshal(users)
		fmt.Println("writing into cache")
		dur, _ := time.ParseDuration("4h30m")
		err = u.rc.Set(ctx, key, string(value), dur)
		return users, nil
	}

	if role == "student" {
		return nil, apperrors.NewAppError(http.StatusForbidden, "Student donot have access")
	}

	// coach or mentor (or both) -> single DB query with OR
	users, err := u.store.ListStudentsForCoachOrMentor(ctx, currId)
	if err != nil {
		return nil, apperrors.NewAppError(http.StatusInternalServerError, err.Error())
	}
	value, _ := json.Marshal(users)
	fmt.Println("writing into cache")
	dur, _ := time.ParseDuration("4h30m")
	err = u.rc.Set(ctx, key, string(value), dur)
	return users, nil
}

func NewAppError(i int, s string) {
	panic("unimplemented")
}
