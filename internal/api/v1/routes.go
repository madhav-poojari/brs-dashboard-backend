package v1

import (
	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
)

type serviceStore struct {
	*store.Store
}

type API struct {
	cfg    *config.Config
	router *chi.Mux
	store  *store.Store
}

func NewAPI(cfg *config.Config, s *store.Store) *API {
	api := &API{cfg: cfg, router: chi.NewRouter(), store: s}
	api.routes()
	return api
}

func (a *API) Routes() *chi.Mux {
	return a.router
}

func (a *API) routes() {
	usvc := service.NewUserService(a.store)
	ss := serviceStore{a.store}

	authH := NewAuthHandler(a.cfg, usvc, ss)
	userH := NewUserHandler(ss)
	adminH := NewAdminHandler(ss)

	r := a.router
	// auth routes
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", authH.Signup)
		r.Post("/login", authH.Login)
		r.Post("/logout", authH.Logout)
		r.Post("/refresh", authH.Refresh)
		r.Post("/google", authH.GoogleSignIn)
	})

	r.Route("/users", func(r chi.Router) {
		r.With(auth.AuthMiddleware(a.store)).Get("/", userH.ListUsers)
		r.With(auth.AuthMiddleware(a.store)).Get("/me", userH.GetSelfProfile)
		r.With(auth.AuthMiddleware(a.store)).Get("/{id}", userH.GetUser)
		r.With(auth.AuthMiddleware(a.store)).Put("/{id}", userH.UpdateUser)
	})

	r.Route("/admin", func(r chi.Router) {
		// r.With(auth.AuthMiddleware(a.store)).With(auth.RoleMiddleware("admin")).Get("/dashboard", adminH.AdminDashboard)
		r.With(auth.AuthMiddleware(a.store)).With(auth.RoleMiddleware("admin")).Put("/user/{id}", adminH.UpdateUserStatus)
	})

	r.Route("/health", func(r chi.Router) {
		r.Get("/", HealthHandler(a.store))
	})
}
