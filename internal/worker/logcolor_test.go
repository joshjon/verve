package worker

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorizeLogLine_ClaudePrefix(t *testing.T) {
	line := "[claude] I'll help you implement this feature."
	result := ColorizeLogLine(line)

	// Should contain ANSI codes
	if !strings.Contains(result, "\033[") {
		t.Error("expected ANSI escape codes in output")
	}
	// Should contain the original text
	if !strings.Contains(result, "I'll help you implement this feature.") {
		t.Error("expected original message text in output")
	}
	// Should contain bold bright magenta for the tag
	if !strings.Contains(result, ansiBold+ansiBrightMagenta) {
		t.Error("expected bold bright magenta for [claude] tag")
	}
	// Should end with reset
	if !strings.HasSuffix(result, ansiReset) {
		t.Error("expected output to end with ANSI reset")
	}
}

func TestColorizeLogLine_ThinkingPrefix(t *testing.T) {
	line := "[thinking] Let me analyze this problem..."
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiCyan) {
		t.Error("expected bold cyan for [thinking] tag")
	}
	if !strings.Contains(result, ansiDim+ansiItalic) {
		t.Error("expected dim italic for thinking text")
	}
}

func TestColorizeLogLine_ToolPrefix(t *testing.T) {
	line := "[tool] Bash: ls -la"
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiBlue) {
		t.Error("expected bold blue for [tool] tag")
	}
}

func TestColorizeLogLine_AgentPrefix(t *testing.T) {
	line := "[agent] Starting Claude Code session..."
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiGreen) {
		t.Error("expected bold green for [agent] tag")
	}
}

func TestColorizeLogLine_ErrorPrefix(t *testing.T) {
	line := "[error] something went wrong"
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiRed) {
		t.Error("expected bold red for [error] tag")
	}
}

func TestColorizeLogLine_StderrPrefix(t *testing.T) {
	line := "[stderr] warning message"
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiRed) {
		t.Error("expected bold red for [stderr] tag")
	}
}

func TestColorizeLogLine_ResultPrefix(t *testing.T) {
	line := "[result] Task completed successfully"
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiYellow) {
		t.Error("expected bold yellow for [result] tag")
	}
}

func TestColorizeLogLine_HeaderLine(t *testing.T) {
	line := "=== Setting up workspace ==="
	result := ColorizeLogLine(line)

	if !strings.Contains(result, ansiBold+ansiCyan) {
		t.Error("expected bold cyan for header lines")
	}
	if !strings.Contains(result, "Setting up workspace") {
		t.Error("expected header text in output")
	}
}

func TestColorizeLogLine_UnrecognizedLine(t *testing.T) {
	line := "some plain output"
	result := ColorizeLogLine(line)

	// Should be dimmed
	if !strings.HasPrefix(result, ansiDim) {
		t.Error("expected dim formatting for unrecognized lines")
	}
	if !strings.Contains(result, "some plain output") {
		t.Error("expected original text in output")
	}
}

func TestWriteColorizedLine(t *testing.T) {
	var buf bytes.Buffer
	WriteColorizedLine(&buf, "tsk_123", "[claude] hello world")
	output := buf.String()

	// Should contain the task ID
	if !strings.Contains(output, "tsk_123") {
		t.Error("expected task ID in output")
	}
	// Should contain the message
	if !strings.Contains(output, "hello world") {
		t.Error("expected message in output")
	}
	// Should end with newline
	if !strings.HasSuffix(output, "\n") {
		t.Error("expected output to end with newline")
	}
}
