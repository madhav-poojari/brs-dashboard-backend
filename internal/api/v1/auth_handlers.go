package v1

import (
	"context"
	"encoding/json"
	"strings"

	// "errors"
	"net/http"
	"time"

	"google.golang.org/api/idtoken"

	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthHandler struct {
	cfg   *config.Config
	user  *service.UserService
	store *serviceStore // wrapper to access store functions (you can pass the store directly)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func NewAuthHandler(cfg *config.Config, userSvc *service.UserService, store serviceStore) *AuthHandler {
	return &AuthHandler{cfg: cfg, user: userSvc, store: &store}
}

// Signup handler
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {

	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"` // Optional: "student", "coach", or "mentor"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid Request body", nil, err.Error())
		return
	}

	// Validate and set role
	role := models.RoleStudent // default role
	if req.Role != "" {
		roleStr := strings.ToLower(strings.TrimSpace(req.Role))
		switch roleStr {
		case "student":
			role = models.RoleStudent
		case "coach":
			role = models.RoleCoach
		case "mentor":
			role = models.RoleMentor
		default:
			utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid role. Must be 'student', 'coach', or 'mentor'", nil, nil)
			return
		}
	}

	user, err := h.user.CreateUser(r.Context(), req.Email, req.Password, req.FirstName, req.LastName, role, "")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "error creating user", nil, err.Error())
		return
	}
	// do not return tokens; user must be approved first
	utils.WriteJSONResponse(w, http.StatusCreated, true, "user created, pending approval", map[string]interface{}{
		"user_id": user.ID,
		"role":    user.Role,
	}, nil)
	return
}

// Login handler
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid request", nil, err.Error())
		return
	}
	u, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "Invalid request", nil, err.Error())
		return
	}
	ok, err := utils.ComparePasswordAndHash(req.Password, u.PasswordHash)
	if err != nil || !ok {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid credentials", nil, nil)
		return
	}
	if !u.Approved {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "account pending approval", nil, nil)
		return
	}
	// generate access token
	access, err := auth.GenerateAccessToken(h.cfg, u.ID, string(u.Role))
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "token error", nil, err.Error())
		return
	}
	// generate refresh token (random string)
	rt := utils.RandomToken()
	expires := time.Now().Add(h.cfg.RefreshTokenTTL)
	if err := h.store.SaveRefreshToken(r.Context(), u.ID, rt, expires); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "save refresh token error", nil, err.Error())
		return
	}

	host := r.Host // example: "api.myapp.com" or "localhost:8080"
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	cookieDomain := host

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rt,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, //set true in production
		SameSite: http.SameSiteLaxMode,
		Domain:   cookieDomain,
		Expires:  expires,
	})

	resp := tokenResp{AccessToken: access, ExpiresIn: int64(h.cfg.AccessTokenTTL.Seconds())}
	utils.WriteJSONResponse(w, http.StatusOK, true, "login successful", resp, nil)
}

// Logout handler expects {"refresh_token":"..."}
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Expect refresh token in cookie named "refresh_token"
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing refresh token cookie", nil, err.Error())
		return
	}
	refreshToken := cookie.Value

	// Revoke refresh token in store
	if err := h.store.RevokeRefreshToken(r.Context(), refreshToken); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "revoke error", nil, err.Error())
		return
	}

	// Clear refresh_token cookie (httpOnly) and also clear session cookie "sid" if present
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	utils.WriteJSONResponse(w, http.StatusOK, true, "logged out", nil, nil)
}

// Refresh handler rotates refresh tokens and returns new access + refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// cookie, err := r.Cookie("refresh_token")
	// if err != nil {
	//     http.Error(w, "Unauthorized", http.StatusUnauthorized)
	//     return
	// }

	// // 2. Verify and Generate new Access Token
	// newAccessToken, err := h.svc.RefreshAccessToken(cookie.Value)
	// if err != nil {
	//     http.Error(w, "Unauthorized", http.StatusUnauthorized)
	//     return
	// }
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing refresh token cookie", nil, err.Error())
		return
	}
	req.RefreshToken = cookie.Value

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// 1) verify token exists and not revoked
	rt, err := h.store.FindRefreshToken(ctx, req.RefreshToken)
	if err != nil || rt == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid refresh token", nil, nil)
		return
	}

	// 2) rotate refresh token (revokes old, inserts new) — your RotateRefreshToken does this in a tx
	newPlain := utils.RandomToken()
	newExpiry := time.Now().Add(h.cfg.RefreshTokenTTL)
	if _, err := h.store.RotateRefreshToken(ctx, req.RefreshToken, newPlain, newExpiry); err != nil {
		// rotation failed (token may have been concurrently revoked/expired)
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid refresh token", nil, nil)
		return
	}
	u, err := h.store.GetUserByID(r.Context(), rt.UserID)
	if err != nil || u == nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "user not found", nil, err.Error())
		return
	}
	// 3) create short-lived access token for the same user (assumes a token creator on handler)
	accessToken, err := auth.GenerateAccessToken(h.cfg, u.ID, string(u.Role))
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "could not create access token", nil, nil)
		return
	}

	// 4) respond with rotated refresh token and new access token
	host := r.Host // example: "api.myapp.com" or "localhost:8080"

	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	cookieDomain := host

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newPlain,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, //set true in production
		SameSite: http.SameSiteLaxMode,
		Domain:   cookieDomain,
		Expires:  newExpiry,
	})

	resp := tokenResp{AccessToken: accessToken, ExpiresIn: int64(h.cfg.AccessTokenTTL.Seconds())}
	utils.WriteJSONResponse(w, http.StatusOK, true, "refresh successful", resp, nil)
}

func (h *AuthHandler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"` // now expecting authorization code from client
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "bad request", nil, "missing code")
		return
	}

	ctx := context.Background()
	oauthCfg := &oauth2.Config{
		ClientID:     h.cfg.GoogleClientID,
		ClientSecret: h.cfg.GoogleClientSecret,
		RedirectURL:  h.cfg.GoogleRedirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}

	// Exchange the code for tokens (server-side exchange using client secret)
	token, err := oauthCfg.Exchange(ctx, req.Code)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "code exchange failed", nil, err.Error())
		return
	}

	// Extract the ID token from the token response
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "id_token not present in token response", nil, nil)
		return
	}

	// Validate id_token (audience must be our client id)
	payload, err := idtoken.Validate(ctx, rawIDToken, h.cfg.GoogleClientID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid id token", nil, err.Error())
		return
	}

	// Pull email/name from token claims
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "email not present in token", nil, nil)
		return
	}
	firstName, _ := payload.Claims["given_name"].(string)
	lastName, _ := payload.Claims["family_name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	// find or create user (passwordless)
	u, err := h.store.GetUserByEmail(ctx, email)
	if err != nil {
		// Create user with nil password (passwordless Google user). CreateUser should accept empty password.
		user, err2 := h.user.CreateUser(ctx, email, "", firstName, lastName, "student", picture)
		if err2 != nil {
			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error creating user", nil, err2.Error())
			return
		}
		// load the created user fully (or use returned user)
		u = &models.User{ID: user.ID, Email: user.Email, Role: user.Role, Approved: user.Approved}
		// better to fetch full row if needed
		u, _ = h.store.GetUserByID(ctx, user.ID)
	}

	// if user exists but not approved -> pending
	if !u.Approved {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "account pending approval", nil, nil)
		return
	}

	// success: issue our access + refresh tokens
	access, err := auth.GenerateAccessToken(h.cfg, u.ID, string(u.Role))
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "token generation error", nil, err.Error())
		return
	}

	rt := utils.RandomToken()
	expires := time.Now().Add(h.cfg.RefreshTokenTTL)

	if err := h.store.SaveRefreshToken(ctx, u.ID, rt, expires); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "save refresh token error", nil, err.Error())
		return
	}
	host := r.Host // example: "api.myapp.com" or "localhost:8080"

	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	cookieDomain := host

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rt,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, //set true in production
		SameSite: http.SameSiteLaxMode,
		Domain:   cookieDomain,
		Expires:  expires,
	})

	resp := tokenResp{AccessToken: access, ExpiresIn: int64(h.cfg.AccessTokenTTL.Seconds())}
	utils.WriteJSONResponse(w, http.StatusOK, true, "login successful", resp, nil)
}

// // Google sign-in: accept id_token, validate, create/link user, return tokens if approved
// func (h *AuthHandler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
// 	var req struct {
// 		IDToken string `json:"id_token"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IDToken == "" {
// 		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "bad request", nil, err.Error())
// 		return
// 	}
// 	ctx := context.Background()
// 	payload, err := idtoken.Validate(ctx, req.IDToken, h.cfg.GoogleClientID)
// 	if err != nil {
// 		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid id token", nil, err.Error())
// 		return
// 	}
// 	email, ok := payload.Claims["email"].(string)
// 	if !ok || email == "" {
// 		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "email not present in token", nil, nil)
// 		return
// 	}
// 	firstName, _ := payload.Claims["given_name"].(string)
// 	lastName, _ := payload.Claims["family_name"].(string)

// 	// find user by email
// 	u, err := h.store.GetUserByEmail(ctx, email)
// 	if err != nil {
// 		// create new user with blank password and approved=false
// 		user, err2 := h.user.CreateUser(ctx, email, "", firstName, lastName, "student")
// 		if err2 != nil {
// 			utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "error creating user", nil, err2.Error())
// 			return
// 		}
// 		u = &models.User{
// 			ID: user.ID,
// 			// rest are zero values
// 		}
// 	}
// 	// if user exists but not approved, return pending
// 	if !u.Approved {
// 		utils.WriteJSONResponse(w, http.StatusForbidden, false, "account pending approval", nil, nil)
// 		return
// 	}
// 	// create tokens (same as login)
// 	access, err := auth.GenerateAccessToken(h.cfg, u.ID, string(u.Role))
// 	if err != nil {
// 		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "token error", nil, err.Error())
// 		return
// 	}
// 	rt := utils.RandomToken()
// 	expires := time.Now().Add(h.cfg.RefreshTokenTTL)
// 	if err := h.store.SaveRefreshToken(ctx, u.ID, rt, expires); err != nil {
// 		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "save refresh token error", nil, err.Error())
// 		return
// 	}
// 	resp := tokenResp{AccessToken: access, RefreshToken: rt, ExpiresIn: int64(h.cfg.AccessTokenTTL.Seconds())}
// 	utils.WriteJSONResponse(w, http.StatusOK, true, "login successful", resp, nil)
// }
