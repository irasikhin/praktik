// Package color provides ANSI escape sequences for terminal output styling.
// Output is automatically disabled when stdout is not a terminal, when the
// NO_COLOR environment variable is set, or when Enabled is set to false.
package color

import (
	"os"
	"sync"

	"golang.org/x/term"
)

// Enabled controls whether color codes are emitted. Defaults to true but is
// set to false when NO_COLOR is present or stdout is not a terminal.
var Enabled bool

var (
	once       sync.Once
	isTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
)

// Init detects terminal capability and the NO_COLOR convention.
func Init() {
	once.Do(detect)
}

func detect() {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		Enabled = false
		return
	}
	Enabled = isTerminal()
}

// Disable turns off color output explicitly (e.g. for the --no-color flag).
func Disable() { Enabled = false }

// --- escape codes ---

const reset = "\033[0m"

func bold() string    { return "\033[1m" }
func faint() string   { return "\033[2m" }
func red() string     { return "\033[31m" }
func green() string   { return "\033[32m" }
func yellow() string  { return "\033[33m" }
func blue() string    { return "\033[34m" }
func magenta() string { return "\033[35m" }
func cyan() string    { return "\033[36m" }
func white() string   { return "\033[37m" }

// --- public styling functions ---

// Bold returns s wrapped in bold, or s unchanged if colors are disabled.
func Bold(s string) string {
	if !Enabled {
		return s
	}
	return bold() + s + reset
}

// Faint returns s wrapped in faint/dim.
func Faint(s string) string {
	if !Enabled {
		return s
	}
	return faint() + s + reset
}

// Red returns s in red.
func Red(s string) string {
	if !Enabled {
		return s
	}
	return red() + s + reset
}

// Green returns s in green.
func Green(s string) string {
	if !Enabled {
		return s
	}
	return green() + s + reset
}

// Yellow returns s in yellow.
func Yellow(s string) string {
	if !Enabled {
		return s
	}
	return yellow() + s + reset
}

// Blue returns s in blue.
func Blue(s string) string {
	if !Enabled {
		return s
	}
	return blue() + s + reset
}

// Magenta returns s in magenta.
func Magenta(s string) string {
	if !Enabled {
		return s
	}
	return magenta() + s + reset
}

// Cyan returns s in cyan.
func Cyan(s string) string {
	if !Enabled {
		return s
	}
	return cyan() + s + reset
}

// White returns s in white (bright).
func White(s string) string {
	if !Enabled {
		return s
	}
	return white() + s + reset
}
