package taskapi

import (
	"verve/internal/github"
	"verve/internal/task"
)

// AddRepoRequest is the request body for adding a repo.
type AddRepoRequest struct {
	FullName string `json:"full_name"`
}

// CreateTaskRequest is the request body for creating a task.
type CreateTaskRequest struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          []string `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	MaxCostUSD         float64  `json:"max_cost_usd,omitempty"`
	SkipPR             bool     `json:"skip_pr,omitempty"`
	Model              string   `json:"model,omitempty"`
	NotReady           bool     `json:"not_ready,omitempty"`
}

// UpdateTaskRequest is the request body for updating a pending task.
// All fields are optional — only provided fields are updated.
type UpdateTaskRequest struct {
	Title              *string  `json:"title,omitempty"`
	Description        *string  `json:"description,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	MaxCostUSD         *float64 `json:"max_cost_usd,omitempty"`
	SkipPR             *bool    `json:"skip_pr,omitempty"`
	Model              *string  `json:"model,omitempty"`
	NotReady           *bool    `json:"not_ready,omitempty"`
}

// SetReadyRequest is the request body for toggling a task's ready state.
type SetReadyRequest struct {
	Ready bool `json:"ready"`
}

// DefaultModelRequest is the request body for setting the default model.
type DefaultModelRequest struct {
	Model string `json:"model"`
}

// DefaultModelResponse is the response for getting the default model.
type DefaultModelResponse struct {
	Model string `json:"model"`
}

// LogsRequest is the request body for appending logs.
type LogsRequest struct {
	Logs    []string `json:"logs"`
	Attempt int      `json:"attempt,omitempty"`
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
	BranchName     string  `json:"branch_name,omitempty"`
	NoChanges      bool    `json:"no_changes,omitempty"`
	Retryable      bool    `json:"retryable,omitempty"`
}

// CloseRequest is the request body for closing a task.
type CloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RetryTaskRequest is the request body for retrying a failed task.
type RetryTaskRequest struct {
	Instructions string `json:"instructions,omitempty"`
}

// FeedbackRequest is the request body for providing feedback on a task in review.
// This re-prompts the agent to iterate on its solution using the user's feedback.
type FeedbackRequest struct {
	Feedback string `json:"feedback"`
}

// StartOverRequest is the request body for starting a task over from scratch.
// All fields are optional — only provided fields are updated; others keep their current values.
type StartOverRequest struct {
	Title              *string  `json:"title,omitempty"`
	Description        *string  `json:"description,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

// RemoveDependencyRequest is the request body for removing a dependency from a task.
type RemoveDependencyRequest struct {
	DependsOn string `json:"depends_on"`
}

// BulkDeleteTasksRequest is the request body for bulk-deleting tasks.
type BulkDeleteTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

// CheckStatusResponse is the response body for the task check status endpoint.
type CheckStatusResponse struct {
	Status           string                  `json:"status"`                       // "pending", "success", "failure", "error"
	Summary          string                  `json:"summary,omitempty"`
	FailedNames      []string                `json:"failed_names,omitempty"`
	CheckRunsSkipped bool                    `json:"check_runs_skipped,omitempty"` // True when GitHub Actions checks couldn't be read (fine-grained PAT)
	Checks           []github.IndividualCheck `json:"checks,omitempty"`
}

// DiffResponse is the response body for the task diff endpoint.
type DiffResponse struct {
	Diff string `json:"diff"`
}

// PollTaskResponse wraps a claimed task with the credentials and repo info
// needed by the worker to execute it. The GitHub token is included so that
// workers don't need their own token configuration.
type PollTaskResponse struct {
	Task         *task.Task `json:"task"`
	GitHubToken  string     `json:"github_token"`
	RepoFullName string     `json:"repo_full_name"`
}

// SaveGitHubTokenRequest is the request body for saving a GitHub token.
type SaveGitHubTokenRequest struct {
	Token string `json:"token"`
}

// GitHubTokenStatusResponse indicates whether a GitHub token is configured.
type GitHubTokenStatusResponse struct {
	Configured  bool `json:"configured"`
	FineGrained bool `json:"fine_grained,omitempty"`
}
