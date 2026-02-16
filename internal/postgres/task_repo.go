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
	dbtx sqlc.DBTX
	db   *sqlc.Queries
	txer *tx.PGXRepositoryTxer[task.Repository]
}

// NewTaskRepository creates a new TaskRepository backed by the given pgx pool.
func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{
		dbtx: pool,
		db:   sqlc.New(pool),
		txer: tx.NewPGXRepositoryTxer(pool, tx.PGXRepositoryTxerConfig[task.Repository]{
			Timeout: tx.DefaultTimeout,
			WithTxFunc: func(repo task.Repository, txer *tx.PGXRepositoryTxer[task.Repository], pgxTx pgx.Tx) task.Repository {
				cpy := *repo.(*TaskRepository)
				cpy.dbtx = pgxTx
				cpy.db = cpy.db.WithTx(pgxTx)
				cpy.txer = txer
				return task.Repository(&cpy)
			},
		}),
	}
}

func (r *TaskRepository) CreateTask(ctx context.Context, t *task.Task) error {
	var maxCostUSD *float64
	if t.MaxCostUSD > 0 {
		maxCostUSD = &t.MaxCostUSD
	}
	var model *string
	if t.Model != "" {
		model = &t.Model
	}
	err := r.db.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:                    t.ID.String(),
		RepoID:                t.RepoID,
		Title:                 t.Title,
		Description:           t.Description,
		Status:                sqlc.TaskStatus(t.Status),
		DependsOn:             t.DependsOn,
		Attempt:               int32(t.Attempt),
		MaxAttempts:           int32(t.MaxAttempts),
		AcceptanceCriteriaList: t.AcceptanceCriteria,
		MaxCostUsd:            maxCostUSD,
		SkipPr:                t.SkipPR,
		Model:                 model,
		CreatedAt:             pgTimestamptz(t.CreatedAt),
		UpdatedAt:             pgTimestamptz(t.UpdatedAt),
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

func (r *TaskRepository) ListTasksByRepo(ctx context.Context, repoID string) ([]*task.Task, error) {
	rows, err := r.db.ListTasksByRepo(ctx, repoID)
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

func (r *TaskRepository) ListPendingTasksByRepos(ctx context.Context, repoIDs []string) ([]*task.Task, error) {
	rows, err := r.db.ListPendingTasksByRepos(ctx, repoIDs)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) AppendTaskLogs(ctx context.Context, id task.TaskID, attempt int, logs []string) error {
	return tagTaskErr(r.db.AppendTaskLogs(ctx, sqlc.AppendTaskLogsParams{
		ID:      id.String(),
		Attempt: int32(attempt),
		Lines:   logs,
	}))
}

func (r *TaskRepository) ReadTaskLogs(ctx context.Context, id task.TaskID) ([]string, error) {
	batches, err := r.db.ReadTaskLogs(ctx, id.String())
	if err != nil {
		return nil, tagTaskErr(err)
	}
	var logs []string
	for _, batch := range batches {
		logs = append(logs, batch.Lines...)
	}
	if logs == nil {
		logs = []string{}
	}
	return logs, nil
}

func (r *TaskRepository) StreamTaskLogs(ctx context.Context, id task.TaskID, fn func(attempt int, lines []string) error) error {
	rows, err := r.dbtx.Query(ctx, "SELECT attempt, lines FROM task_log WHERE task_id = $1 ORDER BY id", id.String())
	if err != nil {
		return tagTaskErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var attempt int
		var lines []string
		if err := rows.Scan(&attempt, &lines); err != nil {
			return err
		}
		if err := fn(attempt, lines); err != nil {
			return err
		}
	}
	return rows.Err()
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

func (r *TaskRepository) ListTasksInReviewByRepo(ctx context.Context, repoID string) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReviewByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
}

func (r *TaskRepository) HasTasksForRepo(ctx context.Context, repoID string) (bool, error) {
	return r.db.HasTasksForRepo(ctx, repoID)
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

func (r *TaskRepository) RetryTask(ctx context.Context, id task.TaskID, reason string) (bool, error) {
	rows, err := r.db.RetryTask(ctx, sqlc.RetryTaskParams{
		ID:          id.String(),
		RetryReason: &reason,
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) SetAgentStatus(ctx context.Context, id task.TaskID, status string) error {
	return tagTaskErr(r.db.SetAgentStatus(ctx, sqlc.SetAgentStatusParams{
		ID:          id.String(),
		AgentStatus: &status,
	}))
}

func (r *TaskRepository) SetRetryContext(ctx context.Context, id task.TaskID, retryCtx string) error {
	return tagTaskErr(r.db.SetRetryContext(ctx, sqlc.SetRetryContextParams{
		ID:           id.String(),
		RetryContext: &retryCtx,
	}))
}

func (r *TaskRepository) AddCost(ctx context.Context, id task.TaskID, costUSD float64) error {
	return tagTaskErr(r.db.AddTaskCost(ctx, sqlc.AddTaskCostParams{
		ID:      id.String(),
		CostUsd: costUSD,
	}))
}

func (r *TaskRepository) SetConsecutiveFailures(ctx context.Context, id task.TaskID, count int) error {
	return tagTaskErr(r.db.SetConsecutiveFailures(ctx, sqlc.SetConsecutiveFailuresParams{
		ID:                  id.String(),
		ConsecutiveFailures: int32(count),
	}))
}

func (r *TaskRepository) SetCloseReason(ctx context.Context, id task.TaskID, reason string) error {
	return tagTaskErr(r.db.SetCloseReason(ctx, sqlc.SetCloseReasonParams{
		ID:          id.String(),
		CloseReason: &reason,
	}))
}

func (r *TaskRepository) SetBranchName(ctx context.Context, id task.TaskID, branchName string) error {
	return tagTaskErr(r.db.SetBranchName(ctx, sqlc.SetBranchNameParams{
		ID:         id.String(),
		BranchName: &branchName,
	}))
}

func (r *TaskRepository) ManualRetryTask(ctx context.Context, id task.TaskID, instructions string) (bool, error) {
	var reason *string
	if instructions != "" {
		reason = &instructions
	}
	rows, err := r.db.ManualRetryTask(ctx, sqlc.ManualRetryTaskParams{
		ID:          id.String(),
		RetryReason: reason,
	})
	return rows > 0, tagTaskErr(err)
}

func (r *TaskRepository) DeleteTaskLogs(ctx context.Context, id task.TaskID) error {
	return tagTaskErr(r.db.DeleteTaskLogs(ctx, id.String()))
}

func (r *TaskRepository) ListTasksInReviewNoPR(ctx context.Context) ([]*task.Task, error) {
	rows, err := r.db.ListTasksInReviewNoPR(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalTaskList(rows), nil
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
