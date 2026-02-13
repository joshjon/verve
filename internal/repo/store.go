package repo

import (
	"context"
	"fmt"
)

// TaskChecker checks whether tasks exist for a given repo.
type TaskChecker interface {
	HasTasksForRepo(ctx context.Context, repoID string) (bool, error)
}

// Store wraps a Repository and adds application-level concerns.
type Store struct {
	repo         Repository
	taskChecker  TaskChecker
}

// NewStore creates a new Store backed by the given Repository.
func NewStore(repo Repository, taskChecker TaskChecker) *Store {
	return &Store{repo: repo, taskChecker: taskChecker}
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

// DeleteRepo deletes a repo if it has no tasks.
func (s *Store) DeleteRepo(ctx context.Context, id RepoID) error {
	has, err := s.taskChecker.HasTasksForRepo(ctx, id.String())
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("cannot delete repo: tasks still exist")
	}
	return s.repo.DeleteRepo(ctx, id)
}
