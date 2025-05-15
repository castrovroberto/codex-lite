package contextkeys

import (
	"context"
	"log/slog"

	"github.com/castrovroberto/codex-lite/internal/config" // Corrected path
	"github.com/castrovroberto/codex-lite/internal/logger" // Corrected path
)

// contextKey is an unexported type to prevent collisions when using context.WithValue.
type contextKey string

const (
	// ConfigKey is the key used to store and retrieve the AppConfig from context.
	ConfigKey contextKey = "appConfig"
	// LoggerKey is the key used to store and retrieve the *slog.Logger from context.
	LoggerKey contextKey = "logger"
)

func ConfigFromContext(ctx context.Context) config.AppConfig {
	val := ctx.Value(ConfigKey)
	if val == nil {
		panic("AppConfig not found in context. Ensure context is initialized with config.")
	}
	cfg, ok := val.(config.AppConfig)
	if !ok {
		panic("Value stored with ConfigKey is not of type config.AppConfig")
	}
	return cfg
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	val := ctx.Value(LoggerKey)
	if val == nil {
		return logger.Get()
	}
	log, ok := val.(*slog.Logger)
	if !ok {
		logger.Get().Error("Value stored with LoggerKey is not of type *slog.Logger, falling back to global logger")
		return logger.Get()
	}
	return log
}