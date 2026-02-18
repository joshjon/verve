package epic

import "github.com/joshjon/kit/errtag"

// ErrTagEpicNotFound indicates an epic was not found.
type ErrTagEpicNotFound struct{ errtag.NotFound }

func (ErrTagEpicNotFound) Msg() string { return "Epic not found" }

func (e ErrTagEpicNotFound) Unwrap() error {
	return errtag.Tag[errtag.NotFound](e.Cause())
}

// ErrTagEpicConflict indicates an epic conflict (e.g. duplicate ID).
type ErrTagEpicConflict struct{ errtag.Conflict }

func (ErrTagEpicConflict) Msg() string { return "Epic conflict" }

func (e ErrTagEpicConflict) Unwrap() error {
	return errtag.Tag[errtag.Conflict](e.Cause())
}
