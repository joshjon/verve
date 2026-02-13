package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/errtag"
	"github.com/joshjon/kit/tx"

	"verve/internal/postgres/sqlc"
	"verve/internal/task"
)

var _ task.Repository = (*TaskRepository)(nil)

// TaskRepository implements task.Repository using PostgreSQL.
type TaskRepository struct {
	db   *sqlc.Queries
	txer *tx.PGXRepositoryTxer[task.Repository]
}

// NewTaskRepository creates a new TaskRepository backed by the given pgx pool.
func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{
		db: sqlc.New(pool),
		txer: tx.NewPGXRepositoryTxer(pool, tx.PGXRepositoryTxerConfig[task.Repository]{
			Timeout: tx.DefaultTimeout,
			WithTxFunc: func(repo task.Repository, txer *tx.PGXRepositoryTxer[task.Repository], pgxTx pgx.Tx) task.Repository {
				cpy := *repo.(*TaskRepository)
				cpy.db = cpy.db.WithTx(pgxTx)
				cpy.txer = txer
				return task.Repository(&cpy)
			},
		}),
	}
}

func (r *TaskRepository) CreateTask(ctx context.Context, t *task.Task) error {
	err := r.db.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:          t.ID.String(),
		Description: t.Description,
		Status:      sqlc.TaskStatus(t.Status),
		Logs:        t.Logs,
		DependsOn:   t.DependsOn,
		CreatedAt:   pgTimestamptz(t.CreatedAt),
		UpdatedAt:   pgTimestamptz(t.UpdatedAt),
	})
	return tagTaskErr(err)
}

func (r *TaskRepository) ReadTask(ctx context.Context, id task.TaskID) (*task.Task, error) {
	row, err := r.db.ReadTask(ctx, id.String())
	if err != nil {
		return nil, tagTaskErr(err)
	}
	return unmarshalTask(row), nil
}

func (r *TaskRepository) ListTasks(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) ListPendingTasks(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListPendingTasks(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) AppendTaskLogs(ctx context.Context, id task.TaskID, logs []string) error {
	return tagTaskErr(r.db.AppendTaskLogs(ctx, sqlc.AppendTaskLogsParams{
		ID:   id.String(),
		Logs: logs,
	}))
}

func (r *TaskRepository) UpdateTaskStatus(ctx context.Context, id task.TaskID, status task.Status) error {
	return tagTaskErr(r.db.UpdateTaskStatus(ctx, sqlc.UpdateTaskStatusParams{
		ID:     id.String(),
		Status: sqlc.TaskStatus(status),
	}))
}

func (r *TaskRepository) SetTaskPullRequest(ctx context.Context, id task.TaskID, prURL string, prNumber int) error {
	return tagTaskErr(r.db.SetTaskPullRequest(ctx, sqlc.SetTaskPullRequestParams{
		ID:             id.String(),
		PullRequestUrl: &prURL,
		PrNumber:       ptr(int32(prNumber)),
	}))
}

func (r *TaskRepository) ListTasksInReview(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReview(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) CloseTask(ctx context.Context, id task.TaskID, reason string) error {
	return tagTaskErr(r.db.CloseTask(ctx, sqlc.CloseTaskParams{
		ID:          id.String(),
		CloseReason: &reason,
	}))
}

func (r *TaskRepository) TaskExists(ctx context.Context, id task.TaskID) (bool, error) {
	exists, err := r.db.TaskExists(ctx, id.String())
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *TaskRepository) ReadTaskStatus(ctx context.Context, id task.TaskID) (task.Status, error) {
	status, err := r.db.ReadTaskStatus(ctx, id.String())
	if err != nil {
		return "", tagTaskErr(err)
	}
	return task.Status(status), nil
}

func (r *TaskRepository) ClaimTask(ctx context.Context, id task.TaskID) (bool, error) {
	rows, err := r.db.ClaimTask(ctx, id.String())
	return rows > 0, err
}

func (r *TaskRepository) WithTx(txn tx.Tx) task.Repository {
	return r.txer.WithTx(r, txn)
}

func (r *TaskRepository) BeginTxFunc(ctx context.Context, fn func(ctx context.Context, txn tx.Tx, repo task.Repository) error) error {
	return r.txer.BeginTxFunc(ctx, r, fn)
}

func tagTaskErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errtag.Tag[task.ErrTagTaskNotFound](err)
	}
	if isPGErrCode(err, pgerrcode.UniqueViolation) {
		return errtag.Tag[task.ErrTagTaskConflict](err)
	}
	return tx.TagPGXTimeoutErr(err)
}

func isPGErrCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
