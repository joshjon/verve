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
	APIURL               string
	AnthropicAPIKey      string // API key auth (pay-per-use)
	ClaudeCodeOAuthToken string // OAuth token auth (subscription-based, alternative to API key)
	AgentImage           string // Docker image for agent - defaults to verve-agent:latest
	MaxConcurrentTasks   int    // Maximum concurrent tasks (default: 1)
	DryRun               bool   // Skip Claude and make a dummy change instead
}

type Task struct {
	ID                 string   `json:"id"`
	RepoID             string   `json:"repo_id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Status             string   `json:"status"`
	Attempt            int      `json:"attempt"`
	MaxAttempts        int      `json:"max_attempts"`
	RetryReason        string   `json:"retry_reason,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	RetryContext       string   `json:"retry_context,omitempty"`
	AgentStatus        string   `json:"agent_status,omitempty"`
	CostUSD            float64  `json:"cost_usd"`
	MaxCostUSD         float64  `json:"max_cost_usd,omitempty"`
	SkipPR             bool     `json:"skip_pr"`
	Model              string   `json:"model,omitempty"`
}

// PollResponse wraps a claimed task with credentials and repo info from the server.
type PollResponse struct {
	Task         Task   `json:"task"`
	GitHubToken  string `json:"github_token"`
	RepoFullName string `json:"repo_full_name"`
}

type Worker struct {
	config       Config
	docker       *DockerRunner
	client       *http.Client
	logger       log.Logger
	pollInterval time.Duration

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
	w.logger.Info("worker starting", "max_concurrent", w.maxConcurrent)

	// Warn if API URL is not HTTPS (tokens will be sent in plaintext)
	if !strings.HasPrefix(w.config.APIURL, "https://") {
		w.logger.Warn("API URL is not HTTPS — GitHub tokens will be sent in plaintext, use HTTPS in production", "api_url", w.config.APIURL)
	}

	// Ensure agent image exists
	if err := w.docker.EnsureImage(ctx); err != nil {
		return err
	}
	w.logger.Info("agent image verified", "image", w.docker.AgentImage())

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

		poll, err := w.pollForTask(ctx)
		if err != nil {
			// Release slot on error
			<-w.semaphore
			w.logger.Error("error polling for task", "error", err)
			time.Sleep(w.pollInterval)
			continue
		}

		if poll == nil {
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
			"task_id", poll.Task.ID,
			"repo", poll.RepoFullName,
			"active", activeCount,
			"max_concurrent", w.maxConcurrent,
			"description", poll.Task.Description,
		)

		// Execute task - use goroutine only if concurrent execution is enabled
		if w.maxConcurrent > 1 {
			w.wg.Add(1)
			go func(p *PollResponse) {
				defer w.wg.Done()
				defer func() {
					<-w.semaphore // Release semaphore slot
					w.activeMu.Lock()
					w.activeTasks--
					w.activeMu.Unlock()
				}()
				w.executeTask(ctx, &p.Task, p.GitHubToken, p.RepoFullName)
			}(poll)
		} else {
			// Sequential execution - simpler, more compatible with restricted networks
			w.executeTask(ctx, &poll.Task, poll.GitHubToken, poll.RepoFullName)
			<-w.semaphore
			w.activeMu.Lock()
			w.activeTasks--
			w.activeMu.Unlock()
		}
	}
}

func (w *Worker) pollForTask(ctx context.Context) (*PollResponse, error) {
	pollURL := w.config.APIURL + "/api/v1/tasks/poll"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var poll PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&poll); err != nil {
		return nil, err
	}

	return &poll, nil
}

// logStreamer buffers log lines and periodically sends them to the API server
type logStreamer struct {
	worker    *Worker
	taskID    string
	attempt   int
	ctx       context.Context
	buffer    []string
	mu        sync.Mutex
	done      chan struct{}
	flushed   chan struct{}
	interval  time.Duration
	batchSize int
}

func newLogStreamer(ctx context.Context, w *Worker, taskID string, attempt int) *logStreamer {
	ls := &logStreamer{
		worker:    w,
		taskID:    taskID,
		attempt:   attempt,
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
	if err := ls.worker.sendLogs(ls.ctx, ls.taskID, ls.attempt, toSend); err != nil {
		ls.worker.logger.Error("failed to send logs", "task_id", ls.taskID, "error", err)
	}
}

func (w *Worker) executeTask(ctx context.Context, task *Task, githubToken, repoFullName string) {
	taskLogger := w.logger.With("task_id", task.ID)

	// Create log streamer for real-time log streaming
	streamer := newLogStreamer(ctx, w, task.ID, task.Attempt)

	// Track PR info, branch info, and agent markers
	var prURL string
	var prNumber int
	var branchName string
	var agentStatus string
	var costUSD float64
	var prereqFailed string
	var markerMu sync.Mutex

	// Log callback - called from Docker log streaming goroutine
	onLog := func(line string) {
		taskLogger.Debug("agent output", "line", line)
		streamer.AddLine(line)

		// Strip markdown formatting (e.g. **bold**) that the agent
		// may wrap around marker lines.
		cleanLine := strings.TrimRight(strings.TrimLeft(line, "*"), "*")

		// Parse PR marker
		if strings.HasPrefix(cleanLine, "VERVE_PR_CREATED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_PR_CREATED:")
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

		// Parse branch pushed marker (skip-PR mode)
		if strings.HasPrefix(cleanLine, "VERVE_BRANCH_PUSHED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_BRANCH_PUSHED:")
			var branchInfo struct {
				Branch string `json:"branch"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &branchInfo); err == nil {
				markerMu.Lock()
				branchName = branchInfo.Branch
				markerMu.Unlock()
				taskLogger.Info("captured branch", "branch", branchName)
			}
		}

		// Parse agent status marker
		if strings.HasPrefix(cleanLine, "VERVE_STATUS:") {
			statusJSON := strings.TrimPrefix(cleanLine, "VERVE_STATUS:")
			markerMu.Lock()
			agentStatus = statusJSON
			markerMu.Unlock()
			taskLogger.Info("captured agent status")
		}

		// Parse prereq failure marker
		if strings.HasPrefix(cleanLine, "VERVE_PREREQ_FAILED:") {
			jsonStr := strings.TrimPrefix(cleanLine, "VERVE_PREREQ_FAILED:")
			markerMu.Lock()
			prereqFailed = jsonStr
			markerMu.Unlock()
			taskLogger.Warn("prerequisite check failed", "details", jsonStr)
		}

		// Parse cost marker
		if strings.HasPrefix(cleanLine, "VERVE_COST:") {
			costStr := strings.TrimPrefix(cleanLine, "VERVE_COST:")
			var cost float64
			if _, err := fmt.Sscanf(costStr, "%f", &cost); err == nil {
				markerMu.Lock()
				costUSD = cost
				markerMu.Unlock()
				taskLogger.Info("captured cost", "cost_usd", cost)
			}
		}
	}

	// Create agent config from worker config + server-provided credentials
	agentCfg := AgentConfig{
		TaskID:               task.ID,
		TaskTitle:            task.Title,
		TaskDescription:      task.Description,
		GitHubToken:          githubToken,
		GitHubRepo:           repoFullName,
		AnthropicAPIKey:      w.config.AnthropicAPIKey,
		ClaudeCodeOAuthToken: w.config.ClaudeCodeOAuthToken,
		ClaudeModel:          task.Model,
		DryRun:               w.config.DryRun,
		SkipPR:               task.SkipPR,
		Attempt:              task.Attempt,
		RetryReason:          task.RetryReason,
		AcceptanceCriteria:   task.AcceptanceCriteria,
		RetryContext:         task.RetryContext,
		PreviousStatus:       task.AgentStatus,
	}

	// Run the agent with streaming logs
	result := w.docker.RunAgent(ctx, agentCfg, onLog)

	// Stop the streamer and flush remaining logs
	streamer.Stop()

	// Get captured marker values
	markerMu.Lock()
	capturedPRURL := prURL
	capturedPRNumber := prNumber
	capturedBranchName := branchName
	capturedAgentStatus := agentStatus
	capturedCostUSD := costUSD
	capturedPrereqFailed := prereqFailed
	markerMu.Unlock()

	// Report completion with PR info, agent status, and cost
	switch {
	case result.Error != nil:
		taskLogger.Error("task failed", "error", result.Error)
		_ = w.completeTask(ctx, task.ID, false, result.Error.Error(), "", 0, "", capturedAgentStatus, capturedCostUSD, capturedPrereqFailed)
	case result.Success:
		taskLogger.Info("task completed successfully")
		_ = w.completeTask(ctx, task.ID, true, "", capturedPRURL, capturedPRNumber, capturedBranchName, capturedAgentStatus, capturedCostUSD, "")
	default:
		errMsg := fmt.Sprintf("exit code %d", result.ExitCode)
		if capturedPrereqFailed != "" {
			errMsg = "prerequisite check failed"
		}
		taskLogger.Error("task failed", "exit_code", result.ExitCode)
		_ = w.completeTask(ctx, task.ID, false, errMsg, "", 0, "", capturedAgentStatus, capturedCostUSD, capturedPrereqFailed)
	}
}

func (w *Worker) sendLogs(ctx context.Context, taskID string, attempt int, logs []string) error {
	body, _ := json.Marshal(map[string]any{"logs": logs, "attempt": attempt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/v1/tasks/"+taskID+"/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (w *Worker) completeTask(ctx context.Context, taskID string, success bool, errMsg, prURL string, prNumber int, branchName, agentStatus string, costUSD float64, prereqFailed string) error {
	payload := map[string]interface{}{"success": success}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	if prURL != "" {
		payload["pull_request_url"] = prURL
		payload["pr_number"] = prNumber
	}
	if branchName != "" {
		payload["branch_name"] = branchName
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.APIURL+"/api/v1/tasks/"+taskID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}
