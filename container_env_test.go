package bender

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureLogger struct {
	debugCount int
	infoCount  int
	errorCount int
}

func (l *captureLogger) Debug(_ string, _ ...any) { l.debugCount++ }
func (l *captureLogger) Info(_ string, _ ...any)  { l.infoCount++ }
func (l *captureLogger) Error(_ string, _ ...any) { l.errorCount++ }

func TestNewWithOptionsUsesBenderLogLevelFromEnv(t *testing.T) {
	t.Setenv("BENDER_LOG_LEVEL", "debug")

	container := NewWithOptions(nil)
	logger, ok := container.logger.(*DefaultLogger)
	require.True(t, ok, "expected default logger from env, got %T", container.logger)
	assert.Equal(t, LogLevelDebug, logger.level)
}

func TestExplicitOptionOverridesBenderLogLevelEnv(t *testing.T) {
	t.Setenv("BENDER_LOG_LEVEL", "error")

	container := NewWithOptions([]ContainerOption{WithLogLevel(LogLevelInfo)})
	logger, ok := container.logger.(*DefaultLogger)
	require.True(t, ok, "expected default logger from option, got %T", container.logger)
	assert.Equal(t, LogLevelInfo, logger.level)
}

func TestWithLoggerOptionUsesCustomLogger(t *testing.T) {
	t.Setenv("BENDER_LOG_LEVEL", "debug")
	custom := &captureLogger{}

	container := NewWithOptions([]ContainerOption{WithLogger(custom)})
	assert.Same(t, custom, container.logger)
}

func TestWithInfoOptionSetsInfoLevel(t *testing.T) {
	container := NewWithOptions([]ContainerOption{WithInfo()})
	logger, ok := container.logger.(*DefaultLogger)
	require.True(t, ok, "expected default logger, got %T", container.logger)
	assert.Equal(t, LogLevelInfo, logger.level)
}

func TestLoggerFromEnvUnsetVariable(t *testing.T) {
	previous, existed := os.LookupEnv("BENDER_LOG_LEVEL")
	_ = os.Unsetenv("BENDER_LOG_LEVEL")
	defer func() {
		if existed {
			_ = os.Setenv("BENDER_LOG_LEVEL", previous)
			return
		}
		_ = os.Unsetenv("BENDER_LOG_LEVEL")
	}()

	logger := loggerFromEnv()
	_, ok := logger.(NoopLogger)
	assert.True(t, ok, "expected NoopLogger when env is unset, got %T", logger)
}
