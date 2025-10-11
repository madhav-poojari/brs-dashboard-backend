package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/api/v1"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/store"

	"github.com/go-chi/cors"
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
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
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
