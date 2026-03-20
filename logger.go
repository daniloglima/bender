package bender

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger is the interface for bender logging.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// LogLevel represents the logging level.
type LogLevel int

const (
	LogLevelNone LogLevel = iota
	LogLevelError
	LogLevelInfo
	LogLevelDebug
)

// DefaultLogger is a simple logger implementation.
type DefaultLogger struct {
	mu     sync.Mutex
	level  LogLevel
	writer io.Writer
}

// NewDefaultLogger creates a new default logger.
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		writer: os.Stdout,
	}
}

// SetOutput sets the output writer for the logger.
func (l *DefaultLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
}

// SetLevel sets the logging level.
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *DefaultLogger) log(level string, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05.000")
	formatted := fmt.Sprintf(msg, args...)

	_, _ = fmt.Fprintf(l.writer, "[%s] [BENDER] [%s] %s\n", timestamp, level, formatted)
}

func (l *DefaultLogger) Debug(msg string, args ...any) {
	if l.level >= LogLevelDebug {
		l.log("DEBUG", msg, args...)
	}
}

func (l *DefaultLogger) Info(msg string, args ...any) {
	if l.level >= LogLevelInfo {
		l.log("INFO", msg, args...)
	}
}

func (l *DefaultLogger) Error(msg string, args ...any) {
	if l.level >= LogLevelError {
		l.log("ERROR", msg, args...)
	}
}

// NoopLogger is a logger that does nothing.
type NoopLogger struct{}

func (NoopLogger) Debug(_ string, _ ...any) {}
func (NoopLogger) Info(_ string, _ ...any)  {}
func (NoopLogger) Error(_ string, _ ...any) {}

func loggerFromEnv() Logger {
	raw, ok := os.LookupEnv("BENDER_LOG_LEVEL")
	if !ok {
		return NoopLogger{}
	}

	level, ok := parseLogLevel(raw)
	if !ok {
		return NoopLogger{}
	}

	return NewDefaultLogger(level)
}

func parseLogLevel(raw string) (LogLevel, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none":
		return LogLevelNone, true
	case "error":
		return LogLevelError, true
	case "info":
		return LogLevelInfo, true
	case "debug":
		return LogLevelDebug, true
	default:
		return LogLevelNone, false
	}
}

// formatKey formats a Key for logging.
func formatKey(k Key) string {
	if k.name != "" {
		return fmt.Sprintf("%s (name=%q)", k.t, k.name)
	}
	return fmt.Sprintf("%s", k.t)
}

// formatPath formats a resolution path for logging.
func formatPath(path []Key) string {
	if len(path) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, k := range path {
		if i > 0 {
			sb.WriteString(" -> ")
		}
		sb.WriteString(formatKey(k))
	}
	return sb.String()
}
