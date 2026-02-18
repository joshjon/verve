package task

import (
	"context"

	"github.com/joshjon/kit/tx"
)

// Repository is the interface for performing CRUD operations on Tasks.
//
// Implementations must handle all database-specific concerns (error mapping,
// marshalling) and return domain types.
type Repository interface {
	TaskRepository
	tx.Repository[Repository]
}

// TaskRepository defines the data access methods for Tasks.
type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	ReadTask(ctx context.Context, id TaskID) (*Task, error)
	ListTasks(ctx context.Context) ([]*Task, error)
	ListTasksByRepo(ctx context.Context, repoID string) ([]*Task, error)
	ListPendingTasks(ctx context.Context) ([]*Task, error)
	ListPendingTasksByRepos(ctx context.Context, repoIDs []string) ([]*Task, error)
	AppendTaskLogs(ctx context.Context, id TaskID, attempt int, logs []string) error
	ReadTaskLogs(ctx context.Context, id TaskID) ([]string, error)
	StreamTaskLogs(ctx context.Context, id TaskID, fn func(attempt int, lines []string) error) error
	UpdateTaskStatus(ctx context.Context, id TaskID, status Status) error
	SetTaskPullRequest(ctx context.Context, id TaskID, prURL string, prNumber int) error
	ListTasksInReview(ctx context.Context) ([]*Task, error)
	ListTasksInReviewByRepo(ctx context.Context, repoID string) ([]*Task, error)
	CloseTask(ctx context.Context, id TaskID, reason string) error
	TaskExists(ctx context.Context, id TaskID) (bool, error)
	ReadTaskStatus(ctx context.Context, id TaskID) (Status, error)
	ClaimTask(ctx context.Context, id TaskID) (bool, error)
	HasTasksForRepo(ctx context.Context, repoID string) (bool, error)
	// RetryTask atomically transitions a task from review → pending, increments
	// attempt, and records the retry reason. Returns false if the task was not
	// in review status (already retried or status changed).
	RetryTask(ctx context.Context, id TaskID, reason string) (bool, error)
	SetAgentStatus(ctx context.Context, id TaskID, status string) error
	SetRetryContext(ctx context.Context, id TaskID, retryCtx string) error
	AddCost(ctx context.Context, id TaskID, costUSD float64) error
	SetConsecutiveFailures(ctx context.Context, id TaskID, count int) error
	SetCloseReason(ctx context.Context, id TaskID, reason string) error
	SetBranchName(ctx context.Context, id TaskID, branchName string) error
	ListTasksInReviewNoPR(ctx context.Context) ([]*Task, error)
	ManualRetryTask(ctx context.Context, id TaskID, instructions string) (bool, error)
	// FeedbackRetryTask transitions a task from review → pending and records
	// the user's feedback. Unlike ManualRetryTask, it preserves the existing
	// PR/branch so the agent pushes fixes to the same branch. The attempt
	// counter is reset to 1 so that subsequent automated failure retries get
	// a fresh retry budget — failures after user-requested changes are
	// caused by those changes, not by the original code.
	FeedbackRetryTask(ctx context.Context, id TaskID, feedback string) (bool, error)
	DeleteTaskLogs(ctx context.Context, id TaskID) error
	RemoveDependency(ctx context.Context, id TaskID, depID string) error
	SetReady(ctx context.Context, id TaskID, ready bool) error
}
