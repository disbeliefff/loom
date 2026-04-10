package logger

import (
	"log/slog"
	"os"
)

// New creates and returns a new structured logger for the application.
func New(debug bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if debug {
		opts.Level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}
