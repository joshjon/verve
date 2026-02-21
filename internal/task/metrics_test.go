package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_GetAgentMetrics_Empty(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	metrics, err := store.GetAgentMetrics(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, metrics.RunningAgents)
	assert.Equal(t, 0, metrics.PendingTasks)
	assert.Equal(t, 0, metrics.ReviewTasks)
	assert.Equal(t, 0, metrics.TotalTasks)
	assert.Equal(t, 0, metrics.CompletedTasks)
	assert.Equal(t, 0, metrics.FailedTasks)
	assert.Equal(t, 0.0, metrics.TotalCostUSD)
	assert.Empty(t, metrics.ActiveAgents)
	assert.Empty(t, metrics.RecentCompletions)
}

func TestStore_GetAgentMetrics_Counts(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Create tasks in various states
	pending := NewTask("repo_1", "pending task", "desc", nil, nil, 0, false, "sonnet", true)
	pending.Status = StatusPending
	repo.tasks[pending.ID.String()] = pending

	now := time.Now()
	startedAt := now.Add(-10 * time.Minute)
	running := NewTask("repo_1", "running task", "desc", nil, nil, 0, false, "opus", true)
	running.Status = StatusRunning
	running.StartedAt = &startedAt
	running.CostUSD = 1.50
	running.Model = "opus"
	repo.tasks[running.ID.String()] = running

	review := NewTask("repo_1", "review task", "desc", nil, nil, 0, false, "sonnet", true)
	review.Status = StatusReview
	review.CostUSD = 0.75
	repo.tasks[review.ID.String()] = review

	merged := NewTask("repo_1", "merged task", "desc", nil, nil, 0, false, "sonnet", true)
	merged.Status = StatusMerged
	merged.CostUSD = 2.00
	merged.UpdatedAt = now.Add(-5 * time.Minute)
	repo.tasks[merged.ID.String()] = merged

	closed := NewTask("repo_1", "closed task", "desc", nil, nil, 0, false, "sonnet", true)
	closed.Status = StatusClosed
	closed.UpdatedAt = now.Add(-3 * time.Minute)
	repo.tasks[closed.ID.String()] = closed

	failed := NewTask("repo_1", "failed task", "desc", nil, nil, 0, false, "sonnet", true)
	failed.Status = StatusFailed
	failed.CostUSD = 0.50
	failed.UpdatedAt = now.Add(-1 * time.Minute)
	repo.tasks[failed.ID.String()] = failed

	metrics, err := store.GetAgentMetrics(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 6, metrics.TotalTasks)
	assert.Equal(t, 1, metrics.RunningAgents)
	assert.Equal(t, 1, metrics.PendingTasks)
	assert.Equal(t, 1, metrics.ReviewTasks)
	assert.Equal(t, 2, metrics.CompletedTasks) // merged + closed
	assert.Equal(t, 1, metrics.FailedTasks)
	assert.InDelta(t, 4.75, metrics.TotalCostUSD, 0.01)

	// Check active agents
	require.Len(t, metrics.ActiveAgents, 1)
	assert.Equal(t, running.ID.String(), metrics.ActiveAgents[0].TaskID)
	assert.Equal(t, "running task", metrics.ActiveAgents[0].TaskTitle)
	assert.Equal(t, "opus", metrics.ActiveAgents[0].Model)
	assert.True(t, metrics.ActiveAgents[0].RunningFor > 0)

	// Check recent completions: merged, closed, failed = 3 total
	assert.Len(t, metrics.RecentCompletions, 3)
	// Should be sorted by UpdatedAt descending
	assert.Equal(t, "failed", metrics.RecentCompletions[0].Status)
}

func TestStore_GetAgentMetrics_IncludesPlanningEpics(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Create a running task
	now := time.Now()
	startedAt := now.Add(-10 * time.Minute)
	running := NewTask("repo_1", "running task", "desc", nil, nil, 0, false, "sonnet", true)
	running.Status = StatusRunning
	running.StartedAt = &startedAt
	repo.tasks[running.ID.String()] = running

	// Set up a mock planning epic lister
	claimedAt := now.Add(-5 * time.Minute)
	store.SetPlanningEpicLister(NewPlanningEpicListerFunc(
		func(ctx context.Context) ([]PlanningEpic, error) {
			return []PlanningEpic{
				{
					ID:        "epc_planning1",
					Title:     "Plan user auth feature",
					RepoID:    "repo_1",
					Model:     "opus",
					ClaimedAt: &claimedAt,
				},
			}, nil
		},
	))

	metrics, err := store.GetAgentMetrics(context.Background())
	require.NoError(t, err)

	// Running agents should include both the running task and the planning epic
	assert.Equal(t, 2, metrics.RunningAgents)
	require.Len(t, metrics.ActiveAgents, 2)

	// Find the planning agent in the list
	var planningAgent *ActiveAgent
	var taskAgent *ActiveAgent
	for i := range metrics.ActiveAgents {
		if metrics.ActiveAgents[i].IsPlanning {
			planningAgent = &metrics.ActiveAgents[i]
		} else {
			taskAgent = &metrics.ActiveAgents[i]
		}
	}

	// Verify task agent
	require.NotNil(t, taskAgent)
	assert.Equal(t, running.ID.String(), taskAgent.TaskID)
	assert.False(t, taskAgent.IsPlanning)

	// Verify planning agent
	require.NotNil(t, planningAgent)
	assert.Equal(t, "epc_planning1", planningAgent.TaskID)
	assert.Equal(t, "Plan user auth feature", planningAgent.TaskTitle)
	assert.Equal(t, "repo_1", planningAgent.RepoID)
	assert.Equal(t, "opus", planningAgent.Model)
	assert.Equal(t, "epc_planning1", planningAgent.EpicID)
	assert.True(t, planningAgent.IsPlanning)
	assert.Equal(t, "Plan user auth feature", planningAgent.EpicTitle)
	assert.True(t, planningAgent.RunningFor > 0)

	// TotalTasks should only count actual tasks, not epics
	assert.Equal(t, 1, metrics.TotalTasks)
}

func TestStore_GetAgentMetrics_NoPlanningEpicLister(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// No epic lister set — should still work fine (backward compat)
	metrics, err := store.GetAgentMetrics(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, metrics.RunningAgents)
	assert.Empty(t, metrics.ActiveAgents)
}

func TestStore_GetAgentMetrics_RecentCompletionsLimit(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	now := time.Now()
	// Create 15 completed tasks
	for i := 0; i < 15; i++ {
		tsk := NewTask("repo_1", "task", "desc", nil, nil, 0, false, "sonnet", true)
		tsk.Status = StatusMerged
		tsk.UpdatedAt = now.Add(-time.Duration(i) * time.Minute)
		repo.tasks[tsk.ID.String()] = tsk
	}

	metrics, err := store.GetAgentMetrics(context.Background())
	require.NoError(t, err)

	// Should be limited to 10
	assert.Len(t, metrics.RecentCompletions, 10)
}
