package workertracker

import (
	"sync"
	"time"
)

// WorkerInfo represents a connected worker and its metadata.
type WorkerInfo struct {
	// WorkerID is a unique identifier for this worker instance.
	WorkerID string `json:"worker_id"`
	// MaxConcurrentTasks is the maximum number of tasks this worker can run simultaneously.
	MaxConcurrentTasks int `json:"max_concurrent_tasks"`
	// ActiveTasks is the number of tasks currently being executed by this worker.
	ActiveTasks int `json:"active_tasks"`
	// ConnectedAt is when this worker first connected.
	ConnectedAt time.Time `json:"connected_at"`
	// LastPollAt is when this worker last polled for work.
	LastPollAt time.Time `json:"last_poll_at"`
	// UptimeMs is total uptime since the worker first connected.
	UptimeMs int64 `json:"uptime_ms"`
	// Polling indicates whether the worker is currently in a long-poll request.
	Polling bool `json:"polling"`
}

// Registry tracks active workers that are polling for tasks.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	workers map[string]*workerEntry
}

type workerEntry struct {
	info    WorkerInfo
	polling bool
}

// New creates a new worker registry.
func New() *Registry {
	return &Registry{
		workers: make(map[string]*workerEntry),
	}
}

// RecordPollStart is called when a worker begins a long-poll request.
func (r *Registry) RecordPollStart(workerID string, maxConcurrent, activeTasks int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	entry, exists := r.workers[workerID]
	if !exists {
		entry = &workerEntry{
			info: WorkerInfo{
				WorkerID:    workerID,
				ConnectedAt: now,
			},
		}
		r.workers[workerID] = entry
	}

	entry.info.MaxConcurrentTasks = maxConcurrent
	entry.info.ActiveTasks = activeTasks
	entry.info.LastPollAt = now
	entry.polling = true
}

// RecordPollEnd is called when a worker's long-poll request completes.
func (r *Registry) RecordPollEnd(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.workers[workerID]; exists {
		entry.polling = false
	}
}

// ListWorkers returns info about all workers that have polled recently.
// Workers that haven't polled in the given staleness duration are pruned.
func (r *Registry) ListWorkers(staleness time.Duration) []WorkerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-staleness)

	var result []WorkerInfo
	for id, entry := range r.workers {
		if entry.info.LastPollAt.Before(cutoff) {
			delete(r.workers, id)
			continue
		}
		info := entry.info
		info.Polling = entry.polling
		info.UptimeMs = now.Sub(info.ConnectedAt).Milliseconds()
		result = append(result, info)
	}

	return result
}
