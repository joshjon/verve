package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Config holds the worker configuration
type Config struct {
	APIURL          string
	GitHubToken     string
	GitHubRepo      string
	AnthropicAPIKey string
	ClaudeModel     string // Model to use (haiku, sonnet, opus) - defaults to haiku
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
}

func New(cfg Config) (*Worker, error) {
	docker, err := NewDockerRunner()
	if err != nil {
		return nil, err
	}

	return &Worker{
		config:       cfg,
		docker:       docker,
		client:       &http.Client{Timeout: 60 * time.Second},
		pollInterval: 5 * time.Second,
	}, nil
}

func (w *Worker) Close() error {
	return w.docker.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	log.Println("Worker starting...")

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	log.Println("Agent image verified")

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down...")
			return ctx.Err()
		default:
		}

		task, err := w.pollForTask(ctx)
		if err != nil {
			log.Printf("Error polling for task: %v", err)
			time.Sleep(w.pollInterval)
			continue
		}

		if task == nil {
			// No task available, continue polling
			continue
		}

		log.Printf("Claimed task %s: %s", task.ID, task.Description)
		w.executeTask(ctx, task)
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

	// Log callback - called from Docker log streaming goroutine
	onLog := func(line string) {
		log.Printf("[agent] %s", line)
		streamer.AddLine(line)
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

	// Report completion
	if result.Error != nil {
		log.Printf("Task %s failed with error: %v", task.ID, result.Error)
		w.completeTask(ctx, task.ID, false, result.Error.Error())
	} else if result.Success {
		log.Printf("Task %s completed successfully", task.ID)
		w.completeTask(ctx, task.ID, true, "")
	} else {
		log.Printf("Task %s failed with exit code %d", task.ID, result.ExitCode)
		w.completeTask(ctx, task.ID, false, fmt.Sprintf("exit code %d", result.ExitCode))
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

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg string) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
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
