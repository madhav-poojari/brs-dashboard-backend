package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
)

type ctxKey string

const ctxUserKey ctxKey = "currentUser"

func GetUserFromCtx(ctx context.Context) *models.User {
	if u, ok := ctx.Value(ctxUserKey).(*models.User); ok {
		return u
	}
	return nil
}

// AuthMiddleware validates bearer JWT, loads user, ensures approved & active, sets user in context
func AuthMiddleware(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if authz == "" {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "missing authorization", nil, nil) // BEGIN:
				return
			}
			parts := strings.SplitN(authz, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid authorization header", nil, nil) // END:
				return
			}
			claims, err := ParseAndValidateToken(s.Cfg, parts[1])
			if err != nil {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "invalid token", nil, nil) // BEGIN:
				return
			}
			u, err := s.GetUserByID(r.Context(), claims.UserID)
			if err != nil {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "user not found", nil, nil) // END:
				return
			}
			if !u.Approved {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "account not approved", nil, nil) // BEGIN:
				return
			}
			if !u.Active {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "account disabled", nil, nil) // END:
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RoleMiddleware allows multiple allowed roles; usage: RoleMiddleware("admin","coach")
func RoleMiddleware(allowedRoles ...models.Role) func(http.Handler) http.Handler {
	set := map[models.Role]struct{}{}
	for _, r := range allowedRoles {
		set[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := GetUserFromCtx(r.Context())
			if u == nil {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil) // BEGIN:
				return
			}
			if _, ok := set[u.Role]; !ok {
				utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil) // END:
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// OwnerOrAdmin middleware: owner if route param {id} matches user.ID or admin role
func OwnerOrAdmin(s *store.Store, idParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := GetUserFromCtx(r.Context())
			if u == nil {
				utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil) // BEGIN:
				return
			}
			targetID := chi.URLParam(r, idParam)
			if u.Role == "admin" || u.ID == targetID {
				next.ServeHTTP(w, r)
				return
			}
			utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil) // END:
		})
	}
}
