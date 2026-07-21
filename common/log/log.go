package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Level represents the severity of a log message.
type Level int

const (
	INFO  Level = iota
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides leveled logging to a file.
type Logger struct {
	w     io.Writer
	debug bool
}

// New creates a Logger that writes to the specified file path.
// The parent directory is created if it does not exist.
func New(path string, debug bool) (*Logger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("log: create dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("log: open file: %w", err)
	}

	return &Logger{w: f, debug: debug}, nil
}

// NewConsole creates a Logger that writes to stdout (useful for debug/CLI mode).
func NewConsole() *Logger {
	return &Logger{w: os.Stdout, debug: true}
}

// Info logs a message at INFO level.
func (l *Logger) Info(format string, args ...any) {
	l.write(INFO, format, args...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(format string, args ...any) {
	l.write(WARN, format, args...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(format string, args ...any) {
	l.write(ERROR, format, args...)
}

// Close flushes and closes the underlying log file.
func (l *Logger) Close() error {
	if closer, ok := l.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (l *Logger) write(level Level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("%s [%s] %s\n", timestamp, level, msg)
	_, _ = fmt.Fprint(l.w, line)

	// In debug mode, also write to stderr for interactive debugging
	if l.debug {
		_, _ = fmt.Fprint(os.Stderr, line)
	}
}
