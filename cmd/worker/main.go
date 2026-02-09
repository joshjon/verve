package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"verve/internal/worker"
)

func main() {
	// Load configuration from environment
	cfg := worker.Config{
		APIURL:          getEnvOrDefault("API_URL", "http://localhost:8080"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubRepo:      os.Getenv("GITHUB_REPO"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:     getEnvOrDefault("CLAUDE_MODEL", "haiku"),
	}

	// Validate required configuration
	if cfg.GitHubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}
	if cfg.GitHubRepo == "" {
		log.Fatal("GITHUB_REPO environment variable is required (e.g., owner/repo)")
	}
	if cfg.AnthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	log.Printf("Connecting to API server at %s", cfg.APIURL)
	log.Printf("Configured for repository: %s", cfg.GitHubRepo)
	log.Printf("Using Claude model: %s", cfg.ClaudeModel)

	w, err := worker.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}
	defer w.Close()

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	if err := w.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Worker error: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
