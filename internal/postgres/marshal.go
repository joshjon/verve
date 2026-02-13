package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"verve/internal/postgres/sqlc"
	"verve/internal/task"
)

func unmarshalTask(in *sqlc.Task) *task.Task {
	t := &task.Task{
		ID:          task.MustParseTaskID(in.ID),
		Description: in.Description,
		Status:      task.Status(in.Status),
		DependsOn:   in.DependsOn,
	}
	if in.PullRequestUrl != nil {
		t.PullRequestURL = *in.PullRequestUrl
	}
	if in.PrNumber != nil {
		t.PRNumber = int(*in.PrNumber)
	}
	if in.CloseReason != nil {
		t.CloseReason = *in.CloseReason
	}
	if in.CreatedAt.Valid {
		t.CreatedAt = in.CreatedAt.Time
	}
	if in.UpdatedAt.Valid {
		t.UpdatedAt = in.UpdatedAt.Time
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	return t
}

func unmarshalTaskList(in []*sqlc.Task) []*task.Task {
	out := make([]*task.Task, len(in))
	for i := range in {
		out[i] = unmarshalTask(in[i])
	}
	return out
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func ptr[T any](v T) *T {
	return &v
}
