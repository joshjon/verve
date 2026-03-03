package repo

import "context"

// Store wraps a Repository and adds application-level concerns.
type Store struct {
	repo Repository
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository) *Store {
	return &Store{repo: repo}
}

// CreateRepo creates a new repo.
func (s *Store) CreateRepo(ctx context.Context, repo *Repo) error {
	return s.repo.CreateRepo(ctx, repo)
}

// ReadRepo reads a repo by ID.
func (s *Store) ReadRepo(ctx context.Context, id RepoID) (*Repo, error) {
	return s.repo.ReadRepo(ctx, id)
}

// ReadRepoByFullName reads a repo by its full name (owner/name).
func (s *Store) ReadRepoByFullName(ctx context.Context, fullName string) (*Repo, error) {
	return s.repo.ReadRepoByFullName(ctx, fullName)
}

// ListRepos returns all repos.
func (s *Store) ListRepos(ctx context.Context) ([]*Repo, error) {
	return s.repo.ListRepos(ctx)
}

// DeleteRepo deletes a repo and cascade-deletes all associated epics, tasks,
// and task logs via ON DELETE CASCADE constraints in the database.
func (s *Store) DeleteRepo(ctx context.Context, id RepoID) error {
	return s.repo.DeleteRepo(ctx, id)
}
