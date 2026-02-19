package epicapi

import "verve/internal/epic"

// CreateEpicRequest is the request body for creating an epic.
type CreateEpicRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	PlanningPrompt string `json:"planning_prompt,omitempty"`
	Model          string `json:"model,omitempty"`
}

// StartPlanningRequest is the request body for starting a planning session.
type StartPlanningRequest struct {
	Prompt string `json:"prompt"`
}

// UpdateProposedTasksRequest is the request body for updating proposed tasks.
type UpdateProposedTasksRequest struct {
	Tasks []epic.ProposedTask `json:"tasks"`
}

// SessionMessageRequest is the request body for sending a message in a planning session.
type SessionMessageRequest struct {
	Message string `json:"message"`
}

// ConfirmEpicRequest is the request body for confirming an epic.
type ConfirmEpicRequest struct {
	NotReady bool `json:"not_ready,omitempty"`
}
