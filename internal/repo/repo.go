package repo

import (
	"fmt"
	"strings"
	"time"
)

// Repo represents a GitHub repository added to Verve.
type Repo struct {
	ID        RepoID    `json:"id"`
	Owner     string    `json:"owner"`
	Name      string    `json:"name"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
}

// NewRepo creates a new Repo from a full name (e.g., "owner/repo").
func NewRepo(fullName string) (*Repo, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repo full name %q: expected owner/name", fullName)
	}
	return &Repo{
		ID:        NewRepoID(),
		Owner:     parts[0],
		Name:      parts[1],
		FullName:  fullName,
		CreatedAt: time.Now(),
	}, nil
}
