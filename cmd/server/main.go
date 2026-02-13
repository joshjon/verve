package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshjon/kit/log"

	"verve/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := log.NewLogger(log.WithDevelopment())

	cfg := app.Config{
		Port:        7400,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Postgres: app.PostgresConfig{
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			HostPort: os.Getenv("POSTGRES_HOST_PORT"),
			Database: os.Getenv("POSTGRES_DATABASE"),
		},
		GitHub: app.GitHubConfig{
			Token: os.Getenv("GITHUB_TOKEN"),
			Repo:  os.Getenv("GITHUB_REPO"),
		},
		CorsOrigins: []string{"http://localhost:5173", "http://localhost:8080"},
	}

	if cfg.GitHub.Token == "" {
		logger.Warn("GITHUB_TOKEN not set, PR status sync disabled")
	}
	if cfg.GitHub.Repo == "" {
		logger.Warn("GITHUB_REPO not set, PR status sync disabled")
	}

	if err := app.Run(ctx, logger, cfg); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
