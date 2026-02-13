package sqlite

import (
	"encoding/json"

	"verve/internal/sqlite/sqlc"
	"verve/internal/task"
)

func unmarshalTask(in *sqlc.Task) *task.Task {
	t := &task.Task{
		ID:          task.MustParseTaskID(in.ID),
		Description: in.Description,
		Status:      task.Status(in.Status),
		Logs:        unmarshalJSONStrings(in.Logs),
		DependsOn:   unmarshalJSONStrings(in.DependsOn),
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
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
	return t
}

func unmarshalTaskList(in []*sqlc.Task) []*task.Task {
	out := make([]*task.Task, len(in))
	for i := range in {
		out[i] = unmarshalTask(in[i])
	}
	return out
}

func marshalJSONStrings(ss []string) string {
	if ss == nil {
		ss = []string{}
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func unmarshalJSONStrings(s string) []string {
	var ss []string
	_ = json.Unmarshal([]byte(s), &ss)
	if ss == nil {
		ss = []string{}
	}
	return ss
}

func ptr[T any](v T) *T {
	return &v
}
