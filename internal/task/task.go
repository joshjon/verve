package task

import "time"

// Status represents the lifecycle state of a Task.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusReview  Status = "review" // PR created, awaiting review/merge
	StatusMerged  Status = "merged" // PR has been merged
	StatusClosed  Status = "closed" // Manually closed by user
	StatusFailed  Status = "failed"
)

// Task represents a unit of work dispatched to an AI coding agent.
type Task struct {
	ID                  TaskID    `json:"id"`
	RepoID              string    `json:"repo_id"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	Status              Status    `json:"status"`
	Logs                []string  `json:"logs"`
	PullRequestURL      string    `json:"pull_request_url,omitempty"`
	PRNumber            int       `json:"pr_number,omitempty"`
	DependsOn           []string  `json:"depends_on,omitempty"`
	CloseReason         string    `json:"close_reason,omitempty"`
	Attempt             int       `json:"attempt"`
	MaxAttempts         int       `json:"max_attempts"`
	RetryReason         string    `json:"retry_reason,omitempty"`
	AcceptanceCriteria  []string  `json:"acceptance_criteria"`
	AgentStatus         string    `json:"agent_status,omitempty"`
	RetryContext        string    `json:"retry_context,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	CostUSD             float64   `json:"cost_usd"`
	MaxCostUSD          float64   `json:"max_cost_usd,omitempty"`
	SkipPR              bool      `json:"skip_pr"`
	Model               string    `json:"model,omitempty"`
	BranchName          string    `json:"branch_name,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// NewTask creates a new Task with a generated TaskID and pending status.
func NewTask(repoID, title, description string, dependsOn []string, acceptanceCriteria []string, maxCostUSD float64, skipPR bool, model string) *Task {
	now := time.Now()
	if dependsOn == nil {
		dependsOn = []string{}
	}
	if acceptanceCriteria == nil {
		acceptanceCriteria = []string{}
	}
	return &Task{
		ID:                 NewTaskID(),
		RepoID:             repoID,
		Title:              title,
		Description:        description,
		Status:             StatusPending,
		DependsOn:          dependsOn,
		Attempt:            1,
		MaxAttempts:        5,
		AcceptanceCriteria: acceptanceCriteria,
		MaxCostUSD:         maxCostUSD,
		SkipPR:             skipPR,
		Model:              model,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
