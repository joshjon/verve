package agentapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"verve/internal/epic"
	"verve/internal/githubtoken"
	"verve/internal/repo"
	"verve/internal/task"
)

// HTTPHandler handles agent-facing API requests.
type HTTPHandler struct {
	taskStore    *task.Store
	epicStore    *epic.Store
	repoStore    *repo.Store
	githubToken  *githubtoken.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(taskStore *task.Store, epicStore *epic.Store, repoStore *repo.Store, githubToken *githubtoken.Service) *HTTPHandler {
	return &HTTPHandler{
		taskStore:   taskStore,
		epicStore:   epicStore,
		repoStore:   repoStore,
		githubToken: githubToken,
	}
}

// Register adds the agent endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	// Unified poll (epics first, then tasks)
	g.GET("/poll", h.Poll)

	// Task agent endpoints
	g.POST("/tasks/:id/logs", h.TaskAppendLogs)
	g.POST("/tasks/:id/heartbeat", h.TaskHeartbeat)
	g.POST("/tasks/:id/complete", h.TaskComplete)

	// Epic agent endpoints
	g.POST("/epics/:id/propose", h.EpicPropose)
	g.GET("/epics/:id/poll-feedback", h.EpicPollFeedback)
	g.POST("/epics/:id/heartbeat", h.EpicHeartbeat)
	g.POST("/epics/:id/logs", h.EpicAppendLogs)
}

// Poll handles GET /poll — unified long-poll for available work.
// Tries to claim an epic first (higher priority), then a task.
func (h *HTTPHandler) Poll(c echo.Context) error {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	ctx := c.Request().Context()

	for {
		// Try epics first (higher priority)
		e, err := h.epicStore.ClaimPendingEpic(ctx)
		if err != nil {
			return jsonError(c, err)
		}
		if e != nil {
			resp, err := h.buildEpicPollResponse(c, e)
			if err != nil {
				return jsonError(c, err)
			}
			return c.JSON(http.StatusOK, resp)
		}

		// Then try tasks
		t, err := h.taskStore.ClaimPendingTask(ctx, nil)
		if err != nil {
			return jsonError(c, err)
		}
		if t != nil {
			resp, err := h.buildTaskPollResponse(c, t)
			if err != nil {
				return jsonError(c, err)
			}
			return c.JSON(http.StatusOK, resp)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.NoContent(http.StatusNoContent)
		}

		// Wait on both stores' pending channels
		select {
		case <-h.epicStore.WaitForPending():
		case <-h.taskStore.WaitForPending():
		case <-time.After(remaining):
			return c.NoContent(http.StatusNoContent)
		case <-ctx.Done():
			return c.NoContent(http.StatusNoContent)
		}
	}
}

func (h *HTTPHandler) buildEpicPollResponse(c echo.Context, e *epic.Epic) (*PollResponse, error) {
	repoID, err := repo.ParseRepoID(e.RepoID)
	if err != nil {
		return nil, err
	}
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return nil, err
	}
	var token string
	if h.githubToken != nil {
		token = h.githubToken.GetToken()
	}
	return &PollResponse{
		Type:         "epic",
		Epic:         e,
		GitHubToken:  token,
		RepoFullName: r.FullName,
	}, nil
}

func (h *HTTPHandler) buildTaskPollResponse(c echo.Context, t *task.Task) (*PollResponse, error) {
	repoID, err := repo.ParseRepoID(t.RepoID)
	if err != nil {
		return nil, err
	}
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return nil, err
	}
	var token string
	if h.githubToken != nil {
		token = h.githubToken.GetToken()
	}
	return &PollResponse{
		Type:         "task",
		Task:         t,
		GitHubToken:  token,
		RepoFullName: r.FullName,
	}, nil
}

// --- Task Agent Endpoints ---

// TaskAppendLogs handles POST /tasks/:id/logs
func (h *HTTPHandler) TaskAppendLogs(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}
	var req struct {
		Logs    []string `json:"logs"`
		Attempt int      `json:"attempt"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	attempt := req.Attempt
	if attempt == 0 {
		attempt = 1
	}
	if err := h.taskStore.AppendTaskLogs(c.Request().Context(), id, attempt, req.Logs); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// TaskHeartbeat handles POST /tasks/:id/heartbeat
func (h *HTTPHandler) TaskHeartbeat(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}
	if err := h.taskStore.Heartbeat(c.Request().Context(), id); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// TaskComplete handles POST /tasks/:id/complete
func (h *HTTPHandler) TaskComplete(c echo.Context) error {
	id, err := task.ParseTaskID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid task ID"))
	}

	var req struct {
		Success        bool    `json:"success"`
		PullRequestURL string  `json:"pull_request_url"`
		PRNumber       int     `json:"pr_number"`
		BranchName     string  `json:"branch_name"`
		Error          string  `json:"error"`
		AgentStatus    string  `json:"agent_status"`
		CostUSD        float64 `json:"cost_usd"`
		PrereqFailed   string  `json:"prereq_failed"`
		NoChanges      bool    `json:"no_changes"`
		Retryable      bool    `json:"retryable"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()

	if req.AgentStatus != "" {
		if err := h.taskStore.SetAgentStatus(ctx, id, req.AgentStatus); err != nil {
			return jsonError(c, err)
		}
	}
	if req.CostUSD > 0 {
		if err := h.taskStore.AddCost(ctx, id, req.CostUSD); err != nil {
			return jsonError(c, err)
		}
	}

	switch {
	case !req.Success:
		if req.PrereqFailed != "" {
			if err := h.taskStore.SetCloseReason(ctx, id, req.PrereqFailed); err != nil {
				return jsonError(c, err)
			}
		}
		if req.Retryable && req.PrereqFailed == "" {
			reason := "rate_limit: " + req.Error
			if err := h.taskStore.ScheduleRetry(ctx, id, reason); err != nil {
				return jsonError(c, err)
			}
			return c.JSON(http.StatusOK, statusOK())
		}
		t, readErr := h.taskStore.ReadTask(ctx, id)
		if readErr != nil {
			return jsonError(c, readErr)
		}
		if req.PrereqFailed == "" && (t.PRNumber > 0 || t.BranchName != "") {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return jsonError(c, err)
			}
		} else {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusFailed); err != nil {
				return jsonError(c, err)
			}
		}
	case req.PullRequestURL != "":
		if err := h.taskStore.SetTaskPullRequest(ctx, id, req.PullRequestURL, req.PRNumber); err != nil {
			return jsonError(c, err)
		}
	case req.BranchName != "":
		if err := h.taskStore.SetTaskBranch(ctx, id, req.BranchName); err != nil {
			return jsonError(c, err)
		}
	default:
		t, readErr := h.taskStore.ReadTask(ctx, id)
		if readErr != nil {
			return jsonError(c, readErr)
		}
		if t.PRNumber > 0 || t.BranchName != "" {
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusReview); err != nil {
				return jsonError(c, err)
			}
		} else {
			if req.NoChanges {
				if err := h.taskStore.SetCloseReason(ctx, id, "No changes needed — the codebase already meets the required criteria"); err != nil {
					return jsonError(c, err)
				}
			}
			if err := h.taskStore.UpdateTaskStatus(ctx, id, task.StatusClosed); err != nil {
				return jsonError(c, err)
			}
		}
	}

	return c.JSON(http.StatusOK, statusOK())
}

// --- Epic Agent Endpoints ---

// EpicPropose handles POST /epics/:id/propose — agent submits proposed tasks.
func (h *HTTPHandler) EpicPropose(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req ProposeTasksRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()
	if err := h.epicStore.UpdateProposedTasks(ctx, id, req.Tasks); err != nil {
		return jsonError(c, err)
	}

	e, err := h.epicStore.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// EpicPollFeedback handles GET /epics/:id/poll-feedback — agent long-polls for user feedback.
func (h *HTTPHandler) EpicPollFeedback(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)
	ctx := c.Request().Context()

	for {
		// Check for pending feedback in DB
		feedback, feedbackType, err := h.epicStore.PollFeedback(ctx, id)
		if err != nil {
			return jsonError(c, err)
		}
		if feedbackType != nil {
			resp := FeedbackResponse{Type: *feedbackType}
			if feedback != nil {
				resp.Feedback = *feedback
			}
			return c.JSON(http.StatusOK, resp)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return c.JSON(http.StatusOK, FeedbackResponse{Type: "timeout"})
		}

		select {
		case <-h.epicStore.WaitForFeedback(id.String()):
		case <-time.After(remaining):
			return c.JSON(http.StatusOK, FeedbackResponse{Type: "timeout"})
		case <-ctx.Done():
			return c.JSON(http.StatusOK, FeedbackResponse{Type: "timeout"})
		}
	}
}

// EpicHeartbeat handles POST /epics/:id/heartbeat
func (h *HTTPHandler) EpicHeartbeat(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}
	if err := h.epicStore.EpicHeartbeat(c.Request().Context(), id); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// EpicAppendLogs handles POST /epics/:id/logs — agent appends session log entries.
func (h *HTTPHandler) EpicAppendLogs(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req SessionLogRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	if err := h.epicStore.AppendSessionLog(c.Request().Context(), id, req.Lines); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// --- Helpers ---

func jsonError(c echo.Context, err error) error {
	c.Logger().Errorf("handler error: method=%s path=%s status=500 error=%v", c.Request().Method, c.Path(), err)
	return c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func statusOK() map[string]string {
	return map[string]string{"status": "ok"}
}
