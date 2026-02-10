package server

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_create_tasks.sql
var schemaMigration string

// PostgresStore implements persistent task storage using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool

	// Channel to notify waiters when a new pending task is available
	pendingMu sync.Mutex
	pendingCh chan struct{}
}

// NewPostgresStore creates a new PostgreSQL-backed store.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if _, err := pool.Exec(ctx, schemaMigration); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &PostgresStore{
		pool:      pool,
		pendingCh: make(chan struct{}, 1),
	}, nil
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// Create creates a new task.
func (s *PostgresStore) Create(description string, opts *CreateOptions) (*Task, error) {
	ctx := context.Background()

	dependsOn := make([]string, 0)
	if opts != nil && len(opts.DependsOn) > 0 {
		// Validate all dependencies exist
		for _, depID := range opts.DependsOn {
			var exists bool
			err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)", depID).Scan(&exists)
			if err != nil {
				return nil, fmt.Errorf("failed to check dependency: %w", err)
			}
			if !exists {
				return nil, fmt.Errorf("dependency task not found: %s", depID)
			}
		}
		dependsOn = opts.DependsOn
	}

	task := &Task{
		ID:          "tsk_" + uuid.New().String()[:8],
		Description: description,
		Status:      TaskStatusPending,
		Logs:        []string{},
		DependsOn:   dependsOn,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO tasks (id, description, status, logs, depends_on, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, task.ID, task.Description, task.Status, task.Logs, task.DependsOn, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Signal that a new pending task is available
	s.notifyPending()

	return task, nil
}

// Get retrieves a task by ID.
func (s *PostgresStore) Get(id string) *Task {
	ctx := context.Background()
	task, err := s.getTask(ctx, id)
	if err != nil {
		return nil
	}
	return task
}

func (s *PostgresStore) getTask(ctx context.Context, id string) (*Task, error) {
	var task Task
	err := s.pool.QueryRow(ctx, `
		SELECT id, description, status, logs,
			COALESCE(pull_request_url, ''), COALESCE(pr_number, 0),
			depends_on, COALESCE(close_reason, ''),
			created_at, updated_at
		FROM tasks WHERE id = $1
	`, id).Scan(
		&task.ID, &task.Description, &task.Status, &task.Logs,
		&task.PullRequestURL, &task.PRNumber, &task.DependsOn, &task.CloseReason,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Ensure slices are never nil (for proper JSON serialization)
	if task.Logs == nil {
		task.Logs = []string{}
	}
	if task.DependsOn == nil {
		task.DependsOn = []string{}
	}
	return &task, nil
}

// List returns all tasks.
func (s *PostgresStore) List() []*Task {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT id, description, status, logs,
			COALESCE(pull_request_url, ''), COALESCE(pr_number, 0),
			depends_on, COALESCE(close_reason, ''),
			created_at, updated_at
		FROM tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return []*Task{}
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID, &task.Description, &task.Status, &task.Logs,
			&task.PullRequestURL, &task.PRNumber, &task.DependsOn, &task.CloseReason,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			continue
		}
		// Ensure slices are never nil
		if task.Logs == nil {
			task.Logs = []string{}
		}
		if task.DependsOn == nil {
			task.DependsOn = []string{}
		}
		tasks = append(tasks, &task)
	}
	return tasks
}

// ClaimPending atomically finds and claims a pending task with all dependencies met.
func (s *PostgresStore) ClaimPending() *Task {
	ctx := context.Background()

	// Get all pending tasks
	rows, err := s.pool.Query(ctx, `
		SELECT id, description, status, logs,
			COALESCE(pull_request_url, ''), COALESCE(pr_number, 0),
			depends_on, COALESCE(close_reason, ''),
			created_at, updated_at
		FROM tasks WHERE status = 'pending' ORDER BY created_at ASC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var pendingTasks []*Task
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID, &task.Description, &task.Status, &task.Logs,
			&task.PullRequestURL, &task.PRNumber, &task.DependsOn, &task.CloseReason,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			continue
		}
		// Ensure slices are never nil
		if task.Logs == nil {
			task.Logs = []string{}
		}
		if task.DependsOn == nil {
			task.DependsOn = []string{}
		}
		pendingTasks = append(pendingTasks, &task)
	}
	rows.Close()

	// Find a task with all dependencies met
	for _, task := range pendingTasks {
		if s.dependenciesMet(ctx, task) {
			// Try to claim it atomically
			result, err := s.pool.Exec(ctx, `
				UPDATE tasks SET status = 'running', updated_at = NOW()
				WHERE id = $1 AND status = 'pending'
			`, task.ID)
			if err != nil {
				continue
			}
			if result.RowsAffected() == 1 {
				task.Status = TaskStatusRunning
				task.UpdatedAt = time.Now()
				return task
			}
		}
	}
	return nil
}

// dependenciesMet checks if all dependencies are in merged or closed status.
func (s *PostgresStore) dependenciesMet(ctx context.Context, t *Task) bool {
	if len(t.DependsOn) == 0 {
		return true
	}

	for _, depID := range t.DependsOn {
		var status TaskStatus
		err := s.pool.QueryRow(ctx, "SELECT status FROM tasks WHERE id = $1", depID).Scan(&status)
		if err != nil {
			return false
		}
		if status != TaskStatusMerged && status != TaskStatusClosed {
			return false
		}
	}
	return true
}

// WaitForPending returns a channel that signals when a pending task might be available.
func (s *PostgresStore) WaitForPending() <-chan struct{} {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pendingCh
}

func (s *PostgresStore) notifyPending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}
}

// AppendLogs appends logs to a task.
func (s *PostgresStore) AppendLogs(id string, logs []string) bool {
	ctx := context.Background()
	result, err := s.pool.Exec(ctx, `
		UPDATE tasks SET logs = logs || $2, updated_at = NOW()
		WHERE id = $1
	`, id, logs)
	if err != nil {
		return false
	}
	return result.RowsAffected() == 1
}

// UpdateStatus updates a task's status.
func (s *PostgresStore) UpdateStatus(id string, status TaskStatus) bool {
	ctx := context.Background()
	result, err := s.pool.Exec(ctx, `
		UPDATE tasks SET status = $2, updated_at = NOW()
		WHERE id = $1
	`, id, status)
	if err != nil {
		return false
	}
	return result.RowsAffected() == 1
}

// SetPullRequest sets the PR URL and number for a task.
func (s *PostgresStore) SetPullRequest(id string, prURL string, prNumber int) bool {
	ctx := context.Background()
	result, err := s.pool.Exec(ctx, `
		UPDATE tasks SET pull_request_url = $2, pr_number = $3, status = 'review', updated_at = NOW()
		WHERE id = $1
	`, id, prURL, prNumber)
	if err != nil {
		return false
	}
	return result.RowsAffected() == 1
}

// GetTasksInReview returns all tasks in review status.
func (s *PostgresStore) GetTasksInReview() []*Task {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT id, description, status, logs,
			COALESCE(pull_request_url, ''), COALESCE(pr_number, 0),
			depends_on, COALESCE(close_reason, ''),
			created_at, updated_at
		FROM tasks WHERE status = 'review'
	`)
	if err != nil {
		return []*Task{}
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID, &task.Description, &task.Status, &task.Logs,
			&task.PullRequestURL, &task.PRNumber, &task.DependsOn, &task.CloseReason,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			continue
		}
		// Ensure slices are never nil
		if task.Logs == nil {
			task.Logs = []string{}
		}
		if task.DependsOn == nil {
			task.DependsOn = []string{}
		}
		tasks = append(tasks, &task)
	}
	return tasks
}

// CloseTask closes a task with an optional reason.
func (s *PostgresStore) CloseTask(id string, reason string) bool {
	ctx := context.Background()
	result, err := s.pool.Exec(ctx, `
		UPDATE tasks SET status = 'closed', close_reason = $2, updated_at = NOW()
		WHERE id = $1
	`, id, reason)
	if err != nil {
		return false
	}
	return result.RowsAffected() == 1
}
