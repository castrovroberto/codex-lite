// internal/config/config.go
package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// AppConfig holds application-wide configurations.
// Viper uses mapstructure tags to unmarshal.
type AppConfig struct {
	OllamaHostURL       string        `mapstructure:"ollama_host_url"`
	DefaultModel        string        `mapstructure:"default_model"`
	LogLevel            string        `mapstructure:"log_level"`
	OllamaRequestTimeout time.Duration `mapstructure:"ollama_request_timeout"`
}

// Cfg is the global application configuration.
var Cfg AppConfig

// LoadConfig reads configuration from file, environment variables.
// Flags are typically bound in cmd/root.go or specific command initializations.
func LoadConfig(cfgFile string) error {
	// Set default values
	viper.SetDefault("ollama_host_url", "http://localhost:11434")
	viper.SetDefault("default_model", "deepseek-coder-v2:lite")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("ollama_request_timeout", 60*time.Second) // Increased default timeout

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile) // Use config file from the flag.
	} else {
		viper.AddConfigPath(".") // Look for config in the current directory
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home) // Then in the home directory
		}
		viper.SetConfigName(".codex-lite") // Name of config file (without extension)
		viper.SetConfigType("yaml")        // Can be yaml, json, toml, etc.
	}

	viper.AutomaticEnv()                                // Read in environment variables that match
	viper.SetEnvPrefix("CODEXLITE")                     // Env vars should be prefixed with CODEXLITE_
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_")) // e.g. ollama_host_url -> CODEXLITE_OLLAMA_HOST_URL

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok || cfgFile != "" {
			// Config file not found, or explicitly set and not found
			return fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, but not explicitly set, so defaults will be used. This is fine.
	}

	return viper.Unmarshal(&Cfg)
}

// FromContext retrieves the AppConfig from the context.
// For simplicity, this now returns the global Cfg.
func FromContext(ctx context.Context) AppConfig {
	return Cfg
}