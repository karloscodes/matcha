package matcha

import (
	"fmt"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
)

// Logger handles output formatting.
type Logger struct{}

// NewLogger creates a new logger.
func NewLogger() *Logger {
	return &Logger{}
}

// Info prints an informational message.
func (l *Logger) Info(format string, args ...any) {
	fmt.Printf("  %s\n", fmt.Sprintf(format, args...))
}

// Success prints a success message.
func (l *Logger) Success(format string, args ...any) {
	fmt.Printf("  %s✓ %s%s\n", colorGreen, fmt.Sprintf(format, args...), colorReset)
}

// Warn prints a warning message.
func (l *Logger) Warn(format string, args ...any) {
	fmt.Printf("  %s⚠ %s%s\n", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

// Error prints an error message.
func (l *Logger) Error(format string, args ...any) {
	fmt.Printf("  %s✗ %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
}

// Step prints a step with status.
func (l *Logger) Step(name string, status string) {
	// Clear line and move to beginning
	fmt.Print("\r\033[K")

	padded := name
	for len(padded) < 20 {
		padded += " "
	}

	switch status {
	case "done":
		fmt.Printf("  %s%s✓%s\n", padded, colorGreen, colorReset)
	case "failed":
		fmt.Printf("  %s%s✗ Failed%s\n", padded, colorRed, colorReset)
	case "running":
		fmt.Printf("  %s⠹", padded)
	}
}

// Timestamp returns current time string for logging.
func (l *Logger) Timestamp() string {
	return time.Now().Format("15:04:05")
}
