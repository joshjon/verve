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
		Port:          7400,
		UI:            os.Getenv("UI") == "true",
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
		Postgres: app.PostgresConfig{
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			HostPort: os.Getenv("POSTGRES_HOST_PORT"),
			Database: os.Getenv("POSTGRES_DATABASE"),
		},
		CorsOrigins: []string{"http://localhost:5173", "http://localhost:8080"},
	}

	if cfg.EncryptionKey == "" {
		logger.Warn("ENCRYPTION_KEY not set, GitHub token storage will be unavailable")
	}

	if err := app.Run(ctx, logger, cfg); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
