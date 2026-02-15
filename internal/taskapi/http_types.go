package taskapi

import "verve/internal/task"

// AddRepoRequest is the request body for adding a repo.
type AddRepoRequest struct {
	FullName string `json:"full_name"`
}

// CreateTaskRequest is the request body for creating a task.
type CreateTaskRequest struct {
	Description        string   `json:"description"`
	DependsOn          []string `json:"depends_on,omitempty"`
	AcceptanceCriteria string   `json:"acceptance_criteria,omitempty"`
	MaxCostUSD         float64  `json:"max_cost_usd,omitempty"`
}

// LogsRequest is the request body for appending logs.
type LogsRequest struct {
	Logs []string `json:"logs"`
}

// CompleteRequest is the request body for completing a task.
type CompleteRequest struct {
	Success        bool    `json:"success"`
	Error          string  `json:"error,omitempty"`
	PullRequestURL string  `json:"pull_request_url,omitempty"`
	PRNumber       int     `json:"pr_number,omitempty"`
	AgentStatus    string  `json:"agent_status,omitempty"`
	CostUSD        float64 `json:"cost_usd,omitempty"`
	PrereqFailed   string  `json:"prereq_failed,omitempty"`
}

// CloseRequest is the request body for closing a task.
type CloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// PollTaskResponse wraps a claimed task with the credentials and repo info
// needed by the worker to execute it. The GitHub token is included so that
// workers don't need their own token configuration.
type PollTaskResponse struct {
	Task         *task.Task `json:"task"`
	GitHubToken  string     `json:"github_token"`
	RepoFullName string     `json:"repo_full_name"`
}
