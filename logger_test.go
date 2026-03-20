package bender

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		level LogLevel
		ok    bool
	}{
		{name: "none", input: "none", level: LogLevelNone, ok: true},
		{name: "error", input: "error", level: LogLevelError, ok: true},
		{name: "info", input: "info", level: LogLevelInfo, ok: true},
		{name: "debug", input: "debug", level: LogLevelDebug, ok: true},
		{name: "trim and case", input: " DEBUG ", level: LogLevelDebug, ok: true},
		{name: "invalid", input: "trace", level: LogLevelNone, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			level, ok := parseLogLevel(tc.input)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.level, level)
		})
	}
}

func TestLoggerFromEnv(t *testing.T) {
	t.Run("unset returns noop", func(t *testing.T) {
		t.Setenv("BENDER_LOG_LEVEL", "")
		logger := loggerFromEnv()
		_, ok := logger.(NoopLogger)
		assert.True(t, ok, "expected NoopLogger, got %T", logger)
	})

	t.Run("invalid returns noop", func(t *testing.T) {
		t.Setenv("BENDER_LOG_LEVEL", "invalid")
		logger := loggerFromEnv()
		_, ok := logger.(NoopLogger)
		assert.True(t, ok, "expected NoopLogger, got %T", logger)
	})

	t.Run("valid returns default logger", func(t *testing.T) {
		t.Setenv("BENDER_LOG_LEVEL", "info")
		logger := loggerFromEnv()
		dl, ok := logger.(*DefaultLogger)
		require.True(t, ok, "expected DefaultLogger, got %T", logger)
		assert.Equal(t, LogLevelInfo, dl.level)
	})
}

func TestDefaultLoggerSetOutputAndSetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDefaultLogger(LogLevelError)
	logger.SetOutput(buf)

	logger.Info("hidden")
	assert.Zero(t, buf.Len())

	logger.SetLevel(LogLevelDebug)
	logger.Debug("hello %s", "world")
	assert.Contains(t, buf.String(), "hello world")
}

func TestNoopLoggerMethods(t *testing.T) {
	var logger NoopLogger
	logger.Debug("x")
	logger.Info("x")
	logger.Error("x")
}

func TestFormatPath(t *testing.T) {
	assert.Equal(t, "", formatPath(nil))

	k1 := keyOfType(typeOf[int](), "")
	k2 := keyOfType(typeOf[string](), "named")
	formatted := formatPath([]Key{k1, k2})
	assert.Contains(t, formatted, "int")
	assert.Contains(t, formatted, "string")
}
