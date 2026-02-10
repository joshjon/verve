package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"verve/internal/worker"
)

func main() {
	// Load configuration from environment
	cfg := worker.Config{
		APIURL:             getEnvOrDefault("API_URL", "http://localhost:8080"),
		GitHubToken:        os.Getenv("GITHUB_TOKEN"),
		GitHubRepo:         os.Getenv("GITHUB_REPO"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:        getEnvOrDefault("CLAUDE_MODEL", "haiku"),
		AgentImage:         getEnvOrDefault("AGENT_IMAGE", "verve-agent:latest"),
		MaxConcurrentTasks: getEnvOrDefaultInt("MAX_CONCURRENT_TASKS", 3),
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
	log.Printf("Using agent image: %s", cfg.AgentImage)
	log.Printf("Max concurrent tasks: %d", cfg.MaxConcurrentTasks)

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

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		log.Printf("Warning: invalid integer for %s, using default %d", key, defaultValue)
	}
	return defaultValue
}
