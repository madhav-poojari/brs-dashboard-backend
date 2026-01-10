package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	v1 "github.com/madhava-poojari/dashboard-api/internal/api/v1"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/store"
)

type Server struct {
	cfg *config.Config
	db  *store.Store
}

func NewServer(cfg *config.Config, pool *store.Store) *Server {
	return &Server{cfg: cfg, db: pool}
}

func (s *Server) NewHTTPServer() *http.Server {
	r := chi.NewRouter()

	// simple CORS for dev; tighten in prod
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://stage-dashboard.brschess.com",
			"https://stage-dashboard.brschess.com",
			"http://dashboard.brschess.com",
			"https://dashboard.brschess.com",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	api := v1.NewAPI(s.cfg, s.db)
	r.Mount("/api/v1", api.Routes())

	srv := &http.Server{
		Addr:         s.cfg.BindAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// run migrations / create tables if not exists
	// if err := s.db.AutoMigrate(context.Background()); err != nil {
	// 	log.Fatalf("migration failed: %v", err)
	// }

	return srv
}
