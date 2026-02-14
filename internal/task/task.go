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
	ID             TaskID    `json:"id"`
	RepoID         string    `json:"repo_id"`
	Description    string    `json:"description"`
	Status         Status    `json:"status"`
	Logs           []string  `json:"logs"`
	PullRequestURL string    `json:"pull_request_url,omitempty"`
	PRNumber       int       `json:"pr_number,omitempty"`
	DependsOn      []string  `json:"depends_on,omitempty"`
	CloseReason    string    `json:"close_reason,omitempty"`
	Attempt        int       `json:"attempt"`
	MaxAttempts    int       `json:"max_attempts"`
	RetryReason    string    `json:"retry_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewTask creates a new Task with a generated TaskID and pending status.
func NewTask(repoID, description string, dependsOn []string) *Task {
	now := time.Now()
	if dependsOn == nil {
		dependsOn = []string{}
	}
	return &Task{
		ID:          NewTaskID(),
		RepoID:      repoID,
		Description: description,
		Status:      StatusPending,
		DependsOn:   dependsOn,
		Attempt:     1,
		MaxAttempts:  5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
