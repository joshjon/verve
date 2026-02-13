package app

import (
	"context"
	"fmt"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/pgdb"
	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/sqlitedb"

	"verve/internal/github"
	"verve/internal/postgres"
	pgmigrations "verve/internal/postgres/migrations"
	"verve/internal/sqlite"
	litemigrations "verve/internal/sqlite/migrations"
	"verve/internal/task"
	"verve/internal/taskapi"
)

// Run starts the API server. If cfg.DatabaseURL is empty, it falls back to
// an in-memory SQLite database with a warning.
func Run(ctx context.Context, logger log.Logger, cfg Config) error {
	store, cleanup, err := initStore(ctx, logger, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	gh := github.NewClient(cfg.GitHub.Token, cfg.GitHub.Repo)

	return serve(ctx, logger, cfg, store, gh)
}

func initStore(ctx context.Context, logger log.Logger, cfg Config) (*task.Store, func(), error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL not set, using in-memory SQLite (data will not persist)")
		return initSQLite(ctx)
	}
	return initPostgres(ctx, cfg.Postgres)
}

func initPostgres(ctx context.Context, cfg PostgresConfig) (*task.Store, func(), error) {
	pool, err := pgdb.Dial(ctx, cfg.User, cfg.Password, cfg.HostPort, cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("dial postgres: %w", err)
	}

	if err := pgdb.Migrate(pool, pgmigrations.FS); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate postgres: %w", err)
	}

	repo := postgres.NewTaskRepository(pool)
	store := task.NewStore(repo)
	return store, func() { pool.Close() }, nil
}

func initSQLite(ctx context.Context) (*task.Store, func(), error) {
	db, err := sqlitedb.Open(ctx, sqlitedb.WithInMemory())
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := sqlitedb.Migrate(db, litemigrations.FS); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	repo := sqlite.NewTaskRepository(db)
	store := task.NewStore(repo)
	return store, func() { db.Close() }, nil
}

func serve(ctx context.Context, logger log.Logger, cfg Config, store *task.Store, gh *github.Client) error {
	opts := []server.Option{
		server.WithLogger(logger),
	}
	if len(cfg.CorsOrigins) > 0 {
		opts = append(opts, server.WithCORS(cfg.CorsOrigins...))
	}

	srv, err := server.NewServer(cfg.Port, opts...)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	srv.Register("/api/v1", taskapi.NewHTTPHandler(store, gh))

	// Background PR sync.
	if gh != nil {
		go backgroundSync(ctx, logger, store, gh, 30*time.Second)
		logger.Info("background PR sync started", "interval", "30s")
	}

	return Serve(ctx, logger, srv)
}

// Serve starts the server and blocks until the context is cancelled.
func Serve(ctx context.Context, logger log.Logger, srv *server.Server) error {
	errs := make(chan error)

	logger.Info("starting server", "address", srv.Address())
	go func() {
		defer close(errs)
		if err := srv.Start(); err != nil {
			errs <- fmt.Errorf("start server: %w", err)
		}
	}()
	defer srv.Stop(ctx)

	if err := srv.WaitHealthy(15, time.Second); err != nil {
		return err
	}
	logger.Info("server healthy")

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		logger.Info("server stopped")
		return nil
	}
}

func backgroundSync(ctx context.Context, logger log.Logger, store *task.Store, gh *github.Client, interval time.Duration) {
	logger = logger.With("component", "pr_sync")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tasks, err := store.ListTasksInReview(ctx)
			if err != nil {
				logger.Error("failed to list review tasks", "error", err)
				continue
			}
			for _, t := range tasks {
				if t.PRNumber > 0 {
					merged, err := gh.IsPRMerged(ctx, t.PRNumber)
					if err != nil {
						logger.Error("failed to check PR status", "task_id", t.ID, "error", err)
						continue
					}
					if merged {
						if err := store.UpdateTaskStatus(ctx, t.ID, task.StatusMerged); err != nil {
							logger.Error("failed to update task status", "task_id", t.ID, "error", err)
						} else {
							logger.Info("task PR merged", "task_id", t.ID)
						}
					}
				}
			}
		}
	}
}
