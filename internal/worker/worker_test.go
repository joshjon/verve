package worker

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line1" {
		t.Errorf("expected 'line1', got %q", lines[0])
	}
	if lines[1] != "line2" {
		t.Errorf("expected 'line2', got %q", lines[1])
	}
	if lines[2] != "line3" {
		t.Errorf("expected 'line3', got %q", lines[2])
	}
}

func TestReadLines_NoTrailingNewline(t *testing.T) {
	input := "line1\nline2"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[1] != "line2" {
		t.Errorf("expected 'line2', got %q", lines[1])
	}
}

func TestReadLines_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestReadLines_SingleLine(t *testing.T) {
	reader := strings.NewReader("single")

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "single" {
		t.Errorf("expected 'single', got %q", lines[0])
	}
}

func TestReadLines_EmptyLines(t *testing.T) {
	input := "line1\n\nline2\n\n"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (empty lines skipped), got %d: %v", len(lines), lines)
	}
}

func TestReadLines_LargeInput(t *testing.T) {
	line := strings.Repeat("x", 5000) + "\n"
	reader := strings.NewReader(line)

	var lines []string
	readLines(reader, func(l string) {
		lines = append(lines, l)
	})

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0]) != 5000 {
		t.Errorf("expected line length 5000, got %d", len(lines[0]))
	}
}

func TestMarkerParsing_VERVE_PR_CREATED(t *testing.T) {
	line := `VERVE_PR_CREATED: {"url":"https://github.com/org/repo/pull/42","number":42}`
	prURL, prNumber := parsePRMarker(line)
	if prURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected PR URL, got %q", prURL)
	}
	if prNumber != 42 {
		t.Errorf("expected PR number 42, got %d", prNumber)
	}
}

func TestMarkerParsing_VERVE_BRANCH_PUSHED(t *testing.T) {
	line := `VERVE_BRANCH_PUSHED: {"branch":"verve/task-tsk_123"}`
	branch := parseBranchMarker(line)
	if branch != "verve/task-tsk_123" {
		t.Errorf("expected branch 'verve/task-tsk_123', got %q", branch)
	}
}

func TestMarkerParsing_VERVE_STATUS(t *testing.T) {
	line := `VERVE_STATUS:{"tests":"pass","confidence":"high"}`
	status := parseStatusMarker(line)
	if status != `{"tests":"pass","confidence":"high"}` {
		t.Errorf("unexpected status: %q", status)
	}
}

func TestMarkerParsing_VERVE_COST(t *testing.T) {
	line := `VERVE_COST: 1.234`
	cost := parseCostMarker(line)
	if cost != 1.234 {
		t.Errorf("expected cost 1.234, got %f", cost)
	}
}

func TestMarkerParsing_VERVE_PREREQ_FAILED(t *testing.T) {
	line := `VERVE_PREREQ_FAILED: {"reason":"deps not met"}`
	prereq := parsePrereqMarker(line)
	if prereq != ` {"reason":"deps not met"}` {
		t.Errorf("unexpected prereq: %q", prereq)
	}
}

func TestMarkerParsing_BoldFormatting(t *testing.T) {
	line := `**VERVE_PR_CREATED: {"url":"https://github.com/org/repo/pull/1","number":1}**`
	cleanLine := strings.TrimRight(strings.TrimLeft(line, "*"), "*")
	prURL, prNumber := parsePRMarker(cleanLine)
	if prURL != "https://github.com/org/repo/pull/1" {
		t.Errorf("expected PR URL, got %q", prURL)
	}
	if prNumber != 1 {
		t.Errorf("expected PR number 1, got %d", prNumber)
	}
}

func TestMarkerParsing_NoMarker(t *testing.T) {
	line := "just a regular log line"
	if prURL, _ := parsePRMarker(line); prURL != "" {
		t.Error("expected empty PR URL for non-marker line")
	}
	if branch := parseBranchMarker(line); branch != "" {
		t.Error("expected empty branch for non-marker line")
	}
	if status := parseStatusMarker(line); status != "" {
		t.Error("expected empty status for non-marker line")
	}
	if cost := parseCostMarker(line); cost != 0 {
		t.Error("expected zero cost for non-marker line")
	}
	if prereq := parsePrereqMarker(line); prereq != "" {
		t.Error("expected empty prereq for non-marker line")
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		APIURL:          "http://localhost:7400",
		AnthropicAPIKey: "sk-ant-test",
		AgentImage:      "verve-agent:latest",
	}
	if cfg.MaxConcurrentTasks != 0 {
		t.Error("expected default max concurrent tasks to be 0 (unset)")
	}
	if cfg.DryRun {
		t.Error("expected DryRun default to be false")
	}
}

func TestPollResponse(t *testing.T) {
	resp := PollResponse{
		Task: Task{
			ID:          "tsk_123",
			Title:       "test task",
			Description: "do something",
			Status:      "running",
		},
		GitHubToken:  "ghp_abc",
		RepoFullName: "owner/repo",
	}

	if resp.Task.ID != "tsk_123" {
		t.Errorf("expected task ID tsk_123, got %s", resp.Task.ID)
	}
	if resp.GitHubToken != "ghp_abc" {
		t.Errorf("expected token ghp_abc, got %s", resp.GitHubToken)
	}
	if resp.RepoFullName != "owner/repo" {
		t.Errorf("expected repo owner/repo, got %s", resp.RepoFullName)
	}
}

func TestLogStreamer_BufferManagement(t *testing.T) {
	var mu sync.Mutex
	var sentBatches [][]string

	ls := &logStreamer{
		taskID:    "tsk_test",
		attempt:   1,
		buffer:    make([]string, 0, 100),
		done:      make(chan struct{}),
		flushed:   make(chan struct{}),
		interval:  100 * time.Millisecond,
		batchSize: 5,
	}

	// Manually add lines to buffer
	ls.mu.Lock()
	ls.buffer = append(ls.buffer, "line1", "line2", "line3")
	ls.mu.Unlock()

	ls.mu.Lock()
	bufLen := len(ls.buffer)
	ls.mu.Unlock()

	if bufLen != 3 {
		t.Errorf("expected 3 buffered lines, got %d", bufLen)
	}

	// Simulate flush: take ownership of buffer
	ls.mu.Lock()
	toSend := ls.buffer
	ls.buffer = make([]string, 0, 100)
	ls.mu.Unlock()

	mu.Lock()
	sentBatches = append(sentBatches, toSend)
	mu.Unlock()

	mu.Lock()
	if len(sentBatches) != 1 {
		t.Errorf("expected 1 batch, got %d", len(sentBatches))
	}
	if len(sentBatches[0]) != 3 {
		t.Errorf("expected batch of 3, got %d", len(sentBatches[0]))
	}
	mu.Unlock()

	ls.mu.Lock()
	if len(ls.buffer) != 0 {
		t.Error("expected buffer to be empty after flush")
	}
	ls.mu.Unlock()
}

func TestRunResult(t *testing.T) {
	success := RunResult{Success: true, ExitCode: 0}
	if !success.Success {
		t.Error("expected success")
	}
	if success.ExitCode != 0 {
		t.Error("expected exit code 0")
	}

	failure := RunResult{Success: false, ExitCode: 1}
	if failure.Success {
		t.Error("expected failure")
	}
	if failure.ExitCode != 1 {
		t.Error("expected exit code 1")
	}

	errResult := RunResult{Error: io.EOF}
	if errResult.Error != io.EOF {
		t.Error("expected EOF error")
	}
}

func TestAgentConfig(t *testing.T) {
	cfg := AgentConfig{
		TaskID:               "tsk_123",
		TaskTitle:            "Fix bug",
		TaskDescription:      "Fix the login bug",
		GitHubToken:          "ghp_test",
		GitHubRepo:           "owner/repo",
		AnthropicAPIKey:      "sk-ant-test",
		ClaudeModel:          "sonnet",
		DryRun:               true,
		SkipPR:               true,
		Attempt:              2,
		RetryReason:          "CI failed",
		AcceptanceCriteria:   []string{"Tests pass"},
		RetryContext:         "Previous attempt logs...",
		PreviousStatus:       `{"tests":"fail"}`,
	}

	if cfg.TaskID != "tsk_123" {
		t.Error("unexpected TaskID")
	}
	if cfg.Attempt != 2 {
		t.Error("unexpected Attempt")
	}
	if !cfg.DryRun {
		t.Error("expected DryRun true")
	}
}

func TestDefaultAgentImage(t *testing.T) {
	if DefaultAgentImage != "verve-agent:latest" {
		t.Errorf("expected 'verve-agent:latest', got %s", DefaultAgentImage)
	}
}
