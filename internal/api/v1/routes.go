package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware" // <--- Add this

	// "github.com/go-chi/cors"
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
	api.router.Use(middleware.Logger)
	// Use cors.Handler (not middleware.CORS)
	// api.router.Use(cors.Handler(cors.Options{
	// AllowedOrigins: []string{
	// 	"http://localhost:5173",
	// 	"http://stage-dashboard.brschess.com",
	// 	"https://stage-dashboard.brschess.com",
	// 	"http://dashboard.brschess.com",
	// 	"https://dashboard.brschess.com",
	// },
	// AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	// AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	// ExposedHeaders:   []string{"Link"},
	// AllowCredentials: true,
	// MaxAge:           300,
	// }))

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
	notesH := NewNotesHandler(ss)

	r := a.router
	// auth routes
	r.Route("/auth", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Post("/signup", authH.Signup)
		r.Post("/login", authH.Login)
		r.Post("/logout", authH.Logout)
		r.Post("/refresh", authH.Refresh)
		r.Post("/google", authH.GoogleSignIn)
	})
	// notes routes (protected)
	r.Route("/notes", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(ss.Store))
			r.Post("/", notesH.CreateNote)
			r.Post("/lesson-plans", notesH.CreateLessonPlan)
			r.Patch("/lesson-plans/{id}", notesH.UpdateLessonPlan)
			r.Get("/", notesH.GetNotesByUser)
			r.Patch("/{id}", notesH.UpdateNote)
			r.Delete("/{id}", notesH.DeleteNote)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.With(auth.AuthMiddleware(a.store)).Get("/", userH.ListUsers)
		r.With(auth.AuthMiddleware(a.store)).Get("/me", userH.GetSelfProfile)
		r.With(auth.AuthMiddleware(a.store)).Get("/{id}", userH.GetUser)
		r.With(auth.AuthMiddleware(a.store)).Put("/{id}", userH.UpdateUser)
	})

	r.Route("/admin", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})

		// All admin routes require authentication and admin role
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(a.store))
			r.Use(auth.RoleMiddleware("admin"))

			// Dashboard data - get all data in one call
			r.Get("/dashboard", adminH.GetAdminDashboard)

			// Pending approvals
			r.Get("/pending-approvals", adminH.GetPendingApprovals)
			r.Post("/users/{id}/approve", adminH.ApproveUser)

			// User management
			r.Put("/user/{id}", adminH.UpdateUserStatus)
			r.Get("/users", adminH.ListAllUsers)

			// Students
			r.Get("/students", adminH.GetStudentsWithAssignments)
			r.Post("/students/assign", adminH.AssignStudentToCoach)

			// Coaches
			r.Get("/coaches", adminH.GetCoachesWithAssignments)
			r.Get("/coaches/all", adminH.GetAllCoaches)
			r.Post("/coaches/assign", adminH.AssignCoachToMentor)

			// Mentor coaches
			r.Get("/mentors", adminH.GetMentorCoaches)
		})
	})

	r.Route("/health", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Get("/", HealthHandler(a.store))
	})
}
