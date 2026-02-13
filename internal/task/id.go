package task

import (
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type taskPrefix struct{}

func (taskPrefix) Prefix() string { return "tsk" }

// TaskID is the unique identifier for a Task.
type TaskID struct {
	typeid.TypeID[taskPrefix]
}

// NewTaskID generates a new unique TaskID.
func NewTaskID() TaskID {
	return id.New[TaskID]()
}

// ParseTaskID parses a string into a TaskID.
func ParseTaskID(s string) (TaskID, error) {
	return id.Parse[TaskID](s)
}

// MustParseTaskID parses a string into a TaskID, panicking on failure.
func MustParseTaskID(s string) TaskID {
	return id.MustParse[TaskID](s)
}
