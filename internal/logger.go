package internal

import (
	"log/slog"
	"os"
)

var (
	Logger *slog.Logger
)

func InitLogger(debug bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	Logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
}