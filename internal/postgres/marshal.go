package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"verve/internal/postgres/sqlc"
	"verve/internal/task"
)

func unmarshalTask(in *sqlc.Task) *task.Task {
	t := &task.Task{
		ID:                 task.MustParseTaskID(in.ID),
		RepoID:             in.RepoID,
		Title:              in.Title,
		Description:        in.Description,
		Status:             task.Status(in.Status),
		DependsOn:          in.DependsOn,
		AcceptanceCriteria: in.AcceptanceCriteriaList,
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
	t.Attempt = int(in.Attempt)
	t.AttemptBase = int(in.AttemptBase)
	t.MaxAttempts = int(in.MaxAttempts)
	if in.RetryReason != nil {
		t.RetryReason = *in.RetryReason
	}
	if in.AgentStatus != nil {
		t.AgentStatus = *in.AgentStatus
	}
	if in.RetryContext != nil {
		t.RetryContext = *in.RetryContext
	}
	t.ConsecutiveFailures = int(in.ConsecutiveFailures)
	t.CostUSD = in.CostUsd
	if in.MaxCostUsd != nil {
		t.MaxCostUSD = *in.MaxCostUsd
	}
	t.SkipPR = in.SkipPr
	t.Ready = in.Ready
	if in.Model != nil {
		t.Model = *in.Model
	}
	if in.BranchName != nil {
		t.BranchName = *in.BranchName
	}
	if in.StartedAt.Valid {
		t.StartedAt = &in.StartedAt.Time
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
	if t.AcceptanceCriteria == nil {
		t.AcceptanceCriteria = []string{}
	}
	t.ComputeDuration()
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
