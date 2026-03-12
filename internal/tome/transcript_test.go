package tome_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/tome"
)

func TestParseTranscriptBasic(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-123","timestamp":"2026-03-10T14:00:00Z","gitBranch":"feat/auth","message":{"role":"user","content":"Add authentication middleware"}}`,
		`{"type":"assistant","sessionId":"sess-123","timestamp":"2026-03-10T14:01:00Z","message":{"role":"assistant","content":[{"type":"text","text":"I'll add authentication middleware to the project."},{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/auth.go"}},{"type":"tool_result","content":"file contents here"},{"type":"text","text":"Now I'll create the middleware."},{"type":"tool_use","name":"Write","input":{"file_path":"/repo/src/middleware.go"}}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "sess-123", session.ID)
	assert.Equal(t, "Add authentication middleware", session.Summary)
	assert.Equal(t, "feat/auth", session.Branch)
	assert.Contains(t, session.Content, "I'll add authentication middleware")
	assert.Contains(t, session.Content, "Now I'll create the middleware.")
	assert.Contains(t, session.Files, "src/auth.go")
	assert.Contains(t, session.Files, "src/middleware.go")
	assert.Equal(t, "succeeded", session.Status)
}

func TestParseTranscriptSkipsSidechain(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-456","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Fix the bug"}}`,
		`{"type":"assistant","sessionId":"sess-456","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"Sidechain exploration text"}]}}`,
		`{"type":"assistant","sessionId":"sess-456","message":{"role":"assistant","content":[{"type":"text","text":"Main branch fix"}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.NotContains(t, session.Content, "Sidechain exploration")
	assert.Contains(t, session.Content, "Main branch fix")
}

func TestParseTranscriptSkipsThinking(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-789","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Refactor auth"}}`,
		`{"type":"assistant","sessionId":"sess-789","message":{"role":"assistant","content":[{"type":"thinking","text":"Let me think about this..."},{"type":"text","text":"I'll refactor the auth module."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.NotContains(t, session.Content, "Let me think")
	assert.Contains(t, session.Content, "I'll refactor the auth module.")
}

func TestParseTranscriptSkipsFileHistorySnapshot(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-abc","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Update config"}}`,
		`{"type":"file-history-snapshot","sessionId":"sess-abc"}`,
		`{"type":"assistant","sessionId":"sess-abc","message":{"role":"assistant","content":[{"type":"text","text":"Updated the config."}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "sess-abc", session.ID)
	assert.Contains(t, session.Content, "Updated the config.")
}

func TestParseTranscriptSummaryTruncation(t *testing.T) {
	longMessage := strings.Repeat("x", 300)
	transcript := `{"type":"user","sessionId":"sess-long","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"` + longMessage + `"}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Len(t, session.Summary, 200)
}

func TestParseTranscriptSummaryFirstLine(t *testing.T) {
	transcript := `{"type":"user","sessionId":"sess-multi","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"First line of task\nSecond line with details\nThird line"}}`

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "First line of task", session.Summary)
}

func TestParseTranscriptDeduplicatesFiles(t *testing.T) {
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-dedup","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Edit files"}}`,
		`{"type":"assistant","sessionId":"sess-dedup","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/main.go"}},{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/src/main.go"}},{"type":"tool_use","name":"Read","input":{"file_path":"/repo/src/util.go"}}]}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	// Should have exactly 2 unique files, not 3
	assert.Len(t, session.Files, 2)
	assert.Contains(t, session.Files, "src/main.go")
	assert.Contains(t, session.Files, "src/util.go")
}

func TestParseTranscriptNoSessionID(t *testing.T) {
	transcript := `{"type":"user","message":{"role":"user","content":"Hello"}}`

	_, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session ID")
}

func TestParseTranscriptStringContent(t *testing.T) {
	// User messages can have string content (not array of blocks).
	transcript := strings.Join([]string{
		`{"type":"user","sessionId":"sess-str","timestamp":"2026-03-10T14:00:00Z","message":{"role":"user","content":"Simple string content"}}`,
		`{"type":"assistant","sessionId":"sess-str","message":{"role":"assistant","content":"Simple assistant response"}}`,
	}, "\n")

	session, err := tome.ParseTranscript(strings.NewReader(transcript), "/repo")
	require.NoError(t, err)

	assert.Equal(t, "Simple string content", session.Summary)
	assert.Contains(t, session.Content, "Simple assistant response")
}
