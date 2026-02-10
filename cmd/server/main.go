package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"verve/internal/server"
)

func main() {
	log.Println("Starting Verve API server...")

	cfg := server.Config{
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		GitHubRepo:  os.Getenv("GITHUB_REPO"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.GitHubToken == "" {
		log.Println("Warning: GITHUB_TOKEN not set - PR status sync disabled")
	}
	if cfg.GitHubRepo == "" {
		log.Println("Warning: GITHUB_REPO not set - PR status sync disabled")
	}
	if cfg.DatabaseURL == "" {
		log.Println("Warning: DATABASE_URL not set - using in-memory storage")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Start background PR status sync
	srv.StartBackgroundSync(ctx, 30*time.Second)

	// Start server in goroutine
	go func() {
		if err := srv.Start(":8080"); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Received shutdown signal, shutting down...")
	cancel()

	// Give the server 10 seconds to finish
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("Server stopped")
}
