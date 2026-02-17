package taskapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/joshjon/kit/tx"
	"github.com/labstack/echo/v4"

	"verve/internal/repo"
	"verve/internal/setting"
	"verve/internal/task"
)

// --- Mock task repository ---

type mockTaskRepo struct {
	tasks              map[string]*task.Task
	taskStatuses       map[string]task.Status
	logs               map[string][]string
	mu                 sync.Mutex

	createTaskErr     error
	readTaskErr       error
	taskExistsResult  bool
	taskExistsErr     error
	updateStatusErr   error
	claimTaskResult   bool
	claimTaskErr      error
	closeTaskErr      error
	appendLogsErr     error
	setPullRequestErr error
	manualRetryResult bool
	manualRetryErr    error
	feedbackResult    bool
	feedbackErr       error
	hasTasksResult    bool
	hasTasksErr       error
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{
		tasks:        make(map[string]*task.Task),
		taskStatuses: make(map[string]task.Status),
		logs:         make(map[string][]string),
	}
}

func (m *mockTaskRepo) CreateTask(_ context.Context, t *task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createTaskErr != nil {
		return m.createTaskErr
	}
	m.tasks[t.ID.String()] = t
	m.taskStatuses[t.ID.String()] = t.Status
	return nil
}

func (m *mockTaskRepo) ReadTask(_ context.Context, id task.TaskID) (*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readTaskErr != nil {
		return nil, m.readTaskErr
	}
	t, ok := m.tasks[id.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockTaskRepo) ListTasks(_ context.Context) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockTaskRepo) ListTasksByRepo(_ context.Context, repoID string) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		if t.RepoID == repoID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskRepo) ListPendingTasks(_ context.Context) ([]*task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*task.Task
	for _, t := range m.tasks {
		if t.Status == task.StatusPending {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTaskRepo) ListPendingTasksByRepos(_ context.Context, repoIDs []string) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) AppendTaskLogs(_ context.Context, id task.TaskID, _ int, logs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendLogsErr != nil {
		return m.appendLogsErr
	}
	m.logs[id.String()] = append(m.logs[id.String()], logs...)
	return nil
}

func (m *mockTaskRepo) ReadTaskLogs(_ context.Context, id task.TaskID) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs[id.String()], nil
}

func (m *mockTaskRepo) StreamTaskLogs(_ context.Context, id task.TaskID, fn func(int, []string) error) error {
	return nil
}

func (m *mockTaskRepo) UpdateTaskStatus(_ context.Context, id task.TaskID, status task.Status) error {
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

func (m *mockTaskRepo) SetTaskPullRequest(_ context.Context, id task.TaskID, prURL string, prNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setPullRequestErr != nil {
		return m.setPullRequestErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.PullRequestURL = prURL
		t.PRNumber = prNumber
		t.Status = task.StatusReview
	}
	return nil
}

func (m *mockTaskRepo) ListTasksInReview(_ context.Context) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) ListTasksInReviewByRepo(_ context.Context, _ string) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) CloseTask(_ context.Context, id task.TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeTaskErr != nil {
		return m.closeTaskErr
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = task.StatusClosed
		t.CloseReason = reason
	}
	return nil
}

func (m *mockTaskRepo) TaskExists(_ context.Context, _ task.TaskID) (bool, error) {
	return m.taskExistsResult, m.taskExistsErr
}

func (m *mockTaskRepo) ReadTaskStatus(_ context.Context, id task.TaskID) (task.Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.taskStatuses[id.String()]
	if !ok {
		return "", errors.New("not found")
	}
	return s, nil
}

func (m *mockTaskRepo) ClaimTask(_ context.Context, id task.TaskID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimTaskErr != nil {
		return false, m.claimTaskErr
	}
	if !m.claimTaskResult {
		return false, nil
	}
	if t, ok := m.tasks[id.String()]; ok {
		t.Status = task.StatusRunning
	}
	return true, nil
}

func (m *mockTaskRepo) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasksResult, m.hasTasksErr
}

func (m *mockTaskRepo) RetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return false, nil
}

func (m *mockTaskRepo) SetAgentStatus(_ context.Context, _ task.TaskID, _ string) error {
	return nil
}

func (m *mockTaskRepo) SetRetryContext(_ context.Context, _ task.TaskID, _ string) error {
	return nil
}

func (m *mockTaskRepo) AddCost(_ context.Context, _ task.TaskID, _ float64) error {
	return nil
}

func (m *mockTaskRepo) SetConsecutiveFailures(_ context.Context, _ task.TaskID, _ int) error {
	return nil
}

func (m *mockTaskRepo) SetCloseReason(_ context.Context, id task.TaskID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.CloseReason = reason
	}
	return nil
}

func (m *mockTaskRepo) SetBranchName(_ context.Context, id task.TaskID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id.String()]; ok {
		t.BranchName = name
	}
	return nil
}

func (m *mockTaskRepo) ListTasksInReviewNoPR(_ context.Context) ([]*task.Task, error) {
	return nil, nil
}

func (m *mockTaskRepo) ManualRetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return m.manualRetryResult, m.manualRetryErr
}

func (m *mockTaskRepo) FeedbackRetryTask(_ context.Context, _ task.TaskID, _ string) (bool, error) {
	return m.feedbackResult, m.feedbackErr
}

func (m *mockTaskRepo) DeleteTaskLogs(_ context.Context, id task.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.logs, id.String())
	return nil
}

func (m *mockTaskRepo) BeginTxFunc(ctx context.Context, fn func(context.Context, tx.Tx, task.Repository) error) error {
	return fn(ctx, nil, m)
}

func (m *mockTaskRepo) WithTx(_ tx.Tx) task.Repository {
	return m
}

// --- Mock repo repository ---

type mockRepoRepo struct {
	repos map[string]*repo.Repo
}

func newMockRepoRepo() *mockRepoRepo {
	return &mockRepoRepo{repos: make(map[string]*repo.Repo)}
}

func (m *mockRepoRepo) CreateRepo(_ context.Context, r *repo.Repo) error {
	m.repos[r.ID.String()] = r
	return nil
}

func (m *mockRepoRepo) ReadRepo(_ context.Context, id repo.RepoID) (*repo.Repo, error) {
	r, ok := m.repos[id.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRepoRepo) ReadRepoByFullName(_ context.Context, fullName string) (*repo.Repo, error) {
	for _, r := range m.repos {
		if r.FullName == fullName {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockRepoRepo) ListRepos(_ context.Context) ([]*repo.Repo, error) {
	var result []*repo.Repo
	for _, r := range m.repos {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRepoRepo) DeleteRepo(_ context.Context, id repo.RepoID) error {
	delete(m.repos, id.String())
	return nil
}

// --- Mock setting repository ---

type mockSettingRepo struct {
	settings map[string]string
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{settings: make(map[string]string)}
}

func (m *mockSettingRepo) UpsertSetting(_ context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockSettingRepo) ReadSetting(_ context.Context, key string) (string, error) {
	v, ok := m.settings[key]
	if !ok {
		return "", setting.ErrNotFound
	}
	return v, nil
}

func (m *mockSettingRepo) DeleteSetting(_ context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

func (m *mockSettingRepo) ListSettings(_ context.Context) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range m.settings {
		result[k] = v
	}
	return result, nil
}

// --- Mock task checker for repo store ---

type mockTaskChecker struct {
	hasTasks bool
}

func (m *mockTaskChecker) HasTasksForRepo(_ context.Context, _ string) (bool, error) {
	return m.hasTasks, nil
}

// --- Test helpers ---

func setupHandler() (*HTTPHandler, *mockTaskRepo, *mockRepoRepo, *repo.Repo) {
	taskRepo := newMockTaskRepo()
	broker := task.NewBroker(nil)
	taskStore := task.NewStore(taskRepo, broker)

	repoRepo := newMockRepoRepo()
	checker := &mockTaskChecker{}
	repoStore := repo.NewStore(repoRepo, checker)

	settingRepo := newMockSettingRepo()
	settingService := setting.NewService(settingRepo)

	handler := NewHTTPHandler(taskStore, repoStore, nil, settingService)

	// Pre-create a repo for use in tests
	r, _ := repo.NewRepo("owner/test-repo")
	_ = repoStore.CreateRepo(context.Background(), r)

	return handler, taskRepo, repoRepo, r
}

func newContext(e *echo.Echo, method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

// --- Tests ---

func TestCreateTask_Success(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug","description":"Fix the login bug"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var created task.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Title != "Fix bug" {
		t.Errorf("expected title 'Fix bug', got %s", created.Title)
	}
	if created.Status != task.StatusPending {
		t.Errorf("expected status pending, got %s", created.Status)
	}
	if created.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %s", created.Model)
	}
}

func TestCreateTask_EmptyTitle(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"","description":"some desc"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateTask_TitleTooLong(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	longTitle := strings.Repeat("a", 151)
	body := `{"title":"` + longTitle + `","description":"desc"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for long title, got %d", rec.Code)
	}
}

func TestCreateTask_InvalidRepoID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug"}`
	c, rec := newContext(e, http.MethodPost, "/repos/invalid/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues("invalid")

	err := handler.CreateTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateTask_WithModel(t *testing.T) {
	handler, _, _, testRepo := setupHandler()
	e := echo.New()

	body := `{"title":"Fix bug","description":"desc","model":"opus"}`
	c, rec := newContext(e, http.MethodPost, "/repos/"+testRepo.ID.String()+"/tasks", body)
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.CreateTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var created task.Task
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Model != "opus" {
		t.Errorf("expected model 'opus', got %s", created.Model)
	}
}

func TestGetTask_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	c, rec := newContext(e, http.MethodGet, "/tasks/"+tsk.ID.String(), "")
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.GetTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var result task.Task
	json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Title != "title" {
		t.Errorf("expected title 'title', got %s", result.Title)
	}
}

func TestGetTask_InvalidID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/tasks/invalid", "")
	c.SetParamNames("id")
	c.SetParamValues("invalid")

	err := handler.GetTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAppendLogs_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"logs":["line 1","line 2"],"attempt":1}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/logs", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.AppendLogs(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAppendLogs_DefaultAttempt(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"logs":["line 1"]}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/logs", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.AppendLogs(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestCompleteTask_Failure(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"error":"exit code 1"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if tsk.Status != task.StatusFailed {
		t.Errorf("expected status failed, got %s", tsk.Status)
	}
}

func TestCompleteTask_SuccessWithPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true,"pull_request_url":"https://github.com/org/repo/pull/42","pr_number":42}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if tsk.Status != task.StatusReview {
		t.Errorf("expected status review, got %s", tsk.Status)
	}
	if tsk.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", tsk.PRNumber)
	}
}

func TestCompleteTask_SuccessWithBranch(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true,"branch_name":"verve/task-tsk_123"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if tsk.BranchName != "verve/task-tsk_123" {
		t.Errorf("expected branch name, got %s", tsk.BranchName)
	}
}

func TestCompleteTask_SuccessNoPR_ClosedIfNoExistingPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if tsk.Status != task.StatusClosed {
		t.Errorf("expected status closed (no PR), got %s", tsk.Status)
	}
}

func TestCompleteTask_SuccessNoPR_ReviewIfExistingPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	tsk.PRNumber = 10
	tsk.PullRequestURL = "https://github.com/org/repo/pull/10"
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":true}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if tsk.Status != task.StatusReview {
		t.Errorf("expected status review (existing PR), got %s", tsk.Status)
	}
}

func TestCloseTask_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"reason":"no longer needed"}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/close", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CloseTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if tsk.Status != task.StatusClosed {
		t.Errorf("expected status closed, got %s", tsk.Status)
	}
}

func TestListTasksByRepo_Success(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk1 := task.NewTask(testRepo.ID.String(), "task 1", "desc", nil, nil, 0, false, "sonnet")
	tsk2 := task.NewTask(testRepo.ID.String(), "task 2", "desc", nil, nil, 0, false, "sonnet")
	taskRepo.tasks[tsk1.ID.String()] = tsk1
	taskRepo.tasks[tsk2.ID.String()] = tsk2

	c, rec := newContext(e, http.MethodGet, "/repos/"+testRepo.ID.String()+"/tasks", "")
	c.SetParamNames("repo_id")
	c.SetParamValues(testRepo.ID.String())

	err := handler.ListTasksByRepo(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var tasks []*task.Task
	json.Unmarshal(rec.Body.Bytes(), &tasks)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestAddRepo_Success(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":"newowner/newrepo"}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var r repo.Repo
	json.Unmarshal(rec.Body.Bytes(), &r)
	if r.FullName != "newowner/newrepo" {
		t.Errorf("expected full_name 'newowner/newrepo', got %s", r.FullName)
	}
}

func TestAddRepo_EmptyFullName(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":""}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAddRepo_InvalidFullName(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"full_name":"noslash"}`
	c, rec := newContext(e, http.MethodPost, "/repos", body)

	err := handler.AddRepo(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestListRepos(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/repos", "")

	err := handler.ListRepos(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestGetDefaultModel_Default(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/settings/default-model", "")

	err := handler.GetDefaultModel(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp DefaultModelResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %s", resp.Model)
	}
}

func TestSaveDefaultModel(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"model":"opus"}`
	c, rec := newContext(e, http.MethodPut, "/settings/default-model", body)

	err := handler.SaveDefaultModel(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestSaveDefaultModel_EmptyModel(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"model":""}`
	c, rec := newContext(e, http.MethodPut, "/settings/default-model", body)

	err := handler.SaveDefaultModel(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestGetGitHubTokenStatus_NotConfigured(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/settings/github-token", "")

	err := handler.GetGitHubTokenStatus(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp GitHubTokenStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Configured {
		t.Error("expected configured=false when no token service")
	}
}

func TestSaveGitHubToken_NoService(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	body := `{"token":"ghp_test"}`
	c, rec := newContext(e, http.MethodPut, "/settings/github-token", body)

	err := handler.SaveGitHubToken(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestGetTaskChecks_NoPR(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.PRNumber = 0
	taskRepo.tasks[tsk.ID.String()] = tsk

	c, rec := newContext(e, http.MethodGet, "/tasks/"+tsk.ID.String()+"/checks", "")
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.GetTaskChecks(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp CheckStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "success" {
		t.Errorf("expected status 'success' for no CI, got %s", resp.Status)
	}
}

func TestFeedbackTask_EmptyFeedback(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusReview
	taskRepo.tasks[tsk.ID.String()] = tsk

	body := `{"feedback":""}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/feedback", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.FeedbackTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty feedback, got %d", rec.Code)
	}
}

func TestRemoveRepo_InvalidID(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodDelete, "/repos/invalid", "")
	c.SetParamNames("repo_id")
	c.SetParamValues("invalid")

	err := handler.RemoveRepo(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := errorResponse("test error")
	if resp["error"] != "test error" {
		t.Errorf("expected 'test error', got %s", resp["error"])
	}
}

func TestStatusOK(t *testing.T) {
	resp := statusOK()
	if resp["status"] != "ok" {
		t.Errorf("expected 'ok', got %s", resp["status"])
	}
}

func TestListAvailableRepos_NoGitHubClient(t *testing.T) {
	handler, _, _, _ := setupHandler()
	e := echo.New()

	c, rec := newContext(e, http.MethodGet, "/repos/available", "")

	err := handler.ListAvailableRepos(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestCompleteTask_WithAgentStatus(t *testing.T) {
	handler, taskRepo, _, testRepo := setupHandler()
	e := echo.New()

	tsk := task.NewTask(testRepo.ID.String(), "title", "desc", nil, nil, 0, false, "sonnet")
	tsk.Status = task.StatusRunning
	taskRepo.tasks[tsk.ID.String()] = tsk
	taskRepo.taskStatuses[tsk.ID.String()] = tsk.Status

	body := `{"success":false,"agent_status":"{\"confidence\":\"high\"}","cost_usd":1.5}`
	c, rec := newContext(e, http.MethodPost, "/tasks/"+tsk.ID.String()+"/complete", body)
	c.SetParamNames("id")
	c.SetParamValues(tsk.ID.String())

	err := handler.CompleteTask(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
