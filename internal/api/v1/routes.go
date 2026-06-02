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
	attH := NewAttendanceHandler(ss)
	imgH := NewImageHandler(ss, a.cfg)
	scraperH := NewScraperHandler(ss, a.cfg)
	referralH := NewReferralHandler(ss)

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

	// attendance routes (protected; coach/mentor/admin only)
	r.Route("/attendances", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(ss.Store))
			r.Use(auth.RoleMiddleware("coach", "mentor", "admin"))
			r.Post("/", attH.CreateAttendance)
			r.Get("/", attH.ListAttendances)
			r.Get("/{id}", attH.GetAttendance)
			r.Patch("/{id}", attH.UpdateAttendance)
			r.Delete("/{id}", attH.DeleteAttendance)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		authMiddleware := auth.AuthMiddleware(a.store)
		r.With(authMiddleware).Get("/", userH.ListUsers)
		r.With(authMiddleware).Get("/students", userH.GetStudents)
		r.With(authMiddleware).Get("/coaches", userH.GetCoachesForAttendance)
		r.With(authMiddleware).Get("/me", userH.GetSelfProfile)
		r.With(authMiddleware).Post("/reset-password", userH.ResetOwnPassword)
		r.With(authMiddleware).Get("/{id}", userH.GetUser)
		r.With(authMiddleware).Put("/{id}", userH.UpdateUser)

		// Profile picture
		r.With(authMiddleware).Post("/{id}/profile-picture", imgH.UploadProfilePicture)
		r.With(authMiddleware).Delete("/{id}/profile-picture", imgH.DeleteProfilePicture)

		// Gallery
		r.With(authMiddleware).Get("/{id}/gallery", imgH.ListGallery)
		r.With(authMiddleware).Post("/{id}/gallery", imgH.UploadGalleryImage)
		r.With(authMiddleware).Delete("/{id}/gallery/{imageId}", imgH.DeleteGalleryImage)
		r.With(authMiddleware).Patch("/{id}/gallery/{imageId}", imgH.UpdateGalleryImageMetadata)

		// Tournaments (nearby)
		r.With(authMiddleware).Get("/{id}/tournaments", userH.GetTournaments)
	})

	// Image routes (public gallery)
	r.Route("/images", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.With(auth.AuthMiddleware(a.store)).Get("/academy-gallery", imgH.ListAcademyGallery)
	})

	r.Route("/admin", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		adminGroup := r.With(auth.AuthMiddleware(a.store)).With(auth.RoleMiddleware("admin"))

		// User management
		adminGroup.Put("/user/{id}", adminH.UpdateUserStatus)

		// Pending approvals
		adminGroup.Get("/unapproved-users", adminH.GetUnapprovedUsers)

		// Students & coaches (assignment views)
		adminGroup.Get("/students", adminH.GetStudentsWithAssignments)
		adminGroup.Get("/coaches", adminH.GetCoachesWithAssignments)

		adminGroup.Get("/coaches/picker", adminH.GetCoachesPicker)
		adminGroup.Get("/mentors/picker", adminH.GetMentorsPicker)

		// Unified assignment update endpoint (student<->coach, coach<->mentor)
		adminGroup.Put("/assignments", adminH.UpdateAssignments)
	})

	r.Route("/referral-network", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(a.store))
			r.Use(auth.RoleMiddleware("admin"))

			// Read endpoints
			r.Get("/graph", referralH.GetGraph)
			r.Get("/node/{user_id}", referralH.GetNodeDetail)

			// Write endpoints
			r.Post("/relationship", referralH.CreateRelationship)
			r.Put("/relationship/{relationship_id}", referralH.UpdateRelationship)
			r.Delete("/relationship/{relationship_id}", referralH.DeleteRelationship)
		})
	})

	r.Route("/health", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Get("/", HealthHandler(a.store))
	})

	// Rating handler (used by both /ratings and /scraper/uscf routes)
	ratingH := NewRatingHandler(ss)

	// Scraper routes (API-key protected, used by the Chrome extension)
	r.Route("/scraper", func(r chi.Router) {
		r.Use(scraperH.APIKeyAuth)
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Get("/zipcodes", scraperH.GetZipcodes)
		r.Post("/tournaments", scraperH.SubmitTournaments)

		// USCF rating scraper (Chrome extension)
		r.Get("/uscf/students", ratingH.GetUSCFStudents)
		r.Post("/uscf/upload", ratingH.UploadUSCFRatings)
	})

	// Schedule routes (class time slots)
	schedH := NewScheduleHandler(ss)
	r.Route("/schedules", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(ss.Store))
			// Coach/Mentor/Admin CRUD
			r.With(auth.RoleMiddleware("coach", "mentor", "admin")).Post("/", schedH.CreateSchedule)
			r.With(auth.RoleMiddleware("coach", "mentor", "admin")).Get("/", schedH.ListSchedules)
			r.With(auth.RoleMiddleware("coach", "mentor", "admin")).Patch("/{id}", schedH.UpdateSchedule)
			r.With(auth.RoleMiddleware("coach", "mentor", "admin")).Delete("/{id}", schedH.DeleteSchedule)
		})
	})

	// Rating history routes (progress charts)
	r.Route("/ratings", func(r chi.Router) {
		r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(ss.Store))
			r.Get("/{studentId}/{platform}", ratingH.GetStudentPlatformRatings)
			r.With(auth.RoleMiddleware("admin")).Post("/trigger-scrape", ratingH.TriggerScrape)
		})
	})

}
