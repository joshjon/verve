package server

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Logs        []string   `json:"logs"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
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

func (s *Store) Create(description string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &Task{
		ID:          "tsk_" + uuid.New().String()[:8],
		Description: description,
		Status:      TaskStatusPending,
		Logs:        []string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	s.tasks[task.ID] = task

	// Signal that a new pending task is available
	select {
	case s.pendingCh <- struct{}{}:
	default:
	}

	return task
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

// ClaimPending atomically finds and claims a pending task.
// Returns nil if no pending task is available.
func (s *Store) ClaimPending() *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.Status == TaskStatusPending {
			t.Status = TaskStatusRunning
			t.UpdatedAt = time.Now()
			return t
		}
	}
	return nil
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
