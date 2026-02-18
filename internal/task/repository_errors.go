package task

import (
	"errors"

	"github.com/joshjon/kit/errtag"
)

// ErrTaskNotPending is returned when an update is attempted on a task that is
// no longer in pending status.
var ErrTaskNotPending = errtag.Tag[ErrTagTaskNotPending](
	errors.New("task is no longer pending"),
)

// ErrTagTaskNotPending indicates an operation was rejected because the task
// is not in pending status.
type ErrTagTaskNotPending struct{ errtag.Conflict }

func (ErrTagTaskNotPending) Msg() string { return "task is no longer pending" }

func (e ErrTagTaskNotPending) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}

// ErrTagTaskNotFound indicates a task was not found.
type ErrTagTaskNotFound struct{ errtag.NotFound }

func (ErrTagTaskNotFound) Msg() string { return "Task not found" }

func (e ErrTagTaskNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

// ErrTagTaskConflict indicates a task conflict (e.g. duplicate ID).
type ErrTagTaskConflict struct{ errtag.Conflict }

func (ErrTagTaskConflict) Msg() string { return "Task conflict" }

func (e ErrTagTaskConflict) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}
