package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		SQLiteDir:     os.Getenv("SQLITE_DIR"),
		EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
		Postgres: app.PostgresConfig{
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			HostPort: os.Getenv("POSTGRES_HOST_PORT"),
			Database: os.Getenv("POSTGRES_DATABASE"),
		},
		CorsOrigins: []string{"http://localhost:5173", "http://localhost:8080"},
	}

	if v := os.Getenv("TASK_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			logger.Error("invalid TASK_TIMEOUT", "value", v, "error", err)
			os.Exit(1)
		}
		cfg.TaskTimeout = d
	}

	if cfg.EncryptionKey == "" {
		logger.Warn("ENCRYPTION_KEY not set, GitHub token storage will be unavailable")
	}

	if err := app.Run(ctx, logger, cfg); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
