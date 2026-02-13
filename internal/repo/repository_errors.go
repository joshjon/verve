package repo

import "github.com/joshjon/kit/errtag"

// ErrTagRepoNotFound tags errors for repo-not-found cases (HTTP 404).
type ErrTagRepoNotFound = errtag.NotFound

// ErrTagRepoConflict tags errors for repo-conflict cases (HTTP 409).
type ErrTagRepoConflict = errtag.Conflict
