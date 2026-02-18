package epic

import "context"

// Repository is the interface for performing CRUD operations on Epics.
type Repository interface {
	CreateEpic(ctx context.Context, epic *Epic) error
	ReadEpic(ctx context.Context, id EpicID) (*Epic, error)
	ListEpics(ctx context.Context) ([]*Epic, error)
	ListEpicsByRepo(ctx context.Context, repoID string) ([]*Epic, error)
	UpdateEpic(ctx context.Context, epic *Epic) error
	UpdateEpicStatus(ctx context.Context, id EpicID, status Status) error
	UpdateProposedTasks(ctx context.Context, id EpicID, tasks []ProposedTask) error
	SetTaskIDs(ctx context.Context, id EpicID, taskIDs []string) error
	AppendSessionLog(ctx context.Context, id EpicID, lines []string) error
	DeleteEpic(ctx context.Context, id EpicID) error
}
