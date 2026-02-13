package taskapi

// CreateTaskRequest is the request body for creating a task.
type CreateTaskRequest struct {
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

// LogsRequest is the request body for appending logs.
type LogsRequest struct {
	Logs []string `json:"logs"`
}

// CompleteRequest is the request body for completing a task.
type CompleteRequest struct {
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	PullRequestURL string `json:"pull_request_url,omitempty"`
	PRNumber       int    `json:"pr_number,omitempty"`
}

// CloseRequest is the request body for closing a task.
type CloseRequest struct {
	Reason string `json:"reason,omitempty"`
}
