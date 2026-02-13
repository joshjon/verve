package task

import (
	"context"
	"fmt"
	"sync"

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
	s.notifyPending()

	t := *task
	t.Logs = nil
	s.broker.Publish(ctx, Event{Type: EventTaskCreated, Task: &t})
	return nil
}

// ReadTask reads a task by ID.
func (s *Store) ReadTask(ctx context.Context, id TaskID) (*Task, error) {
	return s.repo.ReadTask(ctx, id)
}

// ListTasks returns all tasks.
func (s *Store) ListTasks(ctx context.Context) ([]*Task, error) {
	return s.repo.ListTasks(ctx)
}

// ClaimPendingTask finds a pending task with all dependencies met and claims it
// by setting its status to running. The read-check-claim flow is wrapped in a
// transaction and uses optimistic locking (WHERE status = 'pending') so that
// concurrent workers cannot claim the same task.
func (s *Store) ClaimPendingTask(ctx context.Context) (*Task, error) {
	var claimed *Task
	err := s.repo.BeginTxFunc(ctx, func(ctx context.Context, _ tx.Tx, repo Repository) error {
		pending, err := repo.ListPendingTasks(ctx)
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
		s.broker.Publish(ctx, Event{Type: EventTaskUpdated, Task: &t})
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

// AppendTaskLogs appends log lines to a task.
func (s *Store) AppendTaskLogs(ctx context.Context, id TaskID, logs []string) error {
	if err := s.repo.AppendTaskLogs(ctx, id, logs); err != nil {
		return err
	}
	s.broker.Publish(ctx, Event{Type: EventLogsAppended, TaskID: id, Logs: logs})
	return nil
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
	s.broker.Publish(ctx, Event{Type: EventTaskUpdated, Task: t})
}
