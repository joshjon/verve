package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joshjon/kit/log"
)

// Config holds the worker configuration
type Config struct {
	APIURL             string
	GitHubToken        string
	GitHubRepos        []string // "owner/repo" format, comma-separated from env
	AnthropicAPIKey    string
	ClaudeModel        string // Model to use (haiku, sonnet, opus) - defaults to haiku
	AgentImage         string // Docker image for agent - defaults to verve-agent:latest
	MaxConcurrentTasks int    // Maximum concurrent tasks (default: 1)
	DryRun             bool   // Skip Claude and make a dummy change instead
}

type Task struct {
	ID                 string  `json:"id"`
	RepoID             string  `json:"repo_id"`
	Description        string  `json:"description"`
	Status             string  `json:"status"`
	Attempt            int     `json:"attempt"`
	MaxAttempts        int     `json:"max_attempts"`
	RetryReason        string  `json:"retry_reason,omitempty"`
	AcceptanceCriteria string  `json:"acceptance_criteria,omitempty"`
	RetryContext       string  `json:"retry_context,omitempty"`
	AgentStatus        string  `json:"agent_status,omitempty"`
	CostUSD            float64 `json:"cost_usd"`
	MaxCostUSD         float64 `json:"max_cost_usd,omitempty"`
}

type apiRepo struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type Worker struct {
	config       Config
	docker       *DockerRunner
	client       *http.Client
	logger       log.Logger
	pollInterval time.Duration

	// Repo mapping: repo ID -> full name (e.g. "owner/repo")
	repoIDs   []string            // repo IDs to poll for
	repoNames map[string]string   // repo ID -> full name

	// Concurrency control
	maxConcurrent int
	semaphore     chan struct{}
	wg            sync.WaitGroup
	activeTasks   int
	activeMu      sync.Mutex
}

func New(cfg Config, logger log.Logger) (*Worker, error) {
	docker, err := NewDockerRunner(cfg.AgentImage, logger)
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
		logger:        logger,
		pollInterval:  5 * time.Second,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}, nil
}

func (w *Worker) Close() error {
	return w.docker.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("worker starting", "max_concurrent", w.maxConcurrent, "repos", w.config.GitHubRepos)

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	w.logger.Info("agent image verified", "image", w.docker.AgentImage())

	// Fetch repos from API and build ID mapping
	if err := w.initRepoMapping(ctx); err != nil {
		return fmt.Errorf("init repo mapping: %w", err)
	}
	w.logger.Info("repo mapping initialized", "repo_ids", w.repoIDs)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker shutting down, waiting for active tasks")
			w.wg.Wait()
			w.logger.Info("all tasks completed, worker stopped")
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
			w.logger.Error("error polling for task", "error", err)
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

		w.logger.Info("claimed task",
			"task_id", task.ID,
			"active", activeCount,
			"max_concurrent", w.maxConcurrent,
			"description", task.Description,
		)

		// Execute task - use goroutine only if concurrent execution is enabled
		if w.maxConcurrent > 1 {
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
		} else {
			// Sequential execution - simpler, more compatible with restricted networks
			w.executeTask(ctx, task)
			<-w.semaphore
			w.activeMu.Lock()
			w.activeTasks--
			w.activeMu.Unlock()
		}
	}
}

func (w *Worker) initRepoMapping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", w.config.APIURL+"/api/v1/repos", nil)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list repos: status %d: %s", resp.StatusCode, body)
	}

	var repos []apiRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return err
	}

	// Build full name -> ID lookup
	nameToID := make(map[string]string, len(repos))
	idToName := make(map[string]string, len(repos))
	for _, r := range repos {
		nameToID[r.FullName] = r.ID
		idToName[r.ID] = r.FullName
	}

	// Map configured repo full names to their IDs
	var repoIDs []string
	for _, fullName := range w.config.GitHubRepos {
		id, ok := nameToID[fullName]
		if !ok {
			return fmt.Errorf("configured repo %q not found on server — add it via the API first", fullName)
		}
		repoIDs = append(repoIDs, id)
	}

	w.repoIDs = repoIDs
	w.repoNames = idToName
	return nil
}

func (w *Worker) pollForTask(ctx context.Context) (*Task, error) {
	pollURL := w.config.APIURL + "/api/v1/tasks/poll"
	if len(w.repoIDs) > 0 {
		pollURL += "?repos=" + strings.Join(w.repoIDs, ",")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
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
		ls.worker.logger.Error("failed to send logs", "task_id", ls.taskID, "error", err)
	}
}

func (w *Worker) executeTask(ctx context.Context, task *Task) {
	taskLogger := w.logger.With("task_id", task.ID)

	// Create log streamer for real-time log streaming
	streamer := newLogStreamer(ctx, w, task.ID)

	// Track PR info and agent markers
	var prURL string
	var prNumber int
	var agentStatus string
	var costUSD float64
	var prereqFailed string
	var markerMu sync.Mutex

	// Log callback - called from Docker log streaming goroutine
	onLog := func(line string) {
		taskLogger.Debug("agent output", "line", line)
		streamer.AddLine(line)

		// Parse PR marker
		if strings.HasPrefix(line, "VERVE_PR_CREATED:") {
			jsonStr := strings.TrimPrefix(line, "VERVE_PR_CREATED:")
			var prInfo struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &prInfo); err == nil {
				markerMu.Lock()
				prURL = prInfo.URL
				prNumber = prInfo.Number
				markerMu.Unlock()
				taskLogger.Info("captured PR", "url", prURL, "number", prNumber)
			}
		}

		// Parse agent status marker
		if strings.HasPrefix(line, "VERVE_STATUS:") {
			statusJSON := strings.TrimPrefix(line, "VERVE_STATUS:")
			markerMu.Lock()
			agentStatus = statusJSON
			markerMu.Unlock()
			taskLogger.Info("captured agent status")
		}

		// Parse prereq failure marker
		if strings.HasPrefix(line, "VERVE_PREREQ_FAILED:") {
			jsonStr := strings.TrimPrefix(line, "VERVE_PREREQ_FAILED:")
			markerMu.Lock()
			prereqFailed = jsonStr
			markerMu.Unlock()
			taskLogger.Warn("prerequisite check failed", "details", jsonStr)
		}

		// Parse cost marker
		if strings.HasPrefix(line, "VERVE_COST:") {
			costStr := strings.TrimPrefix(line, "VERVE_COST:")
			var cost float64
			if _, err := fmt.Sscanf(costStr, "%f", &cost); err == nil {
				markerMu.Lock()
				costUSD = cost
				markerMu.Unlock()
				taskLogger.Info("captured cost", "cost_usd", cost)
			}
		}
	}

	// Look up repo full name from task's RepoID
	repoFullName := w.repoNames[task.RepoID]
	if repoFullName == "" {
		taskLogger.Error("unknown repo ID for task", "repo_id", task.RepoID)
		w.completeTask(ctx, task.ID, false, fmt.Sprintf("unknown repo ID: %s", task.RepoID), "", 0, "", 0, "")
		return
	}

	// Create agent config from worker config
	agentCfg := AgentConfig{
		TaskID:             task.ID,
		TaskDescription:    task.Description,
		GitHubToken:        w.config.GitHubToken,
		GitHubRepo:         repoFullName,
		AnthropicAPIKey:    w.config.AnthropicAPIKey,
		ClaudeModel:        w.config.ClaudeModel,
		DryRun:             w.config.DryRun,
		Attempt:            task.Attempt,
		RetryReason:        task.RetryReason,
		AcceptanceCriteria: task.AcceptanceCriteria,
		RetryContext:       task.RetryContext,
		PreviousStatus:     task.AgentStatus,
	}

	// Run the agent with streaming logs
	result := w.docker.RunAgent(ctx, agentCfg, onLog)

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	// Get captured marker values
	markerMu.Lock()
	capturedPRURL := prURL
	capturedPRNumber := prNumber
	capturedAgentStatus := agentStatus
	capturedCostUSD := costUSD
	capturedPrereqFailed := prereqFailed
	markerMu.Unlock()

	// Report completion with PR info, agent status, and cost
	if result.Error != nil {
		taskLogger.Error("task failed", "error", result.Error)
		w.completeTask(ctx, task.ID, false, result.Error.Error(), "", 0, capturedAgentStatus, capturedCostUSD, capturedPrereqFailed)
	} else if result.Success {
		taskLogger.Info("task completed successfully")
		w.completeTask(ctx, task.ID, true, "", capturedPRURL, capturedPRNumber, capturedAgentStatus, capturedCostUSD, "")
	} else {
		errMsg := fmt.Sprintf("exit code %d", result.ExitCode)
		if capturedPrereqFailed != "" {
			errMsg = "prerequisite check failed"
		}
		taskLogger.Error("task failed", "exit_code", result.ExitCode)
		w.completeTask(ctx, task.ID, false, errMsg, "", 0, capturedAgentStatus, capturedCostUSD, capturedPrereqFailed)
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

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg string, prURL string, prNumber int, agentStatus string, costUSD float64, prereqFailed string) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	if prURL != "" {
		payload["pull_request_url"] = prURL
		payload["pr_number"] = prNumber
	}
	if agentStatus != "" {
		payload["agent_status"] = agentStatus
	}
	if costUSD > 0 {
		payload["cost_usd"] = costUSD
	}
	if prereqFailed != "" {
		payload["prereq_failed"] = prereqFailed
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
