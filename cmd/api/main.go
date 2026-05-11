package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aoms/smart-worker/internal/config"
	"github.com/aoms/smart-worker/internal/database"
	"github.com/aoms/smart-worker/internal/embeddings"
	"github.com/aoms/smart-worker/internal/logger"
	"github.com/aoms/smart-worker/internal/memory"

	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize logger
	l := logger.New(cfg.LogLevel)
	l.Info(ctx, "starting AOMS server")

	// Initialize database connection
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		l.Fatal(ctx, "failed to connect to database", "error", err)
	}
	defer db.Close()

	l.Info(ctx, "database connected successfully")

	// Initialize memory repository
	repo := memory.NewPostgresRepository(db.Conn())

	// Initialize embeddings client
	var embedder *embeddings.Client
	if cfg.OpenAIAPIKey != "" {
		embedder = embeddings.NewClient(cfg.OpenAIAPIKey)
	}

	// Initialize server
	srv := NewServer(repo, embedder)

	// Register routes
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	l.Info(ctx, "server ready", "port", port)
	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil && err != http.ErrServerClosed {
		l.Fatal(ctx, "server error", "error", err)
	}
	l.Info(ctx, "server shutdown complete")
}