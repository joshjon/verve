package epicapi

import (
	"errors"
	"net/http"

	"github.com/joshjon/kit/errtag"
	"github.com/labstack/echo/v4"

	"verve/internal/epic"
	"verve/internal/repo"
	"verve/internal/setting"
	"verve/internal/task"
)

// HTTPHandler handles epic HTTP requests.
type HTTPHandler struct {
	store          *epic.Store
	repoStore      *repo.Store
	taskStore      *task.Store
	settingService *setting.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(store *epic.Store, repoStore *repo.Store, taskStore *task.Store, settingService *setting.Service) *HTTPHandler {
	return &HTTPHandler{store: store, repoStore: repoStore, taskStore: taskStore, settingService: settingService}
}

// Register adds the epic endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	// Epic CRUD (repo-scoped)
	g.POST("/repos/:repo_id/epics", h.CreateEpic)
	g.GET("/repos/:repo_id/epics", h.ListEpicsByRepo)

	// Epic operations (globally unique IDs)
	g.GET("/epics/:id", h.GetEpic)
	g.GET("/epics/:id/tasks", h.GetEpicTasks)
	g.DELETE("/epics/:id", h.DeleteEpic)

	// Planning session
	g.POST("/epics/:id/plan", h.StartPlanning)
	g.PUT("/epics/:id/proposed-tasks", h.UpdateProposedTasks)
	g.POST("/epics/:id/session-message", h.SendSessionMessage)

	// Confirmation
	g.POST("/epics/:id/confirm", h.ConfirmEpic)
	g.POST("/epics/:id/close", h.CloseEpic)
}

// CreateEpic handles POST /repos/:repo_id/epics
func (h *HTTPHandler) CreateEpic(c echo.Context) error {
	repoID, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	var req CreateEpicRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("title required"))
	}
	if len(req.Title) > 200 {
		return c.JSON(http.StatusBadRequest, errorResponse("title must be 200 characters or less"))
	}

	e := epic.NewEpic(repoID.String(), req.Title, req.Description)
	e.PlanningPrompt = req.PlanningPrompt

	// Resolve model: use request value, fall back to default model setting, then "sonnet".
	model := req.Model
	if model == "" && h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	if model == "" {
		model = "sonnet"
	}
	e.Model = model

	if err := h.store.CreateEpic(c.Request().Context(), e); err != nil {
		return jsonError(c, err)
	}

	return c.JSON(http.StatusCreated, e)
}

// ListEpicsByRepo handles GET /repos/:repo_id/epics
func (h *HTTPHandler) ListEpicsByRepo(c echo.Context) error {
	repoID, err := repo.ParseRepoID(c.Param("repo_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid repo ID"))
	}

	epics, err := h.store.ListEpicsByRepo(c.Request().Context(), repoID.String())
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, epics)
}

// GetEpic handles GET /epics/:id
func (h *HTTPHandler) GetEpic(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	e, err := h.store.ReadEpic(c.Request().Context(), id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// DeleteEpic handles DELETE /epics/:id
func (h *HTTPHandler) DeleteEpic(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	if err := h.store.DeleteEpic(c.Request().Context(), id); err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, statusOK())
}

// StartPlanning handles POST /epics/:id/plan
func (h *HTTPHandler) StartPlanning(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req StartPlanningRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("prompt required"))
	}

	ctx := c.Request().Context()
	if err := h.store.StartPlanning(ctx, id, req.Prompt); err != nil {
		return jsonError(c, err)
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// UpdateProposedTasks handles PUT /epics/:id/proposed-tasks
func (h *HTTPHandler) UpdateProposedTasks(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req UpdateProposedTasksRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()
	if err := h.store.UpdateProposedTasks(ctx, id, req.Tasks); err != nil {
		return jsonError(c, err)
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// SendSessionMessage handles POST /epics/:id/session-message
func (h *HTTPHandler) SendSessionMessage(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req SessionMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, errorResponse("message required"))
	}

	ctx := c.Request().Context()
	if err := h.store.AppendSessionLog(ctx, id, []string{"user: " + req.Message}); err != nil {
		return jsonError(c, err)
	}

	// Signal the agent via feedback
	if err := h.store.SetFeedback(ctx, id, req.Message, epic.FeedbackMessage); err != nil {
		return jsonError(c, err)
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// ConfirmEpic handles POST /epics/:id/confirm
func (h *HTTPHandler) ConfirmEpic(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	var req ConfirmEpicRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid request"))
	}

	ctx := c.Request().Context()
	if err := h.store.ConfirmEpic(ctx, id, req.NotReady); err != nil {
		return jsonError(c, err)
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// CloseEpic handles POST /epics/:id/close
func (h *HTTPHandler) CloseEpic(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	ctx := c.Request().Context()
	if err := h.store.CloseEpic(ctx, id); err != nil {
		return jsonError(c, err)
	}

	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}
	return c.JSON(http.StatusOK, e)
}

// EpicTaskSummary contains the status summary for a task in an epic.
type EpicTaskSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// GetEpicTasks handles GET /epics/:id/tasks
// Returns the status of all tasks in the epic.
func (h *HTTPHandler) GetEpicTasks(c echo.Context) error {
	id, err := epic.ParseEpicID(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse("invalid epic ID"))
	}

	ctx := c.Request().Context()
	e, err := h.store.ReadEpic(ctx, id)
	if err != nil {
		return jsonError(c, err)
	}

	summaries := make([]EpicTaskSummary, 0, len(e.TaskIDs))
	for _, taskIDStr := range e.TaskIDs {
		taskID, parseErr := task.ParseTaskID(taskIDStr)
		if parseErr != nil {
			continue
		}
		t, readErr := h.taskStore.ReadTask(ctx, taskID)
		if readErr != nil {
			// Task may have been deleted
			continue
		}
		summaries = append(summaries, EpicTaskSummary{
			ID:     t.ID.String(),
			Title:  t.Title,
			Status: string(t.Status),
		})
	}

	return c.JSON(http.StatusOK, summaries)
}

func jsonError(c echo.Context, err error) error {
	code := http.StatusInternalServerError
	msg := "internal server error"

	var tagger errtag.Tagger
	if errors.As(err, &tagger) {
		code = tagger.Code()
		msg = tagger.Msg()
	}

	if code >= 500 {
		c.Logger().Errorf("handler error: method=%s path=%s status=%d error=%v", c.Request().Method, c.Path(), code, err)
	}

	return c.JSON(code, errorResponse(msg))
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func statusOK() map[string]string {
	return map[string]string{"status": "ok"}
}
