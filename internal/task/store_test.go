package task

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/joshjon/kit/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements the Repository interface for testing.
type mockRepository struct {
	tasks              map[string]*Task
	taskStatuses       map[string]Status
	logs               map[string][]string
	consecutiveFailMap map[string]int
	mu                 sync.Mutex

	createTaskErr          error
	readTaskErr            error
	taskExistsResult       bool
	taskExistsErr          error
	updateStatusErr        error
	retryTaskResult        bool
	retryTaskErr           error
	manualRetryTaskResult  bool
	manualRetryTaskErr     error
	feedbackRetryResult    bool
	feedbackRetryErr       error
	claimTaskResult        bool
	claimTaskErr           error
	closeTaskErr           error
	setAgentStatusErr      error
	setRetryContextErr     error
	addCostErr             error
	setConsFailErr         error
	setCloseReasonErr      error
	setBranchNameErr       error
	appendLogsErr          error
	setPullRequestErr      error
	hasTasksForRepoResult  bool
	hasTasksForRepoErr     error

	// Track calls
	createCalls       int
	retryTaskCalls    int
	setConsFails      []int
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		tasks:              make(map[string]*Task),
		taskStatuses:       make(map[string]Status),
		logs:               make(map[string][]string),
		consecutiveFailMap: make(map[string]int),
	}
}

func (m *mockRepository) CreateTask(_ context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	if m.createTaskErr != nil {
		return m.createTaskErr
	}
	m.tasks[task.ID.String()] = task
	m.taskStatuses[task.ID.String()] = task.Status
	return nil
}

func (m *mockRepository) ReadTask(_ context.Context, id TaskID) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readTaskErr != nil {
		return nil, m.readTaskErr
	}
	t, ok := m.tasks[id.String()]
	if !ok {
		return nil, errors.New("task not found")
	}
	return t, nil
}

func (m *mockRepository) ListTasks(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockRepository) ListTasksByRepo(_ context.Context, repoID string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListPendingTasks(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusPending {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListPendingTasksByRepos(_ context.Context, repoIDs []string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	repoSet := make(map[string]bool)
	for _, id := range repoIDs {
		repoSet[id] = true
	}
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusPending && repoSet[t.RepoID] {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) AppendTaskLogs(_ context.Context, id TaskID, _ int, logs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendLogsErr != nil {
		return m.appendLogsErr
	}
	m.logs[id.String()] = append(m.logs[id.String()], logs...)
	return nil
}

func (m *mockRepository) ReadTaskLogs(_ context.Context, id TaskID) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs[id.String()], nil
}

func (m *mockRepository) StreamTaskLogs(_ context.Context, id TaskID, fn func(attempt int, lines []string) error) error {
	m.mu.Lock()
	logs := m.logs[id.String()]
	m.mu.Unlock()
	if len(logs) > 0 {
		return fn(1, logs)
	}
	return nil
}

func (m *mockRepository) UpdateTaskStatus(_ context.Context, id TaskID, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = status
	}
	m.taskStatuses[id.String()] = status
	return nil
}

func (m *mockRepository) SetTaskPullRequest(_ context.Context, id TaskID, prURL string, prNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setPullRequestErr != nil {
		return m.setPullRequestErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.PullRequestURL = prURL
		t.PRNumber = prNumber
		t.Status = StatusReview
	}
	return nil
}

func (m *mockRepository) ListTasksInReview(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ListTasksInReviewByRepo(_ context.Context, repoID string) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview && t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) CloseTask(_ context.Context, id TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeTaskErr != nil {
		return m.closeTaskErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = StatusClosed
		t.CloseReason = reason
	}
	return nil
}

func (m *mockRepository) TaskExists(_ context.Context, _ TaskID) (bool, error) {
	return m.taskExistsResult, m.taskExistsErr
}

func (m *mockRepository) ReadTaskStatus(_ context.Context, id TaskID) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.taskStatuses[id.String()]
	if !ok {
		return "", errors.New("not found")
	}
	return status, nil
}

func (m *mockRepository) ClaimTask(_ context.Context, id TaskID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimTaskErr != nil {
		return false, m.claimTaskErr
	}
	if !m.claimTaskResult {
		return false, nil
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = StatusRunning
	}
	return true, nil
}

func (m *mockRepository) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasksForRepoResult, m.hasTasksForRepoErr
}

func (m *mockRepository) RetryTask(_ context.Context, _ TaskID, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryTaskCalls++
	return m.retryTaskResult, m.retryTaskErr
}

func (m *mockRepository) SetAgentStatus(_ context.Context, _ TaskID, _ string) error {
	return m.setAgentStatusErr
}

func (m *mockRepository) SetRetryContext(_ context.Context, _ TaskID, _ string) error {
	return m.setRetryContextErr
}

func (m *mockRepository) AddCost(_ context.Context, _ TaskID, _ float64) error {
	return m.addCostErr
}

func (m *mockRepository) SetConsecutiveFailures(_ context.Context, _ TaskID, count int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setConsFails = append(m.setConsFails, count)
	return m.setConsFailErr
}

func (m *mockRepository) SetCloseReason(_ context.Context, id TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setCloseReasonErr != nil {
		return m.setCloseReasonErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.CloseReason = reason
	}
	return nil
}

func (m *mockRepository) SetBranchName(_ context.Context, id TaskID, branchName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setBranchNameErr != nil {
		return m.setBranchNameErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.BranchName = branchName
	}
	return nil
}

func (m *mockRepository) ListTasksInReviewNoPR(_ context.Context) ([]*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Task
	for _, t := range m.tasks {
		if t.Status == StatusReview && t.PRNumber == 0 && t.BranchName != "" {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockRepository) ManualRetryTask(_ context.Context, _ TaskID, _ string) (bool, error) {
	return m.manualRetryTaskResult, m.manualRetryTaskErr
}

func (m *mockRepository) FeedbackRetryTask(_ context.Context, _ TaskID, _ string) (bool, error) {
	return m.feedbackRetryResult, m.feedbackRetryErr
}

func (m *mockRepository) DeleteTaskLogs(_ context.Context, id TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.logs, id.String())
	return nil
}

func (m *mockRepository) RemoveDependency(_ context.Context, id TaskID, depID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id.String()]
	if !ok {
		return errors.New("task not found")
	}
	filtered := make([]string, 0, len(t.DependsOn))
	for _, d := range t.DependsOn {
		if d != depID {
			filtered = append(filtered, d)
		}
	}
	t.DependsOn = filtered
	return nil
}

func (m *mockRepository) BeginTxFunc(ctx context.Context, fn func(context.Context, tx.Tx, Repository) error) error {
	return fn(ctx, nil, m)
}

func (m *mockRepository) WithTx(_ tx.Tx) Repository {
	return m
}

// --- Store tests ---

func TestStore_CreateTask_Success(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "sonnet")
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.createCalls)
}

func TestStore_CreateTask_InvalidDependencyID(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", []string{"not-a-valid-id"}, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_CreateTask_DependencyNotFound(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = false
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	depID := NewTaskID()
	tsk := NewTask("repo_123", "title", "desc", []string{depID.String()}, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	assert.Error(t, err, "expected error for missing dependency")
}

func TestStore_CreateTask_NotifiesPending(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	// The pending channel should have a notification
	select {
	case <-store.WaitForPending():
		// Good
	default:
		assert.Fail(t, "expected pending notification")
	}
}

func TestStore_CreateTask_PublishesEvent(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	require.NoError(t, err)

	select {
	case event := <-ch:
		assert.Equal(t, EventTaskCreated, event.Type)
		assert.Equal(t, "repo_123", event.RepoID)
		assert.NotNil(t, event.Task, "expected non-nil task in event")
		assert.Nil(t, event.Task.Logs, "expected nil logs in published event")
	default:
		assert.Fail(t, "expected event to be published")
	}
}

func TestStore_RetryTask_MaxAttempts(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed")
	require.NoError(t, err)

	// Task should be failed because max attempts reached
	assert.Equal(t, StatusFailed, tsk.Status)
}

func TestStore_RetryTask_BudgetExceeded(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, "")
	tsk.CostUSD = 6.0
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "", "some reason")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to budget exceeded")
}

func TestStore_RetryTask_CircuitBreaker(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 1
	tsk.RetryReason = "ci_failure:tests: CI tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.RetryTask(context.Background(), tsk.ID, "ci_failure:tests", "CI tests failed again")
	require.NoError(t, err)

	// Circuit breaker should trigger: same category twice
	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to circuit breaker")
}

func TestStore_RetryTask_DifferentCategory(t *testing.T) {
	repo := newMockRepo()
	repo.retryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusReview
	tsk.ConsecutiveFailures = 1
	tsk.RetryReason = "ci_failure:tests: CI tests failed"
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	// Different category should NOT trip circuit breaker
	err := store.RetryTask(context.Background(), tsk.ID, "merge_conflict", "merge conflict detected")
	require.NoError(t, err)

	// Should have set consecutive failures to 1 (reset for new category)
	require.Len(t, repo.setConsFails, 1)
	assert.Equal(t, 1, repo.setConsFails[0])
}

func TestStore_ManualRetryTask(t *testing.T) {
	repo := newMockRepo()
	repo.manualRetryTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusFailed
	repo.tasks[tsk.ID.String()] = tsk

	err := store.ManualRetryTask(context.Background(), tsk.ID, "try again please")
	require.NoError(t, err)
}

func TestStore_ManualRetryTask_NotFailed(t *testing.T) {
	repo := newMockRepo()
	repo.manualRetryTaskResult = false
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk

	err := store.ManualRetryTask(context.Background(), tsk.ID, "")
	require.NoError(t, err)
	// Should be a no-op
}

func TestStore_FeedbackRetryTask_BudgetExceeded(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 5.0, false, "")
	tsk.CostUSD = 6.0
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "fix the tests")
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status, "expected status failed due to budget exceeded")
}

func TestStore_FeedbackRetryTask_IgnoresMaxAttempts(t *testing.T) {
	repo := newMockRepo()
	repo.feedbackRetryResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "fix the tests")
	require.NoError(t, err)

	// Feedback retries should NOT be blocked by max attempts since they
	// represent user-driven iteration, not failure recovery.
	assert.NotEqual(t, StatusFailed, tsk.Status, "feedback should not fail task at max attempts")
}

func TestStore_ClaimPendingTask_NoPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	claimed, err := store.ClaimPendingTask(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, claimed, "expected nil claimed task when no pending tasks")
}

func TestStore_ClaimPendingTask_Success(t *testing.T) {
	repo := newMockRepo()
	repo.claimTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusPending

	claimed, err := store.ClaimPendingTask(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
	assert.Equal(t, StatusRunning, claimed.Status)
}

func TestStore_ClaimPendingTask_WithRepoFilter(t *testing.T) {
	repo := newMockRepo()
	repo.claimTaskResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusPending

	claimed, err := store.ClaimPendingTask(context.Background(), []string{"repo_123"})
	require.NoError(t, err)
	require.NotNil(t, claimed, "expected non-nil claimed task")
}

func TestStore_AppendTaskLogs(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.AppendTaskLogs(context.Background(), tsk.ID, 1, []string{"line 1", "line 2"})
	require.NoError(t, err)

	// Check event published
	select {
	case event := <-ch:
		assert.Equal(t, EventLogsAppended, event.Type)
		assert.Len(t, event.Logs, 2)
	default:
		assert.Fail(t, "expected log event to be published")
	}
}

func TestStore_WaitForPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	// Initially no notification
	ch := store.WaitForPending()
	select {
	case <-ch:
		assert.Fail(t, "expected no pending notification initially")
	default:
		// Good
	}

	// Create a task to trigger notification
	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	_ = store.CreateTask(context.Background(), tsk)

	select {
	case <-ch:
		// Good
	default:
		assert.Fail(t, "expected pending notification after create")
	}
}

func TestStore_CloseTask(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.CloseTask(context.Background(), tsk.ID, "no longer needed")
	require.NoError(t, err)

	assert.Equal(t, StatusClosed, tsk.Status)
	assert.Equal(t, "no longer needed", tsk.CloseReason)
}

func TestStore_UpdateTaskStatus(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.UpdateTaskStatus(context.Background(), tsk.ID, StatusFailed)
	require.NoError(t, err)

	assert.Equal(t, StatusFailed, tsk.Status)
}

func TestStore_SetTaskPullRequest(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk

	err := store.SetTaskPullRequest(context.Background(), tsk.ID, "https://github.com/org/repo/pull/42", 42)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/org/repo/pull/42", tsk.PullRequestURL)
	assert.Equal(t, 42, tsk.PRNumber)
}

func TestStore_RemoveDependency_Success(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	dep := NewTask("repo_123", "dep", "dep desc", nil, nil, 0, false, "")
	repo.tasks[dep.ID.String()] = dep

	tsk := NewTask("repo_123", "title", "desc", []string{dep.ID.String()}, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.RemoveDependency(context.Background(), tsk.ID, dep.ID.String())
	require.NoError(t, err)
	assert.Empty(t, tsk.DependsOn)
}

func TestStore_RemoveDependency_InvalidDepID(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.RemoveDependency(context.Background(), tsk.ID, "not-a-valid-id")
	assert.Error(t, err, "expected error for invalid dependency ID")
}

func TestStore_RemoveDependency_NotifiesPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	dep := NewTask("repo_123", "dep", "dep desc", nil, nil, 0, false, "")
	repo.tasks[dep.ID.String()] = dep

	tsk := NewTask("repo_123", "title", "desc", []string{dep.ID.String()}, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	// Drain any existing notification
	select {
	case <-store.WaitForPending():
	default:
	}

	err := store.RemoveDependency(context.Background(), tsk.ID, dep.ID.String())
	require.NoError(t, err)

	select {
	case <-store.WaitForPending():
		// Good — removing a dependency may unblock the task
	default:
		assert.Fail(t, "expected pending notification after removing dependency")
	}
}
