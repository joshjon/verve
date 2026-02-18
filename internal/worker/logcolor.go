package worker

import (
	"fmt"
	"io"
	"strings"
)

// ANSI color codes for terminal output
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiItalic = "\033[3m"

	// Foreground colors
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiWhite   = "\033[37m"

	// Bright foreground colors
	ansiBrightMagenta = "\033[95m"
	ansiBrightCyan    = "\033[96m"
	ansiBrightWhite   = "\033[97m"
)

// logPrefix defines the color scheme for a specific log prefix.
type logPrefix struct {
	prefix     string // e.g., "[claude] "
	tagColor   string // ANSI codes for the prefix tag
	textColor  string // ANSI codes for the message text
}

// Known agent log prefixes and their color schemes.
var logPrefixes = []logPrefix{
	{
		prefix:   "[claude] ",
		tagColor: ansiBold + ansiBrightMagenta,
		textColor: ansiBold + ansiBrightWhite,
	},
	{
		prefix:   "[thinking] ",
		tagColor: ansiBold + ansiCyan,
		textColor: ansiDim + ansiItalic,
	},
	{
		prefix:   "[tool] ",
		tagColor: ansiBold + ansiBlue,
		textColor: ansiWhite,
	},
	{
		prefix:   "[agent] ",
		tagColor: ansiBold + ansiGreen,
		textColor: ansiWhite,
	},
	{
		prefix:   "[error] ",
		tagColor: ansiBold + ansiRed,
		textColor: ansiRed,
	},
	{
		prefix:   "[stderr] ",
		tagColor: ansiBold + ansiRed,
		textColor: ansiDim + ansiRed,
	},
	{
		prefix:   "[result] ",
		tagColor: ansiBold + ansiYellow,
		textColor: ansiWhite,
	},
}

// ColorizeLogLine applies ANSI color codes to an agent log line based on its prefix.
// Lines without a recognized prefix are returned with dim formatting.
// Header lines (=== ... ===) get special formatting.
func ColorizeLogLine(line string) string {
	// Header lines
	if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
		return ansiBold + ansiCyan + line + ansiReset
	}

	// Check each known prefix
	for _, lp := range logPrefixes {
		if strings.HasPrefix(line, lp.prefix) {
			tag := line[:len(lp.prefix)]
			msg := line[len(lp.prefix):]
			return lp.tagColor + tag + ansiReset + lp.textColor + msg + ansiReset
		}
	}

	// Unrecognized lines: dim
	return ansiDim + line + ansiReset
}

// WriteColorizedLine writes a colorized log line to the writer with a newline.
func WriteColorizedLine(w io.Writer, taskID, line string) {
	colored := ColorizeLogLine(line)
	fmt.Fprintf(w, "%s%s%s %s\n", ansiDim, taskID, ansiReset, colored)
}
