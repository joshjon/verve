package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/pgdb"
	"github.com/joshjon/kit/server"
	"github.com/joshjon/kit/sqlitedb"
	"github.com/labstack/echo/v4"

	"verve/internal/epic"
	"verve/internal/epicapi"
	"verve/internal/frontend"
	"verve/internal/github"
	"verve/internal/githubtoken"
	"verve/internal/postgres"
	"verve/internal/setting"
	pgmigrations "verve/internal/postgres/migrations"
	"verve/internal/repo"
	"verve/internal/sqlite"
	litemigrations "verve/internal/sqlite/migrations"
	"verve/internal/task"
	"verve/internal/taskapi"
)

type stores struct {
	task        *task.Store
	repo        *repo.Store
	epic        *epic.Store
	githubToken *githubtoken.Service
	setting     *setting.Service
}

// Run starts the API server. If Postgres is not configured, it falls back to
// an in-memory SQLite database with a warning.
func Run(ctx context.Context, logger log.Logger, cfg Config) error {
	var encryptionKey []byte
	if cfg.EncryptionKey != "" {
		var err error
		encryptionKey, err = hex.DecodeString(cfg.EncryptionKey)
		if err != nil {
			return fmt.Errorf("decode ENCRYPTION_KEY (expected hex): %w", err)
		}
	}

	s, cleanup, err := initStores(ctx, logger, cfg, encryptionKey)
	if err != nil {
		return err
	}
	defer cleanup()

	if s.githubToken != nil {
		if err := s.githubToken.Load(ctx); err != nil {
			logger.Error("failed to load GitHub token from database", "error", err)
		} else if s.githubToken.HasToken() {
			logger.Info("GitHub token loaded from database")
		}
	}

	if s.setting != nil {
		if err := s.setting.Load(ctx); err != nil {
			logger.Error("failed to load settings from database", "error", err)
		}
	}

	return serve(ctx, logger, cfg, s)
}

func initStores(ctx context.Context, logger log.Logger, cfg Config, encryptionKey []byte) (stores, func(), error) {
	if !cfg.Postgres.IsSet() {
		if cfg.SQLiteDir != "" {
			logger.Info("Postgres not configured, using file-backed SQLite", "dir", cfg.SQLiteDir)
		} else {
			logger.Warn("Postgres not configured, using in-memory SQLite (data will not persist)")
		}
		return initSQLite(ctx, cfg.SQLiteDir, encryptionKey)
	}
	return initPostgres(ctx, logger, cfg.Postgres, encryptionKey)
}

func initPostgres(ctx context.Context, logger log.Logger, cfg PostgresConfig, encryptionKey []byte) (stores, func(), error) {
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

	var ghTokenService *githubtoken.Service
	if encryptionKey != nil {
		ghTokenRepo := postgres.NewGitHubTokenRepository(pool)
		ghTokenService = githubtoken.NewService(ghTokenRepo, encryptionKey)
	}

	settingRepo := postgres.NewSettingRepository(pool)
	settingService := setting.NewService(settingRepo)

	epicRepo := postgres.NewEpicRepository(pool)
	taskCreator := epic.NewTaskCreatorFunc(taskStore.CreateTaskFromEpic)
	epicStore := epic.NewStore(epicRepo, taskCreator)

	return stores{task: taskStore, repo: repoStore, epic: epicStore, githubToken: ghTokenService, setting: settingService}, func() { pool.Close() }, nil
}

func initSQLite(ctx context.Context, dir string, encryptionKey []byte) (stores, func(), error) {
	var opts []sqlitedb.OpenOption
	if dir != "" {
		opts = append(opts, sqlitedb.WithDir(dir), sqlitedb.WithDBName("verve"))
	} else {
		opts = append(opts, sqlitedb.WithInMemory())
	}
	db, err := sqlitedb.Open(ctx, opts...)
	if err != nil {
		return stores{}, nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := sqlitedb.Migrate(db, litemigrations.FS); err != nil {
		_ = db.Close()
		return stores{}, nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	broker := task.NewBroker(nil)
	taskRepo := sqlite.NewTaskRepository(db)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := sqlite.NewRepoRepository(db)
	repoStore := repo.NewStore(repoRepo, taskStore)

	var ghTokenService *githubtoken.Service
	if encryptionKey != nil {
		ghTokenRepo := sqlite.NewGitHubTokenRepository(db)
		ghTokenService = githubtoken.NewService(ghTokenRepo, encryptionKey)
	}

	settingRepo := sqlite.NewSettingRepository(db)
	settingService := setting.NewService(settingRepo)

	epicRepo := sqlite.NewEpicRepository(db)
	taskCreator := epic.NewTaskCreatorFunc(taskStore.CreateTaskFromEpic)
	epicStore := epic.NewStore(epicRepo, taskCreator)

	return stores{task: taskStore, repo: repoStore, epic: epicStore, githubToken: ghTokenService, setting: settingService}, func() { _ = db.Close() }, nil
}

func serve(ctx context.Context, logger log.Logger, cfg Config, s stores) error {
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

	// Register UI
	if cfg.UI {
		uiHandler, err := frontend.DistHandler()
		if err != nil {
			return err
		}
		srv.Add(echo.GET, "/*", uiHandler)
	}

	srv.Register("/api/v1", taskapi.NewHTTPHandler(s.task, s.repo, s.githubToken, s.setting))
	srv.Register("/api/v1", epicapi.NewHTTPHandler(s.epic, s.repo))

	// Background PR sync.
	go backgroundSync(ctx, logger, s, 30*time.Second)

	// Background stale task reaper.
	taskTimeout := cfg.TaskTimeout
	if taskTimeout == 0 {
		taskTimeout = 5 * time.Minute
	}
	go backgroundReaper(ctx, logger, s, 1*time.Minute, taskTimeout)

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
	defer func() { _ = srv.Stop(ctx) }()

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

func backgroundReaper(ctx context.Context, logger log.Logger, s stores, interval, timeout time.Duration) {
	logger = logger.With("component", "task_reaper")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := s.task.TimeoutStaleTasks(ctx, timeout)
			if err != nil {
				logger.Error("failed to timeout stale tasks", "error", err)
			} else if count > 0 {
				logger.Info("timed out stale tasks", "count", count)
			}
		}
	}
}

func backgroundSync(ctx context.Context, logger log.Logger, s stores, interval time.Duration) {
	logger = logger.With("component", "pr_sync")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.githubToken == nil {
				continue
			}
			gh := s.githubToken.GetClient()
			if gh == nil {
				continue
			}
			fineGrained := s.githubToken.IsFineGrained()

			// Sync branch-only tasks: check if PRs were manually created.
			branchTasks, err := s.task.ListTasksInReviewNoPR(ctx)
			if err != nil {
				logger.Error("failed to list branch-only tasks", "error", err)
			} else {
				for _, t := range branchTasks {
					if t.BranchName == "" {
						continue
					}
					// Look up repo for this task.
					repoID, parseErr := repo.ParseRepoID(t.RepoID)
					if parseErr != nil {
						continue
					}
					r, readErr := s.repo.ReadRepo(ctx, repoID)
					if readErr != nil {
						continue
					}
					prURL, prNumber, findErr := gh.FindPRForBranch(ctx, r.Owner, r.Name, t.BranchName)
					if findErr != nil {
						logger.Error("failed to find PR for branch", "task_id", t.ID, "branch", t.BranchName, "error", findErr)
						continue
					}
					if prNumber > 0 {
						if err := s.task.SetTaskPullRequest(ctx, t.ID, prURL, prNumber); err != nil {
							logger.Error("failed to link PR to task", "task_id", t.ID, "error", err)
						} else {
							logger.Info("linked PR to branch-only task", "task_id", t.ID, "pr_number", prNumber)
						}
					}
				}
			}

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

					// 3. Check CI status (skipped for fine-grained tokens).
					if fineGrained {
						continue
					}
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

						// Build category from failed check names so the circuit
						// breaker only trips when the exact same checks keep failing.
						category := "ci_failure"
						if len(checkResult.FailedNames) > 0 {
							names := make([]string, len(checkResult.FailedNames))
							copy(names, checkResult.FailedNames)
							sort.Strings(names)
							category = "ci_failure:" + strings.Join(names, ",")
						}
						reason := fmt.Sprintf("%s: %s", category, checkResult.Summary)
						if err := s.task.RetryTask(ctx, t.ID, category, reason); err != nil {
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
