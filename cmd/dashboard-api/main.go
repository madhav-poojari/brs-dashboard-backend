package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/server"
	"github.com/madhava-poojari/dashboard-api/internal/service"
	"github.com/madhava-poojari/dashboard-api/internal/store"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := store.NewGormStore(cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	appServer := server.NewServer(cfg, pool)

	srv := appServer.NewHTTPServer()

	// Start rating cron scheduler (returns instance for graceful shutdown)
	cronScheduler := service.StartRatingCrons(pool)

	// graceful shutdown
	go func() {
		log.Printf("listening on %s", cfg.BindAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down...")

	// Stop cron scheduler — waits for any running job to finish
	cronCtx := cronScheduler.Stop()
	select {
	case <-cronCtx.Done():
		log.Println("Cron jobs finished gracefully")
	case <-time.After(11 * time.Minute):
		log.Println("Cron jobs did not finish in time, forcing shutdown")
	}

	// Shutdown HTTP server
	ctxShutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
