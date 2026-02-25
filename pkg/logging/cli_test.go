package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCLILogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewCLILogger(tt.level)
			require.NotNil(t, logger)
			logger.Info("test message")
		})
	}
}

func TestCLIHandler_InfoMessage(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler)

	logger.Info("test info message")

	output := buf.String()
	assert.Contains(t, output, "test info message")
	assert.NotContains(t, output, colorRed)
}

func TestCLIHandler_ErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler)

	logger.Error("test error message")

	output := buf.String()
	assert.Contains(t, output, "test error message")
	assert.Contains(t, output, colorRed)
	assert.Contains(t, output, colorReset)
}

func TestCLIHandler_LevelFiltering(t *testing.T) {
	tests := []struct {
		name         string
		handlerLevel slog.Level
		logFunc      func(*slog.Logger)
		shouldLog    bool
	}{
		{"info handler logs info", slog.LevelInfo, func(l *slog.Logger) { l.Info("test") }, true},
		{"info handler filters debug", slog.LevelInfo, func(l *slog.Logger) { l.Debug("test") }, false},
		{"debug handler logs debug", slog.LevelDebug, func(l *slog.Logger) { l.Debug("test") }, true},
		{"error handler logs error", slog.LevelError, func(l *slog.Logger) { l.Error("test") }, true},
		{"error handler filters info", slog.LevelError, func(l *slog.Logger) { l.Info("test") }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewCLIHandler(&buf, tt.handlerLevel)
			logger := slog.New(handler)

			tt.logFunc(logger)

			hasOutput := buf.Len() > 0
			assert.Equal(t, tt.shouldLog, hasOutput)
		})
	}
}

func TestCLIHandler_IncludesAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler)

	logger.Info("test message", "key1", "value1", "key2", "value2")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key1=value1")
	assert.Contains(t, output, "key2=value2")
}

func TestCLIHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)

	result := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	assert.Equal(t, handler, result)

	result = handler.WithAttrs(nil)
	assert.Equal(t, handler, result)
}

func TestCLIHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)

	grouped := handler.WithGroup("import")
	require.NotEqual(t, handler, grouped)

	logger := slog.New(grouped)
	logger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "[import]")
	assert.Contains(t, output, "test message")
}

func TestCLIHandler_WithGroup_Empty(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)

	grouped := handler.WithGroup("")
	logger := slog.New(grouped)
	logger.Info("no prefix")

	output := buf.String()
	assert.NotContains(t, output, "] no prefix")
	assert.Contains(t, output, "no prefix")
}

func TestSetDefaultCLILogger(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	SetDefaultCLILogger("debug")

	defaultLogger := slog.Default()
	require.NotNil(t, defaultLogger)
	defaultLogger.Info("test message from default CLI logger")
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"  debug  ", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLogLevel(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCLIHandler_PrefixInOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := NewCLIHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler).WithGroup("server")

	logger.Info("hello")

	output := buf.String()
	assert.True(t, strings.Contains(output, "[server]"))
	assert.True(t, strings.Contains(output, "hello"))
}
