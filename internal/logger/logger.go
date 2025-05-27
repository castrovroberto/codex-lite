// internal/logger/logger.go
package logger

import (
	"log/slog"
	"os"
	"strings"
)

var globalLogger *slog.Logger

func init() {
	// Initialize with a default logger until InitLogger is called.
	// This ensures that Get() always returns a valid logger.
	globalLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// InitLogger initializes the global logger with the specified log level.
func InitLogger(levelStr string) {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error", "err":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // Default to info for unknown levels
	}

	globalLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// Get returns the initialized global logger.
func Get() *slog.Logger {
	return globalLogger
}
