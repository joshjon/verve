package epic

import (
	"context"
	"time"

	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

// taskPrefix is needed for generating task IDs from the epic package.
type taskPrefix struct{}

func (taskPrefix) Prefix() string { return "tsk" }

// taskID is the task ID type used for generating IDs.
type taskID struct {
	typeid.TypeID[taskPrefix]
}

// NewTaskIDString generates a new unique task ID string.
func NewTaskIDString() string {
	return id.New[taskID]().String()
}

// TaskCreateFunc is a function that creates a task and returns its ID.
type TaskCreateFunc func(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool) (string, error)

// TaskCreatorFunc adapts a function to the TaskCreator interface.
type TaskCreatorFunc struct {
	fn TaskCreateFunc
}

// NewTaskCreatorFunc creates a TaskCreator from a function.
func NewTaskCreatorFunc(fn TaskCreateFunc) *TaskCreatorFunc {
	return &TaskCreatorFunc{fn: fn}
}

func (f *TaskCreatorFunc) CreateTaskFromEpic(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool) (string, error) {
	return f.fn(ctx, repoID, title, description, dependsOn, acceptanceCriteria, epicID, ready)
}

// Now is a helper for generating timestamps.
func Now() time.Time {
	return time.Now()
}
