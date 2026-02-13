package repo

import "context"

// Repository is the interface for performing CRUD operations on Repos.
type Repository interface {
	CreateRepo(ctx context.Context, repo *Repo) error
	ReadRepo(ctx context.Context, id RepoID) (*Repo, error)
	ReadRepoByFullName(ctx context.Context, fullName string) (*Repo, error)
	ListRepos(ctx context.Context) ([]*Repo, error)
	DeleteRepo(ctx context.Context, id RepoID) error
}
