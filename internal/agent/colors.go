package agent

import (
	"os"
)

// Colors holds ANSI color codes for terminal output
type Colors struct {
	Red    string
	Green  string
	Yellow string
	Blue   string
	Cyan   string
	Reset  string
	Bold   string
}

// TermColors contains the color codes for terminal output
var TermColors Colors

func init() {
	// Check if NO_COLOR environment variable is set
	// For now, we'll enable colors by default unless NO_COLOR is set
	if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		TermColors = Colors{
			Red:    "\033[31m",
			Green:  "\033[32m",
			Yellow: "\033[33m",
			Blue:   "\033[34m",
			Cyan:   "\033[36m",
			Reset:  "\033[0m",
			Bold:   "\033[1m",
		}
	} else {
		// No colors if NO_COLOR is set or TERM is dumb
		TermColors = Colors{}
	}
}

// Colorize returns the text wrapped in the given color
func Colorize(text, color string) string {
	if color == "" {
		return text
	}
	return color + text + TermColors.Reset
}