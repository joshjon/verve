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
	ListPendingTasks(ctx context.Context) ([]*Task, error)
	AppendTaskLogs(ctx context.Context, id TaskID, logs []string) error
	UpdateTaskStatus(ctx context.Context, id TaskID, status Status) error
	SetTaskPullRequest(ctx context.Context, id TaskID, prURL string, prNumber int) error
	ListTasksInReview(ctx context.Context) ([]*Task, error)
	CloseTask(ctx context.Context, id TaskID, reason string) error
	TaskExists(ctx context.Context, id TaskID) (bool, error)
	ReadTaskStatus(ctx context.Context, id TaskID) (Status, error)
	ClaimTask(ctx context.Context, id TaskID) (bool, error)
}
