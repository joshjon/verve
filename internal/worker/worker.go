package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds the worker configuration
type Config struct {
	APIURL             string
	GitHubToken        string
	GitHubRepo         string
	AnthropicAPIKey    string
	ClaudeModel        string // Model to use (haiku, sonnet, opus) - defaults to haiku
	AgentImage         string // Docker image for agent - defaults to verve-agent:latest
	MaxConcurrentTasks int    // Maximum concurrent tasks (default: 1)
}

type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type Worker struct {
	config       Config
	docker       *DockerRunner
	client       *http.Client
	pollInterval time.Duration

	// Concurrency control
	maxConcurrent int
	semaphore     chan struct{}
	wg            sync.WaitGroup
	activeTasks   int
	activeMu      sync.Mutex
}

func New(cfg Config) (*Worker, error) {
	docker, err := NewDockerRunner(cfg.AgentImage)
	if err != nil {
		return nil, err
	}

	// Default to 1 concurrent task if not specified
	maxConcurrent := cfg.MaxConcurrentTasks
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	return &Worker{
		config:        cfg,
		docker:        docker,
		client:        &http.Client{Timeout: 60 * time.Second},
		pollInterval:  5 * time.Second,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}, nil
}

func (w *Worker) Close() error {
	return w.docker.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	log.Println("Worker starting...")
	log.Printf("Max concurrent tasks: %d", w.maxConcurrent)

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	log.Printf("Agent image verified: %s", w.docker.AgentImage())

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down, waiting for active tasks...")
			w.wg.Wait()
			log.Println("All tasks completed, worker stopped")
			return ctx.Err()
		default:
		}

		// Try to acquire a semaphore slot (non-blocking check first)
		select {
		case w.semaphore <- struct{}{}:
			// Got a slot, proceed to poll for task
		default:
			// All slots full, wait a bit before checking again
			time.Sleep(100 * time.Millisecond)
			continue
		}

		task, err := w.pollForTask(ctx)
		if err != nil {
			// Release slot on error
			<-w.semaphore
			log.Printf("Error polling for task: %v", err)
			time.Sleep(w.pollInterval)
			continue
		}

		if task == nil {
			// No task available, release slot and continue polling
			<-w.semaphore
			continue
		}

		// Track active task count for logging
		w.activeMu.Lock()
		w.activeTasks++
		activeCount := w.activeTasks
		w.activeMu.Unlock()

		log.Printf("Claimed task %s (%d/%d active): %s", task.ID, activeCount, w.maxConcurrent, task.Description)

		// Execute task in goroutine
		w.wg.Add(1)
		go func(t *Task) {
			defer w.wg.Done()
			defer func() {
				<-w.semaphore // Release semaphore slot
				w.activeMu.Lock()
				w.activeTasks--
				w.activeMu.Unlock()
			}()
			w.executeTask(ctx, t)
		}(task)
	}
}

func (w *Worker) pollForTask(ctx context.Context) (*Task, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", w.config.APIURL+"/api/v1/tasks/poll", nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var task Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

// logStreamer buffers log lines and periodically sends them to the API server
type logStreamer struct {
	worker    *Worker
	taskID    string
	ctx       context.Context
	buffer    []string
	mu        sync.Mutex
	done      chan struct{}
	flushed   chan struct{}
	interval  time.Duration
	batchSize int
}

func newLogStreamer(ctx context.Context, w *Worker, taskID string) *logStreamer {
	ls := &logStreamer{
		worker:    w,
		taskID:    taskID,
		ctx:       ctx,
		buffer:    make([]string, 0, 100),
		done:      make(chan struct{}),
		flushed:   make(chan struct{}),
		interval:  2 * time.Second,
		batchSize: 50,
	}
	go ls.flushLoop()
	return ls
}

// AddLine adds a log line to the buffer (thread-safe)
func (ls *logStreamer) AddLine(line string) {
	ls.mu.Lock()
	ls.buffer = append(ls.buffer, line)
	shouldFlush := len(ls.buffer) >= ls.batchSize
	ls.mu.Unlock()

	// Flush immediately if buffer is large
	if shouldFlush {
		ls.flush()
	}
}

// Stop signals the streamer to stop and waits for final flush
func (ls *logStreamer) Stop() {
	close(ls.done)
	<-ls.flushed
}

func (ls *logStreamer) flushLoop() {
	defer close(ls.flushed)

	ticker := time.NewTicker(ls.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ls.flush()
		case <-ls.done:
			// Final flush
			ls.flush()
			return
		}
	}
}

func (ls *logStreamer) flush() {
	ls.mu.Lock()
	if len(ls.buffer) == 0 {
		ls.mu.Unlock()
		return
	}
	// Take ownership of the buffer
	toSend := ls.buffer
	ls.buffer = make([]string, 0, 100)
	ls.mu.Unlock()

	// Send to API server
	if err := ls.worker.sendLogs(ls.ctx, ls.taskID, toSend); err != nil {
		log.Printf("Failed to send logs: %v", err)
	}
}

func (w *Worker) executeTask(ctx context.Context, task *Task) {
	// Create log streamer for real-time log streaming
	streamer := newLogStreamer(ctx, w, task.ID)

	// Track PR info if we see the marker
	var prURL string
	var prNumber int
	var prMu sync.Mutex

	// Log callback - called from Docker log streaming goroutine
	onLog := func(line string) {
		log.Printf("[agent] %s", line)
		streamer.AddLine(line)

		// Parse PR marker
		if strings.HasPrefix(line, "VERVE_PR_CREATED:") {
			jsonStr := strings.TrimPrefix(line, "VERVE_PR_CREATED:")
			var prInfo struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &prInfo); err == nil {
				prMu.Lock()
				prURL = prInfo.URL
				prNumber = prInfo.Number
				prMu.Unlock()
				log.Printf("[worker] Captured PR URL: %s (#%d)", prURL, prNumber)
			}
		}
	}

	// Create agent config from worker config
	agentCfg := AgentConfig{
		TaskID:          task.ID,
		TaskDescription: task.Description,
		GitHubToken:     w.config.GitHubToken,
		GitHubRepo:      w.config.GitHubRepo,
		AnthropicAPIKey: w.config.AnthropicAPIKey,
		ClaudeModel:     w.config.ClaudeModel,
	}

	// Run the agent with streaming logs
	result := w.docker.RunAgent(ctx, agentCfg, onLog)

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	// Get captured PR info
	prMu.Lock()
	capturedPRURL := prURL
	capturedPRNumber := prNumber
	prMu.Unlock()

	// Report completion with PR info
	if result.Error != nil {
		log.Printf("Task %s failed with error: %v", task.ID, result.Error)
		w.completeTask(ctx, task.ID, false, result.Error.Error(), "", 0)
	} else if result.Success {
		log.Printf("Task %s completed successfully", task.ID)
		w.completeTask(ctx, task.ID, true, "", capturedPRURL, capturedPRNumber)
	} else {
		log.Printf("Task %s failed with exit code %d", task.ID, result.ExitCode)
		w.completeTask(ctx, task.ID, false, fmt.Sprintf("exit code %d", result.ExitCode), "", 0)
	}
}

func (w *Worker) sendLogs(ctx context.Context, taskID string, logs []string) error {
	body, _ := json.Marshal(map[string][]string{"logs": logs})
	req, err := http.NewRequestWithContext(ctx, "POST", w.config.APIURL+"/api/v1/tasks/"+taskID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg string, prURL string, prNumber int) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	if prURL != "" {
		payload["pull_request_url"] = prURL
		payload["pr_number"] = prNumber
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", w.config.APIURL+"/api/v1/tasks/"+taskID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}
