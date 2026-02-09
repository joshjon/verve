package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Handlers struct {
	store *Store
}

func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

type CreateTaskRequest struct {
	Description string `json:"description"`
}

type LogsRequest struct {
	Logs []string `json:"logs"`
}

type CompleteRequest struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
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

	task := h.store.Create(req.Description)
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
			return c.JSON(http.StatusNoContent, nil)
		}

		// Wait for a new task or timeout
		select {
		case <-h.store.WaitForPending():
			// A new task might be available, loop and try again
		case <-time.After(remaining):
			return c.JSON(http.StatusNoContent, nil)
		case <-c.Request().Context().Done():
			return c.JSON(http.StatusNoContent, nil)
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

	status := TaskStatusCompleted
	if !req.Success {
		status = TaskStatusFailed
	}

	if !h.store.UpdateStatus(id, status) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
