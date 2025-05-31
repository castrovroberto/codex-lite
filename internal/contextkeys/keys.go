package contextkeys

import (
	"context"
	"log/slog"

	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/logger"
)

// Key type to avoid collisions in context values
type key int

const (
	// ConfigKey is the key for AppConfig in context
	ConfigKey key = iota
	// LoggerKey is the key for Logger in context
	LoggerKey
)

// ConfigFromContext retrieves AppConfig from context
// Returns zero value if not found - caller should handle this case
func ConfigFromContext(ctx context.Context) config.AppConfig {
	val := ctx.Value(ConfigKey)
	if val == nil {
		// Return zero value - callers should check if config is properly initialized
		return config.AppConfig{}
	}

	// Try pointer first
	if cfg, ok := val.(*config.AppConfig); ok {
		return *cfg
	}

	// Try value
	if cfg, ok := val.(config.AppConfig); ok {
		return cfg
	}

	// Return zero value if type assertion fails
	logger.Get().Warn("Value stored with ConfigKey is not of type config.AppConfig or *config.AppConfig")
	return config.AppConfig{}
}

// ConfigPtrFromContext retrieves a pointer to AppConfig from context
// Returns nil if not found - caller should handle this case
func ConfigPtrFromContext(ctx context.Context) *config.AppConfig {
	val := ctx.Value(ConfigKey)
	if val == nil {
		return nil
	}

	// Try pointer
	if cfg, ok := val.(*config.AppConfig); ok {
		return cfg
	}

	// Try value
	if cfg, ok := val.(config.AppConfig); ok {
		cfgCopy := cfg // Create a copy to avoid returning pointer to temporary
		return &cfgCopy
	}

	// Return nil if type assertion fails
	logger.Get().Warn("Value stored with ConfigKey is not of type config.AppConfig or *config.AppConfig")
	return nil
}

// LoggerFromContext retrieves Logger from context
func LoggerFromContext(ctx context.Context) *slog.Logger {
	val := ctx.Value(LoggerKey)
	if val == nil {
		return logger.Get()
	}
	log, ok := val.(*slog.Logger)
	if !ok {
		logger.Get().Warn("Value stored with LoggerKey is not of type *slog.Logger, using default logger")
		return logger.Get()
	}
	return log
}

/*
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
*/
