package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"
)

// CLIHandler is a custom slog.Handler for CLI output.
type CLIHandler struct {
	writer io.Writer
	level  slog.Level
	prefix string
	attrs  []slog.Attr
}

func NewCLIHandler(w io.Writer, level slog.Level) *CLIHandler {
	return &CLIHandler{
		writer: w,
		level:  level,
	}
}

func (h *CLIHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CLIHandler) Handle(_ context.Context, r slog.Record) error {
	msg := r.Message
	if h.prefix != "" {
		msg = "[" + h.prefix + "] " + msg
	}

	if len(h.attrs) > 0 || r.NumAttrs() > 0 {
		var attrs []string
		for _, a := range h.attrs {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value))
		}
		r.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value))
			return true
		})
		if len(attrs) > 0 {
			msg = msg + ": " + strings.Join(attrs, " ")
		}
	}

	switch {
	case r.Level >= slog.LevelError:
		msg = colorRed + msg + colorReset
	case r.Level >= slog.LevelWarn:
		msg = colorYellow + msg + colorReset
	default:
		msg = colorGreen + msg + colorReset
	}

	_, err := fmt.Fprintln(h.writer, msg)
	return err
}

func (h *CLIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CLIHandler{
		writer: h.writer,
		level:  h.level,
		prefix: h.prefix,
		attrs:  append(h.attrs, attrs...),
	}
}

func (h *CLIHandler) WithGroup(name string) slog.Handler {
	return &CLIHandler{
		writer: h.writer,
		level:  h.level,
		prefix: name,
	}
}

func NewCLILogger(level string) *slog.Logger {
	lev := ParseLogLevel(level)
	handler := NewCLIHandler(os.Stderr, lev)
	return slog.New(handler)
}

func SetDefaultCLILogger(level string) {
	slog.SetDefault(NewCLILogger(level))
}

func SetDefaultJSONLogger(level string) {
	lev := ParseLogLevel(level)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lev})
	slog.SetDefault(slog.New(handler))
}

// ParseLogLevel converts a string log level to slog.Level.
// Defaults to slog.LevelInfo for unrecognized strings.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
