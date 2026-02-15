package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/joshjon/kit/log"

	"verve/internal/worker"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := log.NewLogger(log.WithDevelopment())

	// Load configuration from environment
	cfg := worker.Config{
		APIURL:               getEnvOrDefault("API_URL", "http://localhost:7400"),
		AnthropicAPIKey:      os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeCodeOAuthToken: os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
		ClaudeModel:          getEnvOrDefault("CLAUDE_MODEL", "haiku"),
		AgentImage:           getEnvOrDefault("AGENT_IMAGE", "verve-agent:latest"),
		MaxConcurrentTasks:   getEnvOrDefaultInt(logger, "MAX_CONCURRENT_TASKS", 3),
		DryRun:               os.Getenv("DRY_RUN") == "true",
	}

	// Validate required configuration — need at least one auth method
	if !cfg.DryRun && cfg.AnthropicAPIKey == "" && cfg.ClaudeCodeOAuthToken == "" {
		logger.Error("ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN is required (or set DRY_RUN=true)")
		os.Exit(1)
	}

	authMethod := "api_key"
	if cfg.ClaudeCodeOAuthToken != "" {
		authMethod = "oauth"
	}

	logger.Info("worker configured",
		"api_url", cfg.APIURL,
		"auth", authMethod,
		"model", cfg.ClaudeModel,
		"image", cfg.AgentImage,
		"max_concurrent", cfg.MaxConcurrentTasks,
		"dry_run", cfg.DryRun,
	)

	w, err := worker.New(cfg, logger)
	if err != nil {
		logger.Error("failed to create worker", "error", err)
		os.Exit(1)
	}
	defer w.Close()

	if err := w.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("worker error", "error", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(logger log.Logger, key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
		logger.Warn("invalid integer for env var, using default", "key", key, "default", defaultValue)
	}
	return defaultValue
}
