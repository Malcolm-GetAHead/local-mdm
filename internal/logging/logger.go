package logging

import (
	"log/slog"
	"os"

	"github.com/malcolm-getahead/local-mdm/internal/config"
)

// New creates a new structured logger
func New(cfg config.LoggingConfig) *slog.Logger {
	var handler slog.Handler
	
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Level),
	}
	
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
