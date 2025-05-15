package config

import "context"

const DefaultOllamaHost = "http://localhost:11434"

type Config struct {
	OllamaHostURL string
	// Add other global configurations here later, e.g., DefaultModel
}

type configKeyType struct{}

var configKey = configKeyType{}

func FromContext(ctx context.Context) Config {
	if cfg, ok := ctx.Value(configKey).(Config); ok {
		return cfg
	}
	// Return default config if not found in context
	return Config{OllamaHostURL: DefaultOllamaHost}
}

// NewContextWithConfig can be added later when config loading is implemented
// func NewContextWithConfig(ctx context.Context, cfg Config) context.Context {
// 	return context.WithValue(ctx, configKey, cfg)
// }