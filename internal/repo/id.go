package repo

import (
	"github.com/joshjon/kit/id"
	"go.jetify.com/typeid"
)

type repoPrefix struct{}

func (repoPrefix) Prefix() string { return "repo" }

// RepoID is the unique identifier for a Repo.
type RepoID struct {
	typeid.TypeID[repoPrefix]
}

// NewRepoID generates a new unique RepoID.
func NewRepoID() RepoID {
	return id.New[RepoID]()
}

// ParseRepoID parses a string into a RepoID.
func ParseRepoID(s string) (RepoID, error) {
	return id.Parse[RepoID](s)
}

// MustParseRepoID parses a string into a RepoID, panicking on failure.
func MustParseRepoID(s string) RepoID {
	return id.MustParse[RepoID](s)
}
