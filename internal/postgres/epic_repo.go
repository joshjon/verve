package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joshjon/kit/errtag"

	"verve/internal/epic"
	"verve/internal/postgres/sqlc"
)

var _ epic.Repository = (*EpicRepository)(nil)

// EpicRepository implements epic.Repository using PostgreSQL.
type EpicRepository struct {
	db *sqlc.Queries
}

// NewEpicRepository creates a new EpicRepository backed by the given pgx pool.
func NewEpicRepository(pool *pgxpool.Pool) *EpicRepository {
	return &EpicRepository{
		db: sqlc.New(pool),
	}
}

func (r *EpicRepository) CreateEpic(ctx context.Context, e *epic.Epic) error {
	proposedJSON, _ := json.Marshal(e.ProposedTasks)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	err := r.db.CreateEpic(ctx, sqlc.CreateEpicParams{
		ID:             e.ID.String(),
		RepoID:         e.RepoID,
		Title:          e.Title,
		Description:    e.Description,
		Status:         string(e.Status),
		ProposedTasks:  proposedJSON,
		TaskIds:        e.TaskIDs,
		PlanningPrompt: prompt,
		SessionLog:     e.SessionLog,
		NotReady:       e.NotReady,
		CreatedAt:      pgTimestamptz(e.CreatedAt),
		UpdatedAt:      pgTimestamptz(e.UpdatedAt),
	})
	return tagEpicErr(err)
}

func (r *EpicRepository) ReadEpic(ctx context.Context, id epic.EpicID) (*epic.Epic, error) {
	row, err := r.db.ReadEpic(ctx, id.String())
	if err != nil {
		return nil, tagEpicErr(err)
	}
	return unmarshalEpic(row), nil
}

func (r *EpicRepository) ListEpics(ctx context.Context) ([]*epic.Epic, error) {
	rows, err := r.db.ListEpics(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) ListEpicsByRepo(ctx context.Context, repoID string) ([]*epic.Epic, error) {
	rows, err := r.db.ListEpicsByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) UpdateEpic(ctx context.Context, e *epic.Epic) error {
	proposedJSON, _ := json.Marshal(e.ProposedTasks)
	var prompt *string
	if e.PlanningPrompt != "" {
		prompt = &e.PlanningPrompt
	}
	return tagEpicErr(r.db.UpdateEpic(ctx, sqlc.UpdateEpicParams{
		ID:             e.ID.String(),
		Title:          e.Title,
		Description:    e.Description,
		Status:         string(e.Status),
		ProposedTasks:  proposedJSON,
		TaskIds:        e.TaskIDs,
		PlanningPrompt: prompt,
		SessionLog:     e.SessionLog,
		NotReady:       e.NotReady,
	}))
}

func (r *EpicRepository) UpdateEpicStatus(ctx context.Context, id epic.EpicID, status epic.Status) error {
	return tagEpicErr(r.db.UpdateEpicStatus(ctx, sqlc.UpdateEpicStatusParams{
		ID:     id.String(),
		Status: string(status),
	}))
}

func (r *EpicRepository) UpdateProposedTasks(ctx context.Context, id epic.EpicID, tasks []epic.ProposedTask) error {
	proposedJSON, _ := json.Marshal(tasks)
	return tagEpicErr(r.db.UpdateProposedTasks(ctx, sqlc.UpdateProposedTasksParams{
		ID:            id.String(),
		ProposedTasks: proposedJSON,
	}))
}

func (r *EpicRepository) SetTaskIDs(ctx context.Context, id epic.EpicID, taskIDs []string) error {
	return tagEpicErr(r.db.SetEpicTaskIDs(ctx, sqlc.SetEpicTaskIDsParams{
		ID:      id.String(),
		TaskIds: taskIDs,
	}))
}

func (r *EpicRepository) AppendSessionLog(ctx context.Context, id epic.EpicID, lines []string) error {
	return tagEpicErr(r.db.AppendSessionLog(ctx, sqlc.AppendSessionLogParams{
		ID:         id.String(),
		SessionLog: lines,
	}))
}

func (r *EpicRepository) DeleteEpic(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.DeleteEpic(ctx, id.String()))
}

func (r *EpicRepository) ListPlanningEpics(ctx context.Context) ([]*epic.Epic, error) {
	rows, err := r.db.ListPlanningEpics(ctx)
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func (r *EpicRepository) ClaimEpic(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ClaimEpic(ctx, id.String()))
}

func (r *EpicRepository) EpicHeartbeat(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.EpicHeartbeat(ctx, id.String()))
}

func (r *EpicRepository) SetEpicFeedback(ctx context.Context, id epic.EpicID, feedback, feedbackType string) error {
	return tagEpicErr(r.db.SetEpicFeedback(ctx, sqlc.SetEpicFeedbackParams{
		ID:           id.String(),
		Feedback:     &feedback,
		FeedbackType: &feedbackType,
	}))
}

func (r *EpicRepository) ClearEpicFeedback(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ClearEpicFeedback(ctx, id.String()))
}

func (r *EpicRepository) ReleaseEpicClaim(ctx context.Context, id epic.EpicID) error {
	return tagEpicErr(r.db.ReleaseEpicClaim(ctx, id.String()))
}

func (r *EpicRepository) ListStaleEpics(ctx context.Context, threshold time.Time) ([]*epic.Epic, error) {
	rows, err := r.db.ListStaleEpics(ctx, pgTimestamptz(threshold))
	if err != nil {
		return nil, err
	}
	return unmarshalEpicList(rows), nil
}

func unmarshalEpic(in *sqlc.Epic) *epic.Epic {
	e := &epic.Epic{
		ID:           epic.MustParseEpicID(in.ID),
		RepoID:       in.RepoID,
		Title:        in.Title,
		Description:  in.Description,
		Status:       epic.Status(in.Status),
		TaskIDs:      in.TaskIds,
		SessionLog:   in.SessionLog,
		NotReady:     in.NotReady,
		Feedback:     in.Feedback,
		FeedbackType: in.FeedbackType,
	}
	if in.PlanningPrompt != nil {
		e.PlanningPrompt = *in.PlanningPrompt
	}
	_ = json.Unmarshal(in.ProposedTasks, &e.ProposedTasks)
	if e.ProposedTasks == nil {
		e.ProposedTasks = []epic.ProposedTask{}
	}
	if e.TaskIDs == nil {
		e.TaskIDs = []string{}
	}
	if e.SessionLog == nil {
		e.SessionLog = []string{}
	}
	if in.CreatedAt.Valid {
		e.CreatedAt = in.CreatedAt.Time
	}
	if in.UpdatedAt.Valid {
		e.UpdatedAt = in.UpdatedAt.Time
	}
	if in.ClaimedAt.Valid {
		e.ClaimedAt = &in.ClaimedAt.Time
	}
	if in.LastHeartbeatAt.Valid {
		e.LastHeartbeatAt = &in.LastHeartbeatAt.Time
	}
	return e
}

func unmarshalEpicList(in []*sqlc.Epic) []*epic.Epic {
	out := make([]*epic.Epic, len(in))
	for i := range in {
		out[i] = unmarshalEpic(in[i])
	}
	return out
}

func tagEpicErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errtag.Tag[epic.ErrTagEpicNotFound](err)
	}
	if isPGErrCode(err, pgerrcode.UniqueViolation) {
		return errtag.Tag[epic.ErrTagEpicConflict](err)
	}
	return err
}
