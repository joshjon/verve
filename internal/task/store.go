package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joshjon/kit/tx"
)

// Store wraps a Repository and adds application-level concerns such as
// pending task notification, dependency validation, and event broadcasting.
type Store struct {
	repo      Repository
	broker    *Broker
	pendingMu sync.Mutex
	pendingCh chan struct{}
}

// NewStore creates a new Store backed by the given Repository and Broker.
func NewStore(repo Repository, broker *Broker) *Store {
	return &Store{
		repo:      repo,
		broker:    broker,
		pendingCh: make(chan struct{}, 1),
	}
}

// Subscribe returns a channel that receives task events.
func (s *Store) Subscribe() chan Event {
	return s.broker.Subscribe()
}

// Unsubscribe removes and closes a subscriber channel.
func (s *Store) Unsubscribe(ch chan Event) {
	s.broker.Unsubscribe(ch)
}

// CreateTask validates dependencies and creates a new task.
func (s *Store) CreateTask(ctx context.Context, task *Task) error {
	// Validate all dependencies exist
	for _, depID := range task.DependsOn {
		taskID, err := ParseTaskID(depID)
		if err != nil {
			return fmt.Errorf("invalid dependency ID %q: %w", depID, err)
		}
		exists, err := s.repo.TaskExists(ctx, taskID)
		if err != nil {
			return fmt.Errorf("check dependency %q: %w", depID, err)
		}
		if !exists {
			return fmt.Errorf("dependency task not found: %s", depID)
		}
	}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return err
	}
	if task.Ready {
		s.notifyPending()
	}

	t := *task
	t.Logs = nil
	s.broker.Publish(ctx, Event{Type: EventTaskCreated, RepoID: task.RepoID, Task: &t})
	return nil
}

// CreateTaskFromEpic creates a task associated with an epic. Dependencies
// are not validated since they are created in the same batch.
func (s *Store) CreateTaskFromEpic(ctx context.Context, repoID, title, description string, dependsOn, acceptanceCriteria []string, epicID string, ready bool) (string, error) {
	if dependsOn == nil {
		dependsOn = []string{}
	}
	if acceptanceCriteria == nil {
		acceptanceCriteria = []string{}
	}
	t := NewTask(repoID, title, description, dependsOn, acceptanceCriteria, 0, false, "sonnet", ready)
	t.EpicID = epicID
	if err := s.repo.CreateTask(ctx, t); err != nil {
		return "", err
	}
	if ready {
		s.notifyPending()
	}
	tc := *t
	tc.Logs = nil
	s.broker.Publish(ctx, Event{Type: EventTaskCreated, RepoID: t.RepoID, Task: &tc})
	return t.ID.String(), nil
}

// ReadTask reads a task by ID.
func (s *Store) ReadTask(ctx context.Context, id TaskID) (*Task, error) {
	return s.repo.ReadTask(ctx, id)
}

// ListTasks returns all tasks.
func (s *Store) ListTasks(ctx context.Context) ([]*Task, error) {
	return s.repo.ListTasks(ctx)
}

// ListTasksByRepo returns all tasks for a given repo.
func (s *Store) ListTasksByRepo(ctx context.Context, repoID string) ([]*Task, error) {
	return s.repo.ListTasksByRepo(ctx, repoID)
}

// ListTasksInReviewByRepo returns tasks in review status for a given repo.
func (s *Store) ListTasksInReviewByRepo(ctx context.Context, repoID string) ([]*Task, error) {
	return s.repo.ListTasksInReviewByRepo(ctx, repoID)
}

// HasTasksForRepo checks whether any tasks exist for a given repo.
func (s *Store) HasTasksForRepo(ctx context.Context, repoID string) (bool, error) {
	return s.repo.HasTasksForRepo(ctx, repoID)
}

// RetryTask transitions a task from review back to pending for another attempt.
// category classifies the failure type (e.g. "ci_failure:tests", "merge_conflict")
// for circuit breaker detection. If the same category fails consecutively
// >= 2 times, the task is failed immediately. Categories include the specific
// failed check names so that different CI failures don't trip the breaker.
func (s *Store) RetryTask(ctx context.Context, id TaskID, category, reason string) error {
	t, err := s.repo.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	// Budget check: fail if cost exceeds max
	if t.MaxCostUSD > 0 && t.CostUSD >= t.MaxCostUSD {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	if t.Attempt >= t.MaxAttempts {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	// Circuit breaker: detect consecutive same-category failures.
	// The category must match exactly (e.g. "ci_failure:tests" only
	// matches "ci_failure:tests", not "ci_failure:changelog").
	consecutiveFailures := 1
	if category != "" && strings.HasPrefix(t.RetryReason, category+":") {
		consecutiveFailures = t.ConsecutiveFailures + 1
	}

	if consecutiveFailures >= 2 {
		// Same failure type twice in a row — fail fast
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	// Update consecutive failure count
	if err := s.repo.SetConsecutiveFailures(ctx, id, consecutiveFailures); err != nil {
		return err
	}

	ok, err := s.repo.RetryTask(ctx, id, reason)
	if err != nil {
		return err
	}
	if !ok {
		return nil // task was not in review status
	}

	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return nil
}

// ScheduleRetry transitions a running task back to pending for another attempt.
// This is used when the agent hits a retryable error such as Claude rate limits
// or session max usage exceeded. The task keeps its existing PR/branch info so
// the next attempt can continue where the previous one left off.
func (s *Store) ScheduleRetry(ctx context.Context, id TaskID, reason string) error {
	t, err := s.repo.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	// Budget check: fail if cost exceeds max
	if t.MaxCostUSD > 0 && t.CostUSD >= t.MaxCostUSD {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	if t.Attempt >= t.MaxAttempts {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	// Circuit breaker: same retryable error twice in a row → fail
	consecutiveFailures := 1
	if t.RetryReason == reason {
		consecutiveFailures = t.ConsecutiveFailures + 1
	}
	if consecutiveFailures >= 3 {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	if err := s.repo.SetConsecutiveFailures(ctx, id, consecutiveFailures); err != nil {
		return err
	}

	ok, err := s.repo.ScheduleRetryFromRunning(ctx, id, reason)
	if err != nil {
		return err
	}
	if !ok {
		return nil // task was not in running status
	}

	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return nil
}

// ManualRetryTask transitions a failed task back to pending for another attempt.
// instructions contains optional guidance for the agent on the retry.
// Previous attempt logs are preserved — the UI shows them in separate tabs.
func (s *Store) ManualRetryTask(ctx context.Context, id TaskID, instructions string) error {
	ok, err := s.repo.ManualRetryTask(ctx, id, instructions)
	if err != nil {
		return err
	}
	if !ok {
		return nil // task was not in failed status
	}

	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return nil
}

// FeedbackRetryTask transitions a task in review back to pending so the agent
// can iterate on its solution based on the user's feedback. Unlike ManualRetryTask,
// it preserves the existing PR/branch so the agent pushes fixes to the same branch.
//
// Feedback retries (manual change requests) do not count towards the max retry
// attempts because they represent user-driven iteration rather than failure recovery.
// Both attempt and max_attempts are incremented so the attempt number is unique
// for log tabbing while keeping the retry budget unchanged.
func (s *Store) FeedbackRetryTask(ctx context.Context, id TaskID, feedback string) error {
	t, err := s.repo.ReadTask(ctx, id)
	if err != nil {
		return err
	}

	// Budget check
	if t.MaxCostUSD > 0 && t.CostUSD >= t.MaxCostUSD {
		return s.UpdateTaskStatus(ctx, id, StatusFailed)
	}

	ok, err := s.repo.FeedbackRetryTask(ctx, id, feedback)
	if err != nil {
		return err
	}
	if !ok {
		return nil // task was not in review status
	}

	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return nil
}

// SetAgentStatus stores the structured agent status JSON.
func (s *Store) SetAgentStatus(ctx context.Context, id TaskID, status string) error {
	if err := s.repo.SetAgentStatus(ctx, id, status); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// SetRetryContext stores detailed failure context (e.g. CI logs) for retries.
func (s *Store) SetRetryContext(ctx context.Context, id TaskID, retryCtx string) error {
	return s.repo.SetRetryContext(ctx, id, retryCtx)
}

// AddCost adds to the accumulated cost for a task.
func (s *Store) AddCost(ctx context.Context, id TaskID, costUSD float64) error {
	return s.repo.AddCost(ctx, id, costUSD)
}

// SetCloseReason sets the close/failure reason on a task without changing its status.
func (s *Store) SetCloseReason(ctx context.Context, id TaskID, reason string) error {
	if err := s.repo.SetCloseReason(ctx, id, reason); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// RemoveDependency removes a dependency from a task. The dependency ID must be
// a valid task ID. After removal, if the task is pending, a pending notification
// is sent in case the task is now unblocked.
func (s *Store) RemoveDependency(ctx context.Context, id TaskID, depID string) error {
	if _, err := ParseTaskID(depID); err != nil {
		return fmt.Errorf("invalid dependency ID %q: %w", depID, err)
	}
	if err := s.repo.RemoveDependency(ctx, id, depID); err != nil {
		return err
	}
	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return nil
}

// SetReady updates the ready flag on a task. When a task is marked as ready and
// is pending, a notification is sent so workers can pick it up.
func (s *Store) SetReady(ctx context.Context, id TaskID, ready bool) error {
	if err := s.repo.SetReady(ctx, id, ready); err != nil {
		return err
	}
	if ready {
		s.notifyPending()
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// UpdatePendingTask updates a pending task's editable fields. If the task is no
// longer in pending status, the update is rejected with a conflict error.
func (s *Store) UpdatePendingTask(ctx context.Context, id TaskID, params UpdatePendingTaskParams) error {
	// Validate all dependencies exist
	for _, depID := range params.DependsOn {
		taskID, err := ParseTaskID(depID)
		if err != nil {
			return fmt.Errorf("invalid dependency ID %q: %w", depID, err)
		}
		exists, err := s.repo.TaskExists(ctx, taskID)
		if err != nil {
			return fmt.Errorf("check dependency %q: %w", depID, err)
		}
		if !exists {
			return fmt.Errorf("dependency task not found: %s", depID)
		}
	}

	ok, err := s.repo.UpdatePendingTask(ctx, id, params)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTaskNotPending
	}
	if params.Ready {
		s.notifyPending()
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// StartOverTask resets a task from review or failed back to pending, clearing
// all metadata (logs, PR, branch, agent status, cost) and optionally updating
// the task details (title, description, acceptance criteria). Returns the task
// before reset so the caller can close the PR if needed.
func (s *Store) StartOverTask(ctx context.Context, id TaskID, params StartOverTaskParams) (*Task, error) {
	// Read the task before reset so we can return PR info for cleanup.
	t, err := s.repo.ReadTask(ctx, id)
	if err != nil {
		return nil, err
	}

	ok, err := s.repo.StartOverTask(ctx, id, params)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil // task was not in review or failed status
	}

	// Delete all logs for a clean slate.
	if err := s.repo.DeleteTaskLogs(ctx, id); err != nil {
		return nil, err
	}

	s.notifyPending()
	s.publishTaskUpdated(ctx, id)
	return t, nil
}

// ClaimPendingTask finds a pending task with all dependencies met and claims it
// by setting its status to running. When repoIDs is non-empty, only tasks
// belonging to those repos are considered. The read-check-claim flow is wrapped
// in a transaction and uses optimistic locking (WHERE status = 'pending') so
// that concurrent workers cannot claim the same task.
func (s *Store) ClaimPendingTask(ctx context.Context, repoIDs []string) (*Task, error) {
	var claimed *Task
	err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
		var pending []*Task
		var err error
		if len(repoIDs) > 0 {
			pending, err = repo.ListPendingTasksByRepos(ctx, repoIDs)
		} else {
			pending, err = repo.ListPendingTasks(ctx)
		}
		if err != nil {
			return err
		}
		for _, t := range pending {
			if !dependenciesMet(ctx, repo, t.DependsOn) {
				continue
			}
			ok, err := repo.ClaimTask(ctx, t.ID)
			if err != nil {
				return err
			}
			if !ok {
				continue // Already claimed by another worker
			}
			t.Status = StatusRunning
			claimed = t
			return nil
		}
		return nil
	})
	if err == nil && claimed != nil {
		t := *claimed
		t.Logs = nil
		s.broker.Publish(ctx, Event{Type: EventTaskUpdated, RepoID: claimed.RepoID, Task: &t})
	}
	return claimed, err
}

// dependenciesMet checks if all dependency tasks are in a terminal success state.
func dependenciesMet(ctx context.Context, repo Repository, dependsOn []string) bool {
	for _, depID := range dependsOn {
		id, err := ParseTaskID(depID)
		if err != nil {
			return false
		}
		status, err := repo.ReadTaskStatus(ctx, id)
		if err != nil {
			return false
		}
		if status != StatusMerged && status != StatusClosed {
			return false
		}
	}
	return true
}

// ReadTaskLogs reads all logs for a task.
func (s *Store) ReadTaskLogs(ctx context.Context, id TaskID) ([]string, error) {
	return s.repo.ReadTaskLogs(ctx, id)
}

// StreamTaskLogs iterates log batches from the database one row at a time,
// calling fn for each batch. This avoids loading all logs into memory.
func (s *Store) StreamTaskLogs(ctx context.Context, id TaskID, fn func(attempt int, lines []string) error) error {
	return s.repo.StreamTaskLogs(ctx, id, fn)
}

// AppendTaskLogs appends log lines to a task for the given attempt.
func (s *Store) AppendTaskLogs(ctx context.Context, id TaskID, attempt int, logs []string) error {
	if err := s.repo.AppendTaskLogs(ctx, id, attempt, logs); err != nil {
		return err
	}
	s.broker.Publish(ctx, Event{Type: EventLogsAppended, TaskID: id, Attempt: attempt, Logs: logs})
	return nil
}


// Heartbeat updates the last heartbeat time for a running task.
func (s *Store) Heartbeat(ctx context.Context, id TaskID) error {
	return s.repo.Heartbeat(ctx, id)
}

// TimeoutStaleTasks fails running tasks whose heartbeat has expired.
func (s *Store) TimeoutStaleTasks(ctx context.Context, timeout time.Duration) (int, error) {
	threshold := time.Now().Add(-timeout)
	tasks, err := s.repo.ListStaleTasks(ctx, threshold)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, t := range tasks {
		_ = s.repo.SetCloseReason(ctx, t.ID, "Worker timeout: no heartbeat received")
		if err := s.repo.UpdateTaskStatus(ctx, t.ID, StatusFailed); err != nil {
			continue
		}
		count++
		s.publishTaskUpdated(ctx, t.ID)
	}
	return count, nil
}

// UpdateTaskStatus updates a task's status.
func (s *Store) UpdateTaskStatus(ctx context.Context, id TaskID, status Status) error {
	if err := s.repo.UpdateTaskStatus(ctx, id, status); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// SetTaskPullRequest sets the PR URL and number, moving the task to review status.
func (s *Store) SetTaskPullRequest(ctx context.Context, id TaskID, prURL string, prNumber int) error {
	if err := s.repo.SetTaskPullRequest(ctx, id, prURL, prNumber); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// ListTasksInReview returns all tasks in review status.
func (s *Store) ListTasksInReview(ctx context.Context) ([]*Task, error) {
	return s.repo.ListTasksInReview(ctx)
}

// SetTaskBranch sets the branch name and moves the task to review status.
func (s *Store) SetTaskBranch(ctx context.Context, id TaskID, branchName string) error {
	if err := s.repo.SetBranchName(ctx, id, branchName); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// ListTasksInReviewNoPR returns tasks in review that have a branch but no PR yet.
func (s *Store) ListTasksInReviewNoPR(ctx context.Context) ([]*Task, error) {
	return s.repo.ListTasksInReviewNoPR(ctx)
}

// CloseTask closes a task with an optional reason.
func (s *Store) CloseTask(ctx context.Context, id TaskID, reason string) error {
	if err := s.repo.CloseTask(ctx, id, reason); err != nil {
		return err
	}
	s.publishTaskUpdated(ctx, id)
	return nil
}

// WaitForPending returns a channel that signals when a pending task might be available.
func (s *Store) WaitForPending() <-chan struct{} {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return s.pendingCh
}

func (s *Store) notifyPending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}
}

func (s *Store) publishTaskUpdated(ctx context.Context, id TaskID) {
	t, err := s.repo.ReadTask(ctx, id)
	if err != nil {
		return
	}
	t.Logs = nil
	s.broker.Publish(ctx, Event{Type: EventTaskUpdated, RepoID: t.RepoID, Task: t})
}
