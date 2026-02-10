package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Handlers struct {
	store        TaskStore
	githubClient *GitHubClient
}

func NewHandlers(store TaskStore, githubClient *GitHubClient) *Handlers {
	return &Handlers{store: store, githubClient: githubClient}
}

type CreateTaskRequest struct {
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

type LogsRequest struct {
	Logs []string `json:"logs"`
}

type CompleteRequest struct {
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
	PullRequestURL string `json:"pull_request_url,omitempty"`
	PRNumber       int    `json:"pr_number,omitempty"`
}

type CloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CreateTask handles POST /api/v1/tasks
func (h *Handlers) CreateTask(c echo.Context) error {
	var req CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "description required"})
	}

	opts := &CreateOptions{
		DependsOn: req.DependsOn,
	}

	task, err := h.store.Create(req.Description, opts)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /api/v1/tasks
func (h *Handlers) ListTasks(c echo.Context) error {
	tasks := h.store.List()
	return c.JSON(http.StatusOK, tasks)
}

// GetTask handles GET /api/v1/tasks/:id
func (h *Handlers) GetTask(c echo.Context) error {
	id := c.Param("id")
	task := h.store.Get(id)
	if task == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}
	return c.JSON(http.StatusOK, task)
}

// PollTask handles GET /api/v1/tasks/poll
// Long-polls for a pending task, claiming it atomically
func (h *Handlers) PollTask(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		// Try to claim a pending task
		task := h.store.ClaimPending()
		if task != nil {
			return c.JSON(http.StatusOK, task)
		}

		// Check if we've exceeded the timeout
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.NoContent(http.StatusNoContent)
		}

		// Wait for a new task or timeout
		select {
		case <-h.store.WaitForPending():
			// A new task might be available, loop and try again
		case <-time.After(remaining):
			return c.NoContent(http.StatusNoContent)
		case <-c.Request().Context().Done():
			return c.NoContent(http.StatusNoContent)
		}
	}
}

// AppendLogs handles POST /api/v1/tasks/:id/logs
func (h *Handlers) AppendLogs(c echo.Context) error {
	id := c.Param("id")

	var req LogsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if !h.store.AppendLogs(id, req.Logs) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// CompleteTask handles POST /api/v1/tasks/:id/complete
func (h *Handlers) CompleteTask(c echo.Context) error {
	id := c.Param("id")

	var req CompleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if !req.Success {
		// Failed task
		if !h.store.UpdateStatus(id, TaskStatusFailed) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
	} else if req.PullRequestURL != "" {
		// Success with PR - set to review status
		if !h.store.SetPullRequest(id, req.PullRequestURL, req.PRNumber) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
	} else {
		// Success without PR (no changes) - mark as closed
		if !h.store.UpdateStatus(id, TaskStatusClosed) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// SyncTaskStatus handles POST /api/v1/tasks/:id/sync
// Checks GitHub to update PR merge status
func (h *Handlers) SyncTaskStatus(c echo.Context) error {
	id := c.Param("id")
	task := h.store.Get(id)
	if task == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	if task.Status != TaskStatusReview {
		return c.JSON(http.StatusOK, task)
	}

	// Check GitHub for PR status
	if h.githubClient != nil && task.PRNumber > 0 {
		merged, err := h.githubClient.IsPRMerged(c.Request().Context(), task.PRNumber)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check PR status: " + err.Error()})
		}
		if merged {
			h.store.UpdateStatus(id, TaskStatusMerged)
			task = h.store.Get(id) // Refresh
		}
	}

	return c.JSON(http.StatusOK, task)
}

// SyncAllTasks handles POST /api/v1/tasks/sync
// Syncs all tasks in review status with GitHub
func (h *Handlers) SyncAllTasks(c echo.Context) error {
	if h.githubClient == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"synced": 0,
			"merged": 0,
		})
	}

	tasks := h.store.GetTasksInReview()
	synced := 0
	merged := 0

	for _, task := range tasks {
		if task.PRNumber > 0 {
			synced++
			isMerged, err := h.githubClient.IsPRMerged(c.Request().Context(), task.PRNumber)
			if err != nil {
				continue
			}
			if isMerged {
				h.store.UpdateStatus(task.ID, TaskStatusMerged)
				merged++
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"synced": synced,
		"merged": merged,
	})
}

// CloseTask handles POST /api/v1/tasks/:id/close
// Manually closes a task with an optional reason
func (h *Handlers) CloseTask(c echo.Context) error {
	id := c.Param("id")

	var req CloseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if !h.store.CloseTask(id, req.Reason) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	task := h.store.Get(id)
	return c.JSON(http.StatusOK, task)
}
