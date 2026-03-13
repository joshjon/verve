package conversationapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/joshjon/kit/server"
	"github.com/labstack/echo/v4"

	"github.com/vervesh/verve/internal/conversation"
	"github.com/vervesh/verve/internal/epic"
	"github.com/vervesh/verve/internal/logkey"
	"github.com/vervesh/verve/internal/repo"
	"github.com/vervesh/verve/internal/setting"
)

// HTTPHandler handles conversation HTTP requests.
type HTTPHandler struct {
	conversationStore *conversation.Store
	repoStore         *repo.Store
	epicStore         *epic.Store
	settingService    *setting.Service
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(conversationStore *conversation.Store, repoStore *repo.Store, epicStore *epic.Store, settingService *setting.Service) *HTTPHandler {
	return &HTTPHandler{
		conversationStore: conversationStore,
		repoStore:         repoStore,
		epicStore:         epicStore,
		settingService:    settingService,
	}
}

// Register adds the conversation endpoints to the provided Echo router group.
func (h *HTTPHandler) Register(g *echo.Group) {
	g.POST("/repos/:repo_id/conversations", h.CreateConversation)
	g.GET("/repos/:repo_id/conversations", h.ListConversationsByRepo)

	g.GET("/conversations/:id", h.GetConversation)
	g.DELETE("/conversations/:id", h.DeleteConversation)

	g.POST("/conversations/:id/messages", h.SendMessage)
	g.POST("/conversations/:id/archive", h.ArchiveConversation)
	g.POST("/conversations/:id/generate-tasks", h.GenerateTasks)
}

// CreateConversation handles POST /repos/:repo_id/conversations
func (h *HTTPHandler) CreateConversation(c echo.Context) error {
	req, err := server.BindRequest[CreateConversationRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	// Validate repo exists and setup is complete.
	r, err := h.repoStore.ReadRepo(c.Request().Context(), repoID)
	if err != nil {
		return err
	}
	if r.SetupStatus != repo.SetupStatusReady {
		return echo.NewHTTPError(http.StatusConflict, "repository setup is not complete — finish setup before starting conversations")
	}

	model := req.Model
	if model == "" && h.settingService != nil {
		model = h.settingService.Get(setting.KeyDefaultModel)
	}
	if model == "" {
		model = "sonnet"
	}

	conv := conversation.NewConversation(repoID.String(), req.Title, model)
	if err := h.conversationStore.CreateConversation(c.Request().Context(), conv); err != nil {
		return err
	}

	c.Set(logkey.ConversationID, conv.ID.String())

	// If initial message is provided, queue it immediately.
	if req.InitialMessage != "" {
		if err := h.conversationStore.SendMessage(c.Request().Context(), conv.ID, req.InitialMessage); err != nil {
			return err
		}
		// Re-read to get updated state with the message.
		conv, err = h.conversationStore.ReadConversation(c.Request().Context(), conv.ID)
		if err != nil {
			return err
		}
	}

	return server.SetResponse(c, http.StatusCreated, conv)
}

// ListConversationsByRepo handles GET /repos/:repo_id/conversations
func (h *HTTPHandler) ListConversationsByRepo(c echo.Context) error {
	req, err := server.BindRequest[ListConversationsRequest](c)
	if err != nil {
		return err
	}
	repoID := repo.MustParseRepoID(req.RepoID)
	c.Set(logkey.RepoID, repoID.String())

	convos, err := h.conversationStore.ListConversationsByRepo(c.Request().Context(), repoID.String())
	if err != nil {
		return err
	}

	// Filter by status query param (default: active only).
	statusFilter := c.QueryParam("status")
	if statusFilter == "" {
		statusFilter = string(conversation.StatusActive)
	}

	if statusFilter != "all" {
		filtered := make([]*conversation.Conversation, 0, len(convos))
		for _, conv := range convos {
			if string(conv.Status) == statusFilter {
				filtered = append(filtered, conv)
			}
		}
		convos = filtered
	}

	return server.SetResponseList(c, http.StatusOK, convos, "")
}

// GetConversation handles GET /conversations/:id
func (h *HTTPHandler) GetConversation(c echo.Context) error {
	req, err := server.BindRequest[ConversationIDRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	conv, err := h.conversationStore.ReadConversation(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, conv)
}

// DeleteConversation handles DELETE /conversations/:id
func (h *HTTPHandler) DeleteConversation(c echo.Context) error {
	req, err := server.BindRequest[ConversationIDRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	ctx := c.Request().Context()

	if _, err := h.conversationStore.ReadConversation(ctx, id); err != nil {
		return err
	}
	if err := h.conversationStore.DeleteConversation(ctx, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// SendMessage handles POST /conversations/:id/messages
func (h *HTTPHandler) SendMessage(c echo.Context) error {
	req, err := server.BindRequest[SendMessageRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	ctx := c.Request().Context()
	if err := h.conversationStore.SendMessage(ctx, id, req.Message); err != nil {
		return err
	}

	conv, err := h.conversationStore.ReadConversation(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, conv)
}

// ArchiveConversation handles POST /conversations/:id/archive
func (h *HTTPHandler) ArchiveConversation(c echo.Context) error {
	req, err := server.BindRequest[ConversationIDRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	ctx := c.Request().Context()
	if err := h.conversationStore.ArchiveConversation(ctx, id); err != nil {
		return err
	}

	conv, err := h.conversationStore.ReadConversation(ctx, id)
	if err != nil {
		return err
	}
	return server.SetResponse(c, http.StatusOK, conv)
}

// GenerateTasks handles POST /conversations/:id/generate-tasks
func (h *HTTPHandler) GenerateTasks(c echo.Context) error {
	req, err := server.BindRequest[GenerateTasksRequest](c)
	if err != nil {
		return err
	}
	id := conversation.MustParseConversationID(req.ID)
	c.Set(logkey.ConversationID, id.String())

	ctx := c.Request().Context()

	conv, err := h.conversationStore.ReadConversation(ctx, id)
	if err != nil {
		return err
	}

	// Must be active.
	if conv.Status != conversation.StatusActive {
		return echo.NewHTTPError(http.StatusConflict, "conversation must be active to generate tasks")
	}

	// Must not already have an epic linked.
	if conv.EpicID != nil {
		return echo.NewHTTPError(http.StatusConflict, "conversation already has a linked epic")
	}

	// Build the planning prompt with conversation transcript.
	var transcript strings.Builder
	transcript.WriteString("The following is a conversation about the repository that contains ideas and discussions that should be turned into actionable tasks:\n\n")
	transcript.WriteString("--- Conversation Transcript ---\n")
	for _, msg := range conv.Messages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		transcript.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}
	transcript.WriteString("--- End Transcript ---\n\n")
	transcript.WriteString("Based on this conversation, generate a set of concrete, actionable implementation tasks.\n")
	if req.PlanningPrompt != "" {
		transcript.WriteString("\n")
		transcript.WriteString(req.PlanningPrompt)
		transcript.WriteString("\n")
	}

	// Create the epic.
	e := epic.NewEpic(conv.RepoID, req.Title, fmt.Sprintf("Tasks generated from conversation: %s", conv.Title))
	e.PlanningPrompt = transcript.String()
	e.Model = conv.Model

	if err := h.epicStore.CreateEpic(ctx, e); err != nil {
		return err
	}

	// Link the conversation to the epic.
	if err := h.conversationStore.SetEpicID(ctx, id, e.ID.String()); err != nil {
		return err
	}

	c.Set(logkey.EpicID, e.ID.String())
	return server.SetResponse(c, http.StatusCreated, e)
}
