package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusReview    TaskStatus = "review"    // PR created, awaiting review/merge
	TaskStatusMerged    TaskStatus = "merged"    // PR has been merged
	TaskStatusCompleted TaskStatus = "completed" // Completed without PR (no changes)
	TaskStatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID             string     `json:"id"`
	Description    string     `json:"description"`
	Status         TaskStatus `json:"status"`
	Logs           []string   `json:"logs"`
	PullRequestURL string     `json:"pull_request_url,omitempty"`
	PRNumber       int        `json:"pr_number,omitempty"`
	DependsOn      []string   `json:"depends_on,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Store struct {
	mu    sync.RWMutex
	tasks map[string]*Task

	// Channel to notify waiters when a new pending task is available
	pendingCh chan struct{}
}

func NewStore() *Store {
	return &Store{
		tasks:     make(map[string]*Task),
		pendingCh: make(chan struct{}, 1),
	}
}

// CreateOptions holds optional parameters for task creation
type CreateOptions struct {
	DependsOn []string
}

func (s *Store) Create(description string, opts *CreateOptions) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var dependsOn []string
	if opts != nil && len(opts.DependsOn) > 0 {
		// Validate all dependencies exist
		for _, depID := range opts.DependsOn {
			if _, ok := s.tasks[depID]; !ok {
				return nil, fmt.Errorf("dependency task not found: %s", depID)
			}
		}
		dependsOn = opts.DependsOn
	}

	task := &Task{
		ID:          "tsk_" + uuid.New().String()[:8],
		Description: description,
		Status:      TaskStatusPending,
		Logs:        []string{},
		DependsOn:   dependsOn,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	s.tasks[task.ID] = task

	// Signal that a new pending task is available
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}

	return task, nil
}

func (s *Store) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

func (s *Store) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ClaimPending atomically finds and claims a pending task with all dependencies met.
// Returns nil if no eligible pending task is available.
func (s *Store) ClaimPending() *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.Status == TaskStatusPending && s.dependenciesMet(t) {
			t.Status = TaskStatusRunning
			t.UpdatedAt = time.Now()
			return t
		}
	}
	return nil
}

// dependenciesMet checks if all dependencies of a task are in a terminal success state.
// Must be called with lock held.
func (s *Store) dependenciesMet(t *Task) bool {
	for _, depID := range t.DependsOn {
		dep, ok := s.tasks[depID]
		if !ok {
			// Dependency doesn't exist - treat as unmet
			return false
		}
		// Dependencies are met when parent task is completed, merged, or review
		// (review means the work is done, just awaiting human merge)
		if dep.Status != TaskStatusCompleted && dep.Status != TaskStatusMerged && dep.Status != TaskStatusReview {
			return false
		}
	}
	return true
}

// WaitForPending blocks until a pending task might be available or timeout.
// Returns a channel that signals when to check for pending tasks.
func (s *Store) WaitForPending() <-chan struct{} {
	return s.pendingCh
}

func (s *Store) AppendLogs(id string, logs []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return false
	}
	task.Logs = append(task.Logs, logs...)
	task.UpdatedAt = time.Now()
	return true
}

func (s *Store) UpdateStatus(id string, status TaskStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return false
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	return true
}

// SetPullRequest sets the PR URL and number for a task and updates status to review.
func (s *Store) SetPullRequest(id string, prURL string, prNumber int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return false
	}
	task.PullRequestURL = prURL
	task.PRNumber = prNumber
	task.Status = TaskStatusReview
	task.UpdatedAt = time.Now()
	return true
}

// GetTasksInReview returns all tasks currently in review status.
func (s *Store) GetTasksInReview() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*Task
	for _, t := range s.tasks {
		if t.Status == TaskStatusReview {
			tasks = append(tasks, t)
		}
	}
	return tasks
}
