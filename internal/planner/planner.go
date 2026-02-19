package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"verve/internal/epic"
)

const defaultModel = "claude-sonnet-4-20250514"

// Planner calls the Claude API to generate proposed tasks from an epic.
type Planner struct {
	apiKey string
	model  string
	client *http.Client
}

// New creates a new Planner with the given Anthropic API key.
func New(apiKey string) *Planner {
	return &Planner{
		apiKey: apiKey,
		model:  defaultModel,
		client: &http.Client{},
	}
}

// PlanEpic calls Claude to generate proposed tasks for the given epic.
func (p *Planner) PlanEpic(ctx context.Context, title, description, prompt string) ([]epic.ProposedTask, error) {
	userContent := fmt.Sprintf("Epic Title: %s\n\nEpic Description:\n%s", title, description)
	if prompt != "" {
		userContent += fmt.Sprintf("\n\nAdditional Planning Instructions:\n%s", prompt)
	}

	reqBody := apiRequest{
		Model:     p.model,
		MaxTokens: 8192,
		System:    systemPrompt,
		Messages: []message{
			{Role: "user", Content: userContent},
		},
		Tools: []tool{
			{
				Name:        "propose_tasks",
				Description: "Propose a set of tasks that implement the epic. Each task should be small enough for one PR.",
				InputSchema: proposedTasksSchema(),
			},
		},
		ToolChoice: &toolChoice{Type: "tool", Name: "propose_tasks"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Claude API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude API error (status %d): %s", resp.StatusCode, respBody)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Extract the tool use result from the response.
	for _, block := range apiResp.Content {
		if block.Type != "tool_use" || block.Name != "propose_tasks" {
			continue
		}

		var result proposedTasksResult
		raw, err := json.Marshal(block.Input)
		if err != nil {
			return nil, fmt.Errorf("marshal tool input: %w", err)
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("unmarshal tool result: %w", err)
		}

		tasks := make([]epic.ProposedTask, len(result.Tasks))
		for i, t := range result.Tasks {
			tempID := fmt.Sprintf("task_%d", i+1)

			var depIDs []string
			for _, depIdx := range t.DependsOn {
				if depIdx >= 1 && depIdx <= len(result.Tasks) {
					depIDs = append(depIDs, fmt.Sprintf("task_%d", depIdx))
				}
			}

			tasks[i] = epic.ProposedTask{
				TempID:             tempID,
				Title:              t.Title,
				Description:        t.Description,
				DependsOnTempIDs:   depIDs,
				AcceptanceCriteria: t.AcceptanceCriteria,
			}
		}
		return tasks, nil
	}

	return nil, fmt.Errorf("no tool_use block found in Claude response")
}

const systemPrompt = `You are an expert software planning agent. Your job is to break down an epic (a large software deliverable) into a set of small, well-scoped implementation tasks.

Guidelines:
- Each task should be completable in a single pull request
- Tasks should be ordered logically — data models and APIs before UI
- Specify dependencies between tasks using 1-based task indices
- Write clear, actionable titles (imperative mood)
- Include detailed descriptions with implementation guidance
- Add concrete acceptance criteria for each task
- Aim for 3-10 tasks depending on epic complexity
- Keep tasks focused: one concern per task`

// API types

type apiRequest struct {
	Model      string      `json:"model"`
	MaxTokens  int         `json:"max_tokens"`
	System     string      `json:"system"`
	Messages   []message   `json:"messages"`
	Tools      []tool      `json:"tools"`
	ToolChoice *toolChoice `json:"tool_choice,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type apiResponse struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type proposedTasksResult struct {
	Tasks []taskResult `json:"tasks"`
}

type taskResult struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	DependsOn          []int    `json:"depends_on"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

func proposedTasksSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"tasks": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"title": {
							"type": "string",
							"description": "Clear, actionable task title in imperative mood"
						},
						"description": {
							"type": "string",
							"description": "Detailed implementation description with guidance"
						},
						"depends_on": {
							"type": "array",
							"items": {"type": "integer"},
							"description": "1-based indices of tasks this depends on"
						},
						"acceptance_criteria": {
							"type": "array",
							"items": {"type": "string"},
							"description": "Concrete acceptance criteria"
						}
					},
					"required": ["title", "description", "acceptance_criteria"]
				}
			}
		},
		"required": ["tasks"]
	}`)
}
