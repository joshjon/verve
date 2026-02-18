package epic

import "time"

// Status represents the lifecycle state of an Epic.
type Status string

const (
	StatusDraft     Status = "draft"     // Planning session active, tasks being designed
	StatusPlanning  Status = "planning"  // Agent is actively proposing/updating tasks
	StatusReady     Status = "ready"     // Planning complete, tasks confirmed but not started
	StatusActive    Status = "active"    // Tasks are being picked up by agents
	StatusCompleted Status = "completed" // All tasks finished
	StatusClosed    Status = "closed"    // Manually closed
)

// ProposedTask represents a task proposed by the planning agent during
// an epic planning session. These are not yet created in the task system.
type ProposedTask struct {
	TempID             string   `json:"temp_id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOnTempIDs   []string `json:"depends_on_temp_ids,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

// Epic represents a large deliverable that contains multiple related tasks.
type Epic struct {
	ID             EpicID         `json:"id"`
	RepoID         string         `json:"repo_id"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Status         Status         `json:"status"`
	ProposedTasks  []ProposedTask `json:"proposed_tasks"`
	TaskIDs        []string       `json:"task_ids"`
	PlanningPrompt string         `json:"planning_prompt,omitempty"`
	SessionLog     []string       `json:"session_log,omitempty"`
	NotReady       bool           `json:"not_ready"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// NewEpic creates a new Epic with a generated EpicID and draft status.
func NewEpic(repoID, title, description string) *Epic {
	now := time.Now()
	return &Epic{
		ID:            NewEpicID(),
		RepoID:        repoID,
		Title:         title,
		Description:   description,
		Status:        StatusDraft,
		ProposedTasks: []ProposedTask{},
		TaskIDs:       []string{},
		SessionLog:    []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
