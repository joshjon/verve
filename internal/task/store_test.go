package task

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/joshjon/kit/tx"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createCalls != 1 {
		t.Errorf("expected 1 create call, got %d", repo.createCalls)
	}
}

func TestStore_CreateTask_InvalidDependencyID(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", []string{"not-a-valid-id"}, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	if err == nil {
		t.Error("expected error for invalid dependency ID")
	}
}

func TestStore_CreateTask_DependencyNotFound(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = false
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	depID := NewTaskID()
	tsk := NewTask("repo_123", "title", "desc", []string{depID.String()}, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	if err == nil {
		t.Error("expected error for missing dependency")
	}
}

func TestStore_CreateTask_NotifiesPending(t *testing.T) {
	repo := newMockRepo()
	repo.taskExistsResult = true
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	err := store.CreateTask(context.Background(), tsk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The pending channel should have a notification
	select {
	case <-store.WaitForPending():
		// Good
	default:
		t.Error("expected pending notification")
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case event := <-ch:
		if event.Type != EventTaskCreated {
			t.Errorf("expected event type %s, got %s", EventTaskCreated, event.Type)
		}
		if event.RepoID != "repo_123" {
			t.Errorf("expected repo_id repo_123, got %s", event.RepoID)
		}
		if event.Task == nil {
			t.Error("expected non-nil task in event")
		}
		if event.Task.Logs != nil {
			t.Error("expected nil logs in published event")
		}
	default:
		t.Error("expected event to be published")
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Task should be failed because max attempts reached
	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed, got %s", tsk.Status)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed due to budget exceeded, got %s", tsk.Status)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Circuit breaker should trigger: same category twice
	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed due to circuit breaker, got %s", tsk.Status)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have set consecutive failures to 1 (reset for new category)
	if len(repo.setConsFails) != 1 || repo.setConsFails[0] != 1 {
		t.Errorf("expected setConsecutiveFailures(1), got %v", repo.setConsFails)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed due to budget exceeded, got %s", tsk.Status)
	}
}

func TestStore_FeedbackRetryTask_MaxAttempts(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Attempt = 5
	tsk.MaxAttempts = 5
	tsk.Status = StatusReview
	repo.tasks[tsk.ID.String()] = tsk
	repo.taskStatuses[tsk.ID.String()] = StatusReview

	err := store.FeedbackRetryTask(context.Background(), tsk.ID, "fix the tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed due to max attempts, got %s", tsk.Status)
	}
}

func TestStore_ClaimPendingTask_NoPending(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	claimed, err := store.ClaimPendingTask(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimed != nil {
		t.Error("expected nil claimed task when no pending tasks")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil claimed task")
	}
	if claimed.Status != StatusRunning {
		t.Errorf("expected status running, got %s", claimed.Status)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil claimed task")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check event published
	select {
	case event := <-ch:
		if event.Type != EventLogsAppended {
			t.Errorf("expected event type %s, got %s", EventLogsAppended, event.Type)
		}
		if len(event.Logs) != 2 {
			t.Errorf("expected 2 logs, got %d", len(event.Logs))
		}
	default:
		t.Error("expected log event to be published")
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
		t.Error("expected no pending notification initially")
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
		t.Error("expected pending notification after create")
	}
}

func TestStore_CloseTask(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.CloseTask(context.Background(), tsk.ID, "no longer needed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.Status != StatusClosed {
		t.Errorf("expected status closed, got %s", tsk.Status)
	}
	if tsk.CloseReason != "no longer needed" {
		t.Errorf("expected close reason 'no longer needed', got %s", tsk.CloseReason)
	}
}

func TestStore_UpdateTaskStatus(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	repo.tasks[tsk.ID.String()] = tsk

	err := store.UpdateTaskStatus(context.Background(), tsk.ID, StatusFailed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.Status != StatusFailed {
		t.Errorf("expected status failed, got %s", tsk.Status)
	}
}

func TestStore_SetTaskPullRequest(t *testing.T) {
	repo := newMockRepo()
	broker := NewBroker(nil)
	store := NewStore(repo, broker)

	tsk := NewTask("repo_123", "title", "desc", nil, nil, 0, false, "")
	tsk.Status = StatusRunning
	repo.tasks[tsk.ID.String()] = tsk

	err := store.SetTaskPullRequest(context.Background(), tsk.ID, "https://github.com/org/repo/pull/42", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tsk.PullRequestURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("unexpected PR URL: %s", tsk.PullRequestURL)
	}
	if tsk.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", tsk.PRNumber)
	}
}
