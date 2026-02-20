package task

import (
	"context"
	"time"
)

// AgentMetrics provides a snapshot of agent activity and performance.
type AgentMetrics struct {
	// Currently running agents
	RunningAgents int `json:"running_agents"`
	// Tasks pending to be picked up
	PendingTasks int `json:"pending_tasks"`
	// Tasks in review (PR created, awaiting merge)
	ReviewTasks int `json:"review_tasks"`
	// Total tasks (all statuses)
	TotalTasks int `json:"total_tasks"`

	// Completed tasks (merged + closed)
	CompletedTasks int `json:"completed_tasks"`
	// Failed tasks
	FailedTasks int `json:"failed_tasks"`

	// Total cost across all tasks (USD)
	TotalCostUSD float64 `json:"total_cost_usd"`

	// Details about each currently running agent
	ActiveAgents []ActiveAgent `json:"active_agents"`

	// Recent completions (last 10 tasks that reached a terminal state)
	RecentCompletions []CompletedAgent `json:"recent_completions"`
}

// ActiveAgent describes a single running agent session.
type ActiveAgent struct {
	TaskID     string  `json:"task_id"`
	TaskTitle  string  `json:"task_title"`
	RepoID     string  `json:"repo_id"`
	StartedAt  string  `json:"started_at"`
	RunningFor int64   `json:"running_for_ms"`
	Attempt    int     `json:"attempt"`
	CostUSD    float64 `json:"cost_usd"`
	Model      string  `json:"model,omitempty"`
	EpicID     string  `json:"epic_id,omitempty"`
}

// CompletedAgent describes a recently completed agent session.
type CompletedAgent struct {
	TaskID     string  `json:"task_id"`
	TaskTitle  string  `json:"task_title"`
	RepoID     string  `json:"repo_id"`
	Status     string  `json:"status"`
	DurationMs *int64  `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd"`
	Attempt    int     `json:"attempt"`
	FinishedAt string  `json:"finished_at"`
}

// GetAgentMetrics computes agent observability metrics from current task data.
func (s *Store) GetAgentMetrics(ctx context.Context) (*AgentMetrics, error) {
	tasks, err := s.repo.ListTasks(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m := &AgentMetrics{}

	var recentTerminal []*Task

	for _, t := range tasks {
		m.TotalTasks++
		m.TotalCostUSD += t.CostUSD

		switch t.Status {
		case StatusPending:
			m.PendingTasks++
		case StatusRunning:
			m.RunningAgents++
			agent := ActiveAgent{
				TaskID:    t.ID.String(),
				TaskTitle: t.Title,
				RepoID:    t.RepoID,
				Attempt:   t.Attempt,
				CostUSD:   t.CostUSD,
				Model:     t.Model,
				EpicID:    t.EpicID,
			}
			if t.StartedAt != nil {
				agent.StartedAt = t.StartedAt.Format(time.RFC3339)
				agent.RunningFor = now.Sub(*t.StartedAt).Milliseconds()
			}
			m.ActiveAgents = append(m.ActiveAgents, agent)
		case StatusReview:
			m.ReviewTasks++
		case StatusMerged, StatusClosed:
			m.CompletedTasks++
			recentTerminal = append(recentTerminal, t)
		case StatusFailed:
			m.FailedTasks++
			recentTerminal = append(recentTerminal, t)
		}
	}

	// Sort terminal tasks by updated_at descending (most recent first) and take top 10.
	// Simple selection: iterate and keep the 10 most recent.
	if len(recentTerminal) > 0 {
		// Sort by UpdatedAt descending.
		for i := 0; i < len(recentTerminal); i++ {
			for j := i + 1; j < len(recentTerminal); j++ {
				if recentTerminal[j].UpdatedAt.After(recentTerminal[i].UpdatedAt) {
					recentTerminal[i], recentTerminal[j] = recentTerminal[j], recentTerminal[i]
				}
			}
		}
		limit := 10
		if len(recentTerminal) < limit {
			limit = len(recentTerminal)
		}
		for _, t := range recentTerminal[:limit] {
			t.ComputeDuration()
			c := CompletedAgent{
				TaskID:     t.ID.String(),
				TaskTitle:  t.Title,
				RepoID:     t.RepoID,
				Status:     string(t.Status),
				DurationMs: t.DurationMs,
				CostUSD:    t.CostUSD,
				Attempt:    t.Attempt,
				FinishedAt: t.UpdatedAt.Format(time.RFC3339),
			}
			m.RecentCompletions = append(m.RecentCompletions, c)
		}
	}

	if m.ActiveAgents == nil {
		m.ActiveAgents = []ActiveAgent{}
	}
	if m.RecentCompletions == nil {
		m.RecentCompletions = []CompletedAgent{}
	}

	return m, nil
}
