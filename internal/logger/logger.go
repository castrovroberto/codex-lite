// internal/logger/logger.go
package logger

import (
	"io"
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

// InitLoggerForTUI initializes the logger to write to a file instead of stderr to avoid TUI interference
func InitLoggerForTUI(levelStr string, logFile string) error {
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

	// Create or open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Create a multi-writer that writes to both file and discard (no stderr during TUI)
	writer := io.MultiWriter(file, io.Discard)
	globalLogger = slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level}))

	return nil
}

// InitLoggerWithWriter initializes the logger with a custom writer
func InitLoggerWithWriter(levelStr string, writer io.Writer) {
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

	globalLogger = slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level}))
}

// Get returns the initialized global logger.
func Get() *slog.Logger {
	return globalLogger
}
