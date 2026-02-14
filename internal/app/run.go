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
	"verve/internal/repo"
	"verve/internal/sqlite"
	litemigrations "verve/internal/sqlite/migrations"
	"verve/internal/task"
	"verve/internal/taskapi"
)

type stores struct {
	task *task.Store
	repo *repo.Store
}

// Run starts the API server. If cfg.DatabaseURL is empty, it falls back to
// an in-memory SQLite database with a warning.
func Run(ctx context.Context, logger log.Logger, cfg Config) error {
	s, cleanup, err := initStores(ctx, logger, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	gh := github.NewClient(cfg.GitHub.Token)

	return serve(ctx, logger, cfg, s, gh)
}

func initStores(ctx context.Context, logger log.Logger, cfg Config) (stores, func(), error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL not set, using in-memory SQLite (data will not persist)")
		return initSQLite(ctx)
	}
	return initPostgres(ctx, logger, cfg.Postgres)
}

func initPostgres(ctx context.Context, logger log.Logger, cfg PostgresConfig) (stores, func(), error) {
	pool, err := pgdb.Dial(ctx, cfg.User, cfg.Password, cfg.HostPort, cfg.Database)
	if err != nil {
		return stores{}, nil, fmt.Errorf("dial postgres: %w", err)
	}

	if err := pgdb.Migrate(pool, pgmigrations.FS); err != nil {
		pool.Close()
		return stores{}, nil, fmt.Errorf("migrate postgres: %w", err)
	}

	notifier := postgres.NewEventNotifier(pool, logger)
	broker := task.NewBroker(notifier)
	go notifier.Listen(ctx, broker)

	taskRepo := postgres.NewTaskRepository(pool)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := postgres.NewRepoRepository(pool)
	repoStore := repo.NewStore(repoRepo, taskStore)

	return stores{task: taskStore, repo: repoStore}, func() { pool.Close() }, nil
}

func initSQLite(ctx context.Context) (stores, func(), error) {
	db, err := sqlitedb.Open(ctx, sqlitedb.WithInMemory())
	if err != nil {
		return stores{}, nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := sqlitedb.Migrate(db, litemigrations.FS); err != nil {
		db.Close()
		return stores{}, nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	broker := task.NewBroker(nil)
	taskRepo := sqlite.NewTaskRepository(db)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo, taskStore)

	return stores{task: taskStore, repo: repoStore}, func() { db.Close() }, nil
}

func serve(ctx context.Context, logger log.Logger, cfg Config, s stores, gh *github.Client) error {
	opts := []server.Option{
		server.WithLogger(logger),
		server.WithRequestTimeout(server.DefaultRequestTimeout, "/api/v1/events", "/api/v1/tasks/:id/logs"),
	}
	if len(cfg.CorsOrigins) > 0 {
		opts = append(opts, server.WithCORS(cfg.CorsOrigins...))
	}

	srv, err := server.NewServer(cfg.Port, opts...)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	srv.Register("/api/v1", taskapi.NewHTTPHandler(s.task, s.repo, gh))

	// Background PR sync.
	if gh != nil {
		go backgroundSync(ctx, logger, s, gh, 30*time.Second)
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

func backgroundSync(ctx context.Context, logger log.Logger, s stores, gh *github.Client, interval time.Duration) {
	logger = logger.With("component", "pr_sync")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			repos, err := s.repo.ListRepos(ctx)
			if err != nil {
				logger.Error("failed to list repos", "error", err)
				continue
			}
			for _, r := range repos {
				tasks, err := s.task.ListTasksInReviewByRepo(ctx, r.ID.String())
				if err != nil {
					logger.Error("failed to list review tasks", "repo", r.FullName, "error", err)
					continue
				}
				for _, t := range tasks {
					if t.PRNumber <= 0 {
						continue
					}

					// 1. Check if merged (terminal positive).
					merged, err := gh.IsPRMerged(ctx, r.Owner, r.Name, t.PRNumber)
					if err != nil {
						logger.Error("failed to check PR merged", "task_id", t.ID, "error", err)
						continue
					}
					if merged {
						if err := s.task.UpdateTaskStatus(ctx, t.ID, task.StatusMerged); err != nil {
							logger.Error("failed to update task status", "task_id", t.ID, "error", err)
						} else {
							logger.Info("task PR merged", "task_id", t.ID)
						}
						continue
					}

					// 2. Check for merge conflicts.
					mergeability, err := gh.GetPRMergeability(ctx, r.Owner, r.Name, t.PRNumber)
					if err != nil {
						logger.Error("failed to check mergeability", "task_id", t.ID, "error", err)
						continue
					}
					if mergeability.HasConflicts {
						logger.Info("PR has merge conflicts, retrying", "task_id", t.ID, "attempt", t.Attempt)
						reason := "merge_conflict: PR has conflicts with base branch"
						if err := s.task.RetryTask(ctx, t.ID, "merge_conflict", reason); err != nil {
							logger.Error("failed to retry task", "task_id", t.ID, "error", err)
						}
						continue
					}

					// 3. Check CI status.
					checkResult, err := gh.GetPRCheckStatus(ctx, r.Owner, r.Name, t.PRNumber)
					if err != nil {
						logger.Error("failed to check CI status", "task_id", t.ID, "error", err)
						continue
					}
					if checkResult.Status == github.CheckStatusFailure {
						logger.Info("PR checks failed, retrying", "task_id", t.ID, "attempt", t.Attempt, "summary", checkResult.Summary)

						// Fetch actual CI failure logs for targeted retry
						failureLogs, logErr := gh.GetFailedCheckLogs(ctx, r.Owner, r.Name, t.PRNumber)
						if logErr != nil {
							logger.Warn("failed to fetch CI logs", "task_id", t.ID, "error", logErr)
						} else if failureLogs != "" {
							if err := s.task.SetRetryContext(ctx, t.ID, failureLogs); err != nil {
								logger.Warn("failed to set retry context", "task_id", t.ID, "error", err)
							}
						}

						reason := fmt.Sprintf("ci_failure: %s", checkResult.Summary)
						if err := s.task.RetryTask(ctx, t.ID, "ci_failure", reason); err != nil {
							logger.Error("failed to retry task", "task_id", t.ID, "error", err)
						}
						continue
					}
					// If pending, do nothing — wait for checks to complete.
				}
			}
		}
	}
}
