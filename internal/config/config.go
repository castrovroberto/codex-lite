// internal/config/config.go
package config

import "context"

// AppConfig holds application-wide configurations.
type AppConfig struct {
	OllamaHostURL string
	// Add other configurations here as needed (e.g., DefaultModel)
}

// configKey is an unexported type for context keys.
type configKey struct{}

// NewContext returns a new context with the provided AppConfig.
func NewContext(ctx context.Context, cfg AppConfig) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// FromContext retrieves the AppConfig from the context.
// It panics if the AppConfig is not found, as it's considered a programming error
// (AppConfig should always be set up at the application's entry point).
func FromContext(ctx context.Context) AppConfig {
	cfg, ok := ctx.Value(configKey{}).(AppConfig)
	if !ok {
		panic("config.AppConfig not found in context")
	}
	return cfg
}