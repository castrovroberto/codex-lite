package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// AppConfig holds the application's global configuration.
type AppConfig struct {
	DefaultModel           string        `mapstructure:"default_model"`
	LogLevel               string        `mapstructure:"log_level"`
	OllamaHostURL          string        `mapstructure:"ollama_host_url"`
	OllamaRequestTimeout   time.Duration `mapstructure:"ollama_request_timeout"`
	OllamaKeepAlive        string        `mapstructure:"ollama_keep_alive"`
	MaxConcurrentAnalyzers int           `mapstructure:"max_concurrent_analyzers"`

	// Add other global settings here
}

// Sentinel errors for configuration loading.
var (
	ErrConfigFileNotFound   = errors.New("config: specified config file not found")
	ErrConfigReadPermission = errors.New("config: permission denied reading config file")
	ErrConfigUnmarshal      = errors.New("config: failed to unmarshal config data")
	ErrConfigRead           = errors.New("config: generic error reading config file")
)

var (
	Cfg  AppConfig
	once sync.Once
)

// LoadConfig loads configuration from file, environment variables, and defaults.
// It ensures this happens only once.
func LoadConfig(cfgFile string) error {
	var loadErr error
	once.Do(func() {
		// Set default values
		viper.SetDefault("default_model", "llama3:latest") // A sensible default
		viper.SetDefault("log_level", "info")
		viper.SetDefault("ollama_host_url", "http://localhost:11434") // Default Ollama URL
		viper.SetDefault("ollama_request_timeout", "120s")            // Default timeout for Ollama requests
		viper.SetDefault("ollama_keep_alive", "5m")                   // Default keep_alive for Ollama models

		if cfgFile != "" {
			// Use config file from the flag.
			viper.SetConfigFile(cfgFile)
		} else {
			// Search for config file in home directory and current directory.
			home, err := os.UserHomeDir()
			if err == nil {
				viper.AddConfigPath(filepath.Join(home, ".codex-lite")) // ~/.codex-lite/config.yaml
				viper.AddConfigPath(home)                               // ~/.codex-lite.yaml
			}
			viper.AddConfigPath(".")           // ./config.yaml or ./.codex-lite.yaml
			viper.SetConfigName("config")      // Default config file name (config.yaml, config.json etc.)
			viper.AddConfigPath(".codex-lite") // ./.codex-lite/config.yaml
			viper.SetConfigName(".codex-lite") // .codex-lite.yaml
		}

		viper.AutomaticEnv() // Read in environment variables that match
		viper.SetEnvPrefix("CODEXLITE")
		_ = viper.BindEnv("default_model", "CODEXLITE_DEFAULT_MODEL")
		_ = viper.BindEnv("log_level", "CODEXLITE_LOG_LEVEL")
		_ = viper.BindEnv("ollama_host_url", "CODEXLITE_OLLAMA_HOST_URL")
		_ = viper.BindEnv("ollama_request_timeout", "CODEXLITE_OLLAMA_REQUEST_TIMEOUT")
		_ = viper.BindEnv("ollama_keep_alive", "CODEXLITE_OLLAMA_KEEP_ALIVE")

		// Attempt to read the configuration file.
		if err := viper.ReadInConfig(); err != nil {
			var v ViperConfigFileNotFoundError // Alias for type assertion
			if errors.As(err, &v) {
				// Config file not found. This is only an error if a specific cfgFile was provided.
				if cfgFile != "" {
					loadErr = fmt.Errorf("%w: %s", ErrConfigFileNotFound, cfgFile)
					return
				}
				// If no specific cfgFile, it's okay if default files aren't found; defaults will be used.
			} else if os.IsPermission(err) {
				loadErr = fmt.Errorf("%w: %v", ErrConfigReadPermission, err)
				return
			} else {
				// For other types of read errors
				loadErr = fmt.Errorf("%w: %v", ErrConfigRead, err)
				return
			}
		}

		// Unmarshal the config into the Cfg struct.
		if err := viper.Unmarshal(&Cfg); err != nil {
			loadErr = fmt.Errorf("%w: %v", ErrConfigUnmarshal, err)
			return
		}

		// TODO: Add validation logic here for Cfg fields if necessary.
		// For example, check if OllamaHostURL is a valid URL.
	})
	return loadErr
}

// GetConfig returns the loaded application configuration.
// It ensures that LoadConfig has been called.
func GetConfig() AppConfig {
	if Cfg.OllamaHostURL == "" { // A simple check to see if config is initialized.
		// This might happen if GetConfig is called before LoadConfig (e.g. in tests or if LoadConfig fails silently)
		// For robustness, ensure LoadConfig is called if Cfg seems uninitialized,
		// though ideally LoadConfig is called once at startup.
		// Passing an empty cfgFile to use default search paths.
		err := LoadConfig("")
		if err != nil {
			// If LoadConfig fails here, it's a critical issue.
			// We might panic or log.Fatal, as the app can't run without config.
			// For now, we'll return the (partially) default Cfg, but log the error.
			// In a real app, you might have a logger available here or handle it more gracefully.
			fmt.Fprintf(os.Stderr, "Warning: GetConfig called before successful LoadConfig, or LoadConfig failed: %v\n", err)
			// Return Cfg which would have defaults set by viper.SetDefault even if file read/unmarshal failed.
		}
	}
	return Cfg
}

// ViperConfigFileNotFoundError is an alias for viper.ConfigFileNotFoundError
// This is used for type assertion with errors.As.
type ViperConfigFileNotFoundError = viper.ConfigFileNotFoundError
