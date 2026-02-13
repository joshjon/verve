package task

import "github.com/joshjon/kit/errtag"

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
