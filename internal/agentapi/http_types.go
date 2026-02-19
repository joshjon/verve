package agentapi

import (
	"verve/internal/epic"
	"verve/internal/task"
)

// PollResponse is the discriminated union returned by the unified poll endpoint.
type PollResponse struct {
	Type string `json:"type"` // "task" or "epic"

	// Task fields (present when Type == "task")
	Task *task.Task `json:"task,omitempty"`

	// Epic fields (present when Type == "epic")
	Epic *epic.Epic `json:"epic,omitempty"`

	// Common fields
	GitHubToken  string `json:"github_token,omitempty"`
	RepoFullName string `json:"repo_full_name"`
}

// ProposeTasksRequest is the request body for agent proposing tasks.
type ProposeTasksRequest struct {
	Tasks []epic.ProposedTask `json:"tasks"`
}

// SessionLogRequest is the request body for appending session log entries.
type SessionLogRequest struct {
	Lines []string `json:"lines"`
}

// FeedbackResponse is returned from the poll-feedback endpoint.
type FeedbackResponse struct {
	Type     string `json:"type"` // "feedback", "confirmed", "closed", "timeout"
	Feedback string `json:"feedback,omitempty"`
}
