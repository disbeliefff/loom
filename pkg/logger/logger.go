package logger

import (
	"log/slog"
	"os"
)

// Init configures the default structured logger for the application.
func Init(debug bool) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if debug {
		opts.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}
