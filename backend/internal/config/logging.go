package config

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// SetupLogging configures slog to write JSON to stdout and plain text to a log file.
// Returns a closer for the log file (may be nil if file logging is disabled).
func SetupLogging() (io.Closer, error) {
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "logs/app.log"
	}

	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	stdoutHandler := slog.NewJSONHandler(os.Stdout, opts)

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		slog.SetDefault(slog.New(stdoutHandler))
		return nil, err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.SetDefault(slog.New(stdoutHandler))
		return nil, err
	}

	fileHandler := slog.NewTextHandler(f, opts)
	slog.SetDefault(slog.New(multiHandler{stdoutHandler, fileHandler}))
	slog.Info("file logging enabled", "path", logPath, "stdout_format", "json", "file_format", "text")
	return f, nil
}

type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h {
		if !handler.Enabled(ctx, r.Level) {
			continue
		}
		if err := handler.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(h))
	for i, handler := range h {
		out[i] = handler.WithAttrs(attrs)
	}
	return out
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(h))
	for i, handler := range h {
		out[i] = handler.WithGroup(name)
	}
	return out
}
