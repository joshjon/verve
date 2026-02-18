package worker

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	require.Len(t, lines, 3)
	assert.Equal(t, "line1", lines[0])
	assert.Equal(t, "line2", lines[1])
	assert.Equal(t, "line3", lines[2])
}

func TestReadLines_NoTrailingNewline(t *testing.T) {
	input := "line1\nline2"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	require.Len(t, lines, 2)
	assert.Equal(t, "line2", lines[1])
}

func TestReadLines_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	assert.Len(t, lines, 0)
}

func TestReadLines_SingleLine(t *testing.T) {
	reader := strings.NewReader("single")

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	require.Len(t, lines, 1)
	assert.Equal(t, "single", lines[0])
}

func TestReadLines_EmptyLines(t *testing.T) {
	input := "line1\n\nline2\n\n"
	reader := strings.NewReader(input)

	var lines []string
	readLines(reader, func(line string) {
		lines = append(lines, line)
	})

	require.Len(t, lines, 2, "expected 2 lines (empty lines skipped)")
}

func TestReadLines_LargeInput(t *testing.T) {
	line := strings.Repeat("x", 5000) + "\n"
	reader := strings.NewReader(line)

	var lines []string
	readLines(reader, func(l string) {
		lines = append(lines, l)
	})

	require.Len(t, lines, 1)
	assert.Len(t, lines[0], 5000)
}

func TestMarkerParsing_VERVE_PR_CREATED(t *testing.T) {
	line := `VERVE_PR_CREATED: {"url":"https://github.com/org/repo/pull/42","number":42}`
	prURL, prNumber := parsePRMarker(line)
	assert.Equal(t, "https://github.com/org/repo/pull/42", prURL)
	assert.Equal(t, 42, prNumber)
}

func TestMarkerParsing_VERVE_BRANCH_PUSHED(t *testing.T) {
	line := `VERVE_BRANCH_PUSHED: {"branch":"verve/task-tsk_123"}`
	branch := parseBranchMarker(line)
	assert.Equal(t, "verve/task-tsk_123", branch)
}

func TestMarkerParsing_VERVE_STATUS(t *testing.T) {
	line := `VERVE_STATUS:{"tests":"pass","confidence":"high"}`
	status := parseStatusMarker(line)
	assert.Equal(t, `{"tests":"pass","confidence":"high"}`, status)
}

func TestMarkerParsing_VERVE_COST(t *testing.T) {
	line := `VERVE_COST: 1.234`
	cost := parseCostMarker(line)
	assert.Equal(t, 1.234, cost)
}

func TestMarkerParsing_VERVE_PREREQ_FAILED(t *testing.T) {
	line := `VERVE_PREREQ_FAILED: {"reason":"deps not met"}`
	prereq := parsePrereqMarker(line)
	assert.Equal(t, ` {"reason":"deps not met"}`, prereq)
}

func TestMarkerParsing_BoldFormatting(t *testing.T) {
	line := `**VERVE_PR_CREATED: {"url":"https://github.com/org/repo/pull/1","number":1}**`
	cleanLine := strings.TrimRight(strings.TrimLeft(line, "*"), "*")
	prURL, prNumber := parsePRMarker(cleanLine)
	assert.Equal(t, "https://github.com/org/repo/pull/1", prURL)
	assert.Equal(t, 1, prNumber)
}

func TestMarkerParsing_NoMarker(t *testing.T) {
	line := "just a regular log line"
	prURL, _ := parsePRMarker(line)
	assert.Empty(t, prURL, "expected empty PR URL for non-marker line")
	assert.Empty(t, parseBranchMarker(line), "expected empty branch for non-marker line")
	assert.Empty(t, parseStatusMarker(line), "expected empty status for non-marker line")
	assert.Equal(t, float64(0), parseCostMarker(line), "expected zero cost for non-marker line")
	assert.Empty(t, parsePrereqMarker(line), "expected empty prereq for non-marker line")
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		APIURL:          "http://localhost:7400",
		AnthropicAPIKey: "sk-ant-test",
		AgentImage:      "verve-agent:latest",
	}
	assert.Equal(t, 0, cfg.MaxConcurrentTasks, "expected default max concurrent tasks to be 0 (unset)")
	assert.False(t, cfg.DryRun, "expected DryRun default to be false")
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

	assert.Equal(t, "tsk_123", resp.Task.ID)
	assert.Equal(t, "ghp_abc", resp.GitHubToken)
	assert.Equal(t, "owner/repo", resp.RepoFullName)
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

	assert.Equal(t, 3, bufLen, "expected 3 buffered lines")

	// Simulate flush: take ownership of buffer
	ls.mu.Lock()
	toSend := ls.buffer
	ls.buffer = make([]string, 0, 100)
	ls.mu.Unlock()

	mu.Lock()
	sentBatches = append(sentBatches, toSend)
	mu.Unlock()

	mu.Lock()
	assert.Len(t, sentBatches, 1)
	assert.Len(t, sentBatches[0], 3)
	mu.Unlock()

	ls.mu.Lock()
	assert.Len(t, ls.buffer, 0, "expected buffer to be empty after flush")
	ls.mu.Unlock()
}

func TestRunResult(t *testing.T) {
	success := RunResult{Success: true, ExitCode: 0}
	assert.True(t, success.Success, "expected success")
	assert.Equal(t, 0, success.ExitCode, "expected exit code 0")

	failure := RunResult{Success: false, ExitCode: 1}
	assert.False(t, failure.Success, "expected failure")
	assert.Equal(t, 1, failure.ExitCode, "expected exit code 1")

	errResult := RunResult{Error: io.EOF}
	assert.Equal(t, io.EOF, errResult.Error, "expected EOF error")
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

	assert.Equal(t, "tsk_123", cfg.TaskID, "unexpected TaskID")
	assert.Equal(t, 2, cfg.Attempt, "unexpected Attempt")
	assert.True(t, cfg.DryRun, "expected DryRun true")
}

func TestDefaultAgentImage(t *testing.T) {
	assert.Equal(t, "verve-agent:latest", DefaultAgentImage)
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"Error: Claude max usage exceeded for this session", true},
		{"rate limit exceeded, please try again later", true},
		{"API returned rate_limit error", true},
		{"Error: Too many requests to the API", true},
		{"Error: overloaded_error from API", true},
		{"Max Usage reached for this billing period", true},
		{"RATE LIMIT: please wait before retrying", true},
		{"normal log line about building code", false},
		{"successfully compiled the project", false},
		{"running tests...", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			assert.Equal(t, tt.expected, isRateLimitError(tt.line))
		})
	}
}
