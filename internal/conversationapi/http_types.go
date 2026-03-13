package conversationapi

import (
	"github.com/cohesivestack/valgo"

	"github.com/vervesh/verve/internal/conversation"
	"github.com/vervesh/verve/internal/repo"
)

// CreateConversationRequest is the request body for creating a conversation.
type CreateConversationRequest struct {
	RepoID         string `param:"repo_id" json:"-"`
	Title          string `json:"title"`
	InitialMessage string `json:"initial_message,omitempty"`
	Model          string `json:"model,omitempty"`
}

func (r CreateConversationRequest) Validate() error {
	return valgo.
		In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).
		Is(valgo.String(r.Title, "title").Not().Blank().MaxLength(200)).
		ToError()
}

// ConversationIDRequest captures the :id path parameter.
type ConversationIDRequest struct {
	ID string `param:"id" json:"-"`
}

func (r ConversationIDRequest) Validate() error {
	return valgo.In("params", valgo.Is(conversation.ConversationIDValidator(r.ID, "id"))).ToError()
}

// SendMessageRequest is the request body for sending a message in a conversation.
type SendMessageRequest struct {
	ID      string `param:"id" json:"-"`
	Message string `json:"message"`
}

func (r SendMessageRequest) Validate() error {
	return valgo.In("params", valgo.Is(conversation.ConversationIDValidator(r.ID, "id"))).
		Is(valgo.String(r.Message, "message").Not().Blank()).
		ToError()
}

// ListConversationsRequest captures the :repo_id path parameter for listing conversations.
type ListConversationsRequest struct {
	RepoID string `param:"repo_id" json:"-"`
}

func (r ListConversationsRequest) Validate() error {
	return valgo.In("params", valgo.Is(repo.RepoIDValidator(r.RepoID, "repo_id"))).ToError()
}

// GenerateTasksRequest is the request body for generating tasks from a conversation.
type GenerateTasksRequest struct {
	ID             string `param:"id" json:"-"`
	Title          string `json:"title"`
	PlanningPrompt string `json:"planning_prompt,omitempty"`
}

func (r GenerateTasksRequest) Validate() error {
	return valgo.In("params", valgo.Is(conversation.ConversationIDValidator(r.ID, "id"))).
		Is(valgo.String(r.Title, "title").Not().Blank().MaxLength(200)).
		ToError()
}
