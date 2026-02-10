package server

// TaskStore defines the interface for task storage.
type TaskStore interface {
	// Create creates a new task.
	Create(description string, opts *CreateOptions) (*Task, error)

	// Get retrieves a task by ID.
	Get(id string) *Task

	// List returns all tasks.
	List() []*Task

	// ClaimPending atomically finds and claims a pending task with all dependencies met.
	ClaimPending() *Task

	// WaitForPending returns a channel that signals when a pending task might be available.
	WaitForPending() <-chan struct{}

	// AppendLogs appends logs to a task.
	AppendLogs(id string, logs []string) bool

	// UpdateStatus updates a task's status.
	UpdateStatus(id string, status TaskStatus) bool

	// SetPullRequest sets the PR URL and number for a task.
	SetPullRequest(id string, prURL string, prNumber int) bool

	// GetTasksInReview returns all tasks in review status.
	GetTasksInReview() []*Task

	// CloseTask closes a task with an optional reason.
	CloseTask(id string, reason string) bool
}
