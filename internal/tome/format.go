package tome

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// FormatSearchResults writes search results in human-readable text format.
func FormatSearchResults(w io.Writer, results []SearchResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No sessions found.")
		return
	}

	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		formatSession(w, r.Session)
	}
}

// FormatLog writes sessions in human-readable text format.
func FormatLog(w io.Writer, sessions []Session) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions recorded.")
		return
	}

	for i, s := range sessions {
		if i > 0 {
			fmt.Fprintln(w)
		}
		formatSession(w, s)
	}
}

// FormatJSON writes the value as indented JSON.
func FormatJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func formatSession(w io.Writer, s Session) {
	fmt.Fprintf(w, "━━ %s (%s, %s) ━━\n", s.Summary, s.Status, relativeTime(s.CreatedAt))

	if len(s.Files) > 0 {
		fmt.Fprintf(w, "Files: %s\n", strings.Join(s.Files, ", "))
	}
	if len(s.Tags) > 0 {
		fmt.Fprintf(w, "Tags:  %s\n", strings.Join(s.Tags, ", "))
	}
	if s.Branch != "" {
		fmt.Fprintf(w, "Branch: %s\n", s.Branch)
	}
	if s.Author != "" {
		fmt.Fprintf(w, "Author: %s\n", s.Author)
	}
	if s.Learnings != "" {
		fmt.Fprintln(w, "Learnings:")
		for _, line := range strings.Split(s.Learnings, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
