package taskapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/joshjon/kit/errtag"
	"github.com/labstack/echo/v4"

	"verve/internal/github"
	"verve/internal/task"
)

// HTTPHandler handles task-related HTTP requests.
type HTTPHandler struct {
	store  *task.Store
	github *github.Client
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *task.Store, githubClient *github.Client) *HTTPHandler {
	return &HTTPHandler{store: store, github: githubClient}
}

// Register adds the task endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.POST("/tasks", h.CreateTask)
	g.GET("/tasks", h.ListTasks)
	g.POST("/tasks/sync", h.SyncAllTasks)
	g.GET("/tasks/poll", h.PollTask)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/logs", h.AppendLogs)
	g.POST("/tasks/:id/complete", h.CompleteTask)
	g.POST("/tasks/:id/close", h.CloseTask)
	g.POST("/tasks/:id/sync", h.SyncTaskStatus)
}

// CreateTask handles POST /tasks
func (h *HTTPHandler) CreateTask(c echo.Context) error {
	var req CreateTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("description required"))
	}

	t := task.NewTask(req.Description, req.DependsOn)
	if err := h.store.CreateTask(c.Request().Context(), t); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusCreated, t)
}

// ListTasks handles GET /tasks
func (h *HTTPHandler) ListTasks(c echo.Context) error {
	tasks, err := h.store.ListTasks(c.Request().Context())
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, tasks)
}

// GetTask handles GET /tasks/:id
func (h *HTTPHandler) GetTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	t, err := h.store.ReadTask(c.Request().Context(), id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// PollTask handles GET /tasks/poll
// Long-polls for a pending task, claiming it atomically.
func (h *HTTPHandler) PollTask(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		t, err := h.store.ClaimPendingTask(c.Request().Context())
		if err != nil {
			return jsonError(c, err)
		}
		if t != nil {
			return c.JSON(http.StatusOK, t)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.NoContent(http.StatusNoContent)
		}

		select {
		case <-h.store.WaitForPending():
		case <-time.After(remaining):
			return c.NoContent(http.StatusNoContent)
		case <-c.Request().Context().Done():
			return c.NoContent(http.StatusNoContent)
		}
	}
}

// AppendLogs handles POST /tasks/:id/logs
func (h *HTTPHandler) AppendLogs(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req LogsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	if err := h.store.AppendTaskLogs(c.Request().Context(), id, req.Logs); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// CompleteTask handles POST /tasks/:id/complete
func (h *HTTPHandler) CompleteTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req CompleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	if !req.Success {
		if err := h.store.UpdateTaskStatus(ctx, id, task.StatusFailed); err != nil {
			return jsonError(c, err)
		}
	} else if req.PullRequestURL != "" {
		if err := h.store.SetTaskPullRequest(ctx, id, req.PullRequestURL, req.PRNumber); err != nil {
			return jsonError(c, err)
		}
	} else {
		if err := h.store.UpdateTaskStatus(ctx, id, task.StatusClosed); err != nil {
			return jsonError(c, err)
		}
	}

	return c.JSON(http.StatusOK, statusOK())
}

// SyncTaskStatus handles POST /tasks/:id/sync
func (h *HTTPHandler) SyncTaskStatus(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	ctx := c.Request().Context()

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	if t.Status == task.StatusReview && h.github != nil && t.PRNumber > 0 {
		merged, err := h.github.IsPRMerged(ctx, t.PRNumber)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResponse("failed to check PR status: "+err.Error()))
		}
		if merged {
			if err := h.store.UpdateTaskStatus(ctx, id, task.StatusMerged); err != nil {
				return jsonError(c, err)
			}
			t, err = h.store.ReadTask(ctx, id)
			if err != nil {
				return jsonError(c, err)
			}
		}
	}

	return c.JSON(http.StatusOK, t)
}

// SyncAllTasks handles POST /tasks/sync
func (h *HTTPHandler) SyncAllTasks(c echo.Context) error {
	if h.github == nil {
		return c.JSON(http.StatusOK, map[string]int{"synced": 0, "merged": 0})
	}

	ctx := c.Request().Context()

	tasks, err := h.store.ListTasksInReview(ctx)
	if err != nil {
		return jsonError(c, err)
	}

	synced := 0
	merged := 0
	for _, t := range tasks {
		if t.PRNumber > 0 {
			synced++
			isMerged, err := h.github.IsPRMerged(ctx, t.PRNumber)
			if err != nil {
				continue
			}
			if isMerged {
				_ = h.store.UpdateTaskStatus(ctx, t.ID, task.StatusMerged)
				merged++
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]int{"synced": synced, "merged": merged})
}

// CloseTask handles POST /tasks/:id/close
func (h *HTTPHandler) CloseTask(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req CloseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	if err := h.store.CloseTask(ctx, id, req.Reason); err != nil {
		return jsonError(c, err)
	}

	t, err := h.store.ReadTask(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

// jsonError maps errtag-tagged errors to appropriate HTTP status codes.
func jsonError(c echo.Context, err error) error {
	code := http.StatusInternalServerError
	msg := "internal server error"

	var tagger errtag.Tagger
	if errors.As(err, &tagger) {
		code = tagger.Code()
		msg = tagger.Msg()
	}

	return c.JSON(code, errorResponse(msg))
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func statusOK() map[string]string {
	return map[string]string{"status": "ok"}
}
