package tome

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

type transcriptEntry struct {
	Type        string             `json:"type"`
	SessionID   string             `json:"sessionId"`
	Timestamp   string             `json:"timestamp"`
	GitBranch   string             `json:"gitBranch"`
	IsSidechain bool               `json:"isSidechain"`
	Message     *transcriptMessage `json:"message"`
}

type transcriptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type  string          `json:"type"`  // "text", "tool_use", "tool_result", "thinking"
	Text  string          `json:"text"`
	Name  string          `json:"name"`  // tool name for tool_use
	Input json.RawMessage `json:"input"` // tool parameters
}

// toolInput is used to extract file_path from tool_use inputs.
type toolInput struct {
	FilePath string `json:"file_path"`
}

// fileToolNames is the set of tool names that operate on files.
var fileToolNames = map[string]bool{
	"Read":         true,
	"Write":        true,
	"Edit":         true,
	"NotebookEdit": true,
}

// ParseTranscript reads a Claude Code .jsonl transcript and extracts a Session.
// The repoRoot is used to strip absolute paths to relative paths.
func ParseTranscript(r io.Reader, repoRoot string) (Session, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB buffer for large tool results

	var (
		sessionID string
		branch    string
		summary   string
		createdAt time.Time
		content   strings.Builder
		filesSet  = make(map[string]bool)
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}

		// Skip non-message entries.
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}

		// Skip sidechain entries.
		if entry.IsSidechain {
			continue
		}

		// Extract session ID from first entry.
		if sessionID == "" && entry.SessionID != "" {
			sessionID = entry.SessionID
		}

		// Extract branch from first non-empty gitBranch.
		if branch == "" && entry.GitBranch != "" {
			branch = entry.GitBranch
		}

		// Extract timestamp from first entry.
		if createdAt.IsZero() && entry.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
				createdAt = t
			} else if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
				createdAt = t
			}
		}

		if entry.Message == nil {
			continue
		}

		// Extract summary from first user message.
		if summary == "" && entry.Message.Role == "user" {
			summary = extractSummary(entry.Message.Content)
		}

		// Extract content and files from assistant messages.
		if entry.Message.Role == "assistant" {
			blocks := parseContentBlocks(entry.Message.Content)
			for _, block := range blocks {
				switch block.Type {
				case "text":
					if block.Text != "" {
						if content.Len() > 0 {
							content.WriteByte('\n')
						}
						content.WriteString(block.Text)
					}
				case "tool_use":
					if fileToolNames[block.Name] && len(block.Input) > 0 {
						var ti toolInput
						if err := json.Unmarshal(block.Input, &ti); err == nil && ti.FilePath != "" {
							relPath := toRelativePath(ti.FilePath, repoRoot)
							filesSet[relPath] = true
						}
					}
				}
				// Skip tool_result, thinking blocks
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Session{}, fmt.Errorf("scan transcript: %w", err)
	}

	if sessionID == "" {
		return Session{}, fmt.Errorf("no session ID found in transcript")
	}

	if summary == "" {
		summary = "(no summary)"
	}

	files := make([]string, 0, len(filesSet))
	for f := range filesSet {
		files = append(files, f)
	}

	return Session{
		ID:        sessionID,
		Summary:   summary,
		Content:   content.String(),
		Files:     files,
		Branch:    branch,
		Status:    "succeeded",
		CreatedAt: createdAt,
	}, nil
}

// extractSummary gets the first line of the first user message, truncated to 200 chars.
func extractSummary(raw json.RawMessage) string {
	// Try as plain string first.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return truncateSummary(text)
	}

	// Try as array of content blocks.
	blocks := parseContentBlocks(raw)
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			return truncateSummary(b.Text)
		}
	}

	return ""
}

func truncateSummary(text string) string {
	// Take first line.
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	text = strings.TrimSpace(text)
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

// parseContentBlocks parses content that may be a string or an array of blocks.
func parseContentBlocks(raw json.RawMessage) []contentBlock {
	// Try as array of blocks.
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}

	// Try as plain string.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil && text != "" {
		return []contentBlock{{Type: "text", Text: text}}
	}

	return nil
}

// toRelativePath strips the repo root from an absolute path.
func toRelativePath(absPath, repoRoot string) string {
	if repoRoot == "" {
		return absPath
	}
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return absPath
	}
	return rel
}
