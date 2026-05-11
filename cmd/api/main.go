package main

import (
	"context"
	"log"
	"os"

	"github.com/aoms/smart-worker/internal/config"
	"github.com/aoms/smart-worker/internal/database"
	"github.com/aoms/smart-worker/internal/logger"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize logger
	log := logger.New(cfg.LogLevel)
	log.Info(ctx, "starting AOMS server")

	// Initialize database connection
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(ctx, "failed to connect to database", "error", err)
	}
	defer db.Close()

	log.Info(ctx, "database connected successfully")

	// Server would be started here in implementation phase
	// For now, this is infrastructure skeleton only

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Info(ctx, "server ready", "port", port)
	log.Info(ctx, "AOMS infrastructure initialized")
}