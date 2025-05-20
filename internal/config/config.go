package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// AppConfig holds the application's global configuration.
type AppConfig struct {
	DefaultModel                  string        `mapstructure:"default_model"`
	LogLevel                      string        `mapstructure:"log_level"`
	OllamaHostURL                 string        `mapstructure:"ollama_host_url"`
	OllamaRequestTimeout          time.Duration `mapstructure:"ollama_request_timeout"`
	OllamaKeepAlive               string        `mapstructure:"ollama_keep_alive"`
	ChatSystemPromptFile          string        `mapstructure:"chat_system_prompt_file"` // New: Path to the system prompt file for chat
	MaxAgentConcurrency           int           `mapstructure:"max_agent_concurrency"`
	AgentTimeout                  time.Duration `mapstructure:"agent_timeout"` // New: Timeout for individual agent execution
	WorkspaceRoot                 string        `mapstructure:"workspace_root"`
	loadedChatSystemPromptContent string        // Unexported field to store the loaded content

	// Add other global settings here
}

// GetLoadedChatSystemPrompt returns the content of the system prompt file after it has been loaded.
// It provides safe access to the unexported loadedChatSystemPromptContent field.
func (ac *AppConfig) GetLoadedChatSystemPrompt() string {
	return ac.loadedChatSystemPromptContent
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

const defaultInternalSystemPrompt = "You are a helpful AI assistant."

// resolvePath tries to resolve a path. If configFilePath is provided and path is relative,
// it attempts to resolve relative to the config file's directory.
// Otherwise, it tries to make it absolute based on the current working directory.
func resolvePath(path string, configFilePath string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	if configFilePath != "" {
		configDir := filepath.Dir(configFilePath)
		absPath := filepath.Join(configDir, path)
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
		// If not found relative to config, fall through to try CWD or return error if strict
	}
	// Default to trying to make it absolute from CWD
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to make path absolute: %w", err)
	}
	return absPath, nil
}

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
		viper.SetDefault("chat_system_prompt_file", "")               // Default to empty, meaning no external file unless specified
		viper.SetDefault("max_agent_concurrency", 1)                  // Default to 1 for sequential execution as per backlog task for new orchestrator logic
		viper.SetDefault("agent_timeout", "30s")                      // Default per-agent timeout
		viper.SetDefault("workspace_root", ".")                       // Default to current directory

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
			viper.AddConfigPath(".codex-lite") // ./.codex-lite/config.yaml

			// Try both config names
			viper.SetConfigName(".codex-lite")
			if err := viper.ReadInConfig(); err != nil {
				// If .codex-lite.yaml is not found, try config.yaml
				viper.SetConfigName("config")
			}
		}

		viper.AutomaticEnv() // Read in environment variables that match
		viper.SetEnvPrefix("CODEXLITE")
		_ = viper.BindEnv("default_model", "CODEXLITE_DEFAULT_MODEL")
		_ = viper.BindEnv("log_level", "CODEXLITE_LOG_LEVEL")
		_ = viper.BindEnv("ollama_host_url", "CODEXLITE_OLLAMA_HOST_URL")
		_ = viper.BindEnv("ollama_request_timeout", "CODEXLITE_OLLAMA_REQUEST_TIMEOUT")
		_ = viper.BindEnv("ollama_keep_alive", "CODEXLITE_OLLAMA_KEEP_ALIVE")
		_ = viper.BindEnv("chat_system_prompt_file", "CODEXLITE_CHAT_SYSTEM_PROMPT_FILE") // Env var for the file path
		_ = viper.BindEnv("max_agent_concurrency", "CODEXLITE_MAX_AGENT_CONCURRENCY")     // Bind new env var
		_ = viper.BindEnv("agent_timeout", "CODEXLITE_AGENT_TIMEOUT")                     // Bind new env var

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

		// Load chat system prompt from file if specified
		if Cfg.ChatSystemPromptFile != "" {
			// Resolve the path relative to the config file or absolute
			absPath, err := resolvePath(Cfg.ChatSystemPromptFile, viper.ConfigFileUsed())
			if err != nil {
				log.Printf("Warning: could not determine absolute path for chat_system_prompt_file: %v", err)
				// Potentially use Cfg.ChatSystemPromptFile as is, or handle error
				Cfg.loadedChatSystemPromptContent = defaultInternalSystemPrompt
			} else {
				content, err := os.ReadFile(absPath)
				if err != nil {
					log.Printf("Warning: could not read chat_system_prompt_file '%s': %v. Using default prompt.", absPath, err)
					Cfg.loadedChatSystemPromptContent = defaultInternalSystemPrompt // Use default
				} else {
					Cfg.loadedChatSystemPromptContent = string(content)
					log.Printf("Loaded chat system prompt from: %s", absPath)
				}
			}
		} else {
			Cfg.loadedChatSystemPromptContent = defaultInternalSystemPrompt // Use default if not specified
			log.Printf("chat_system_prompt_file not specified, using default internal system prompt.")
		}

		// Validate AgentTimeout
		if Cfg.AgentTimeout <= 0 {
			log.Printf("Warning: agent_timeout must be positive, setting to default (30s)")
			Cfg.AgentTimeout = 30 * time.Second
		}

		if Cfg.MaxAgentConcurrency < 1 {
			log.Printf("Warning: max_agent_concurrency must be at least 1, setting to 1 (sequential)")
			Cfg.MaxAgentConcurrency = 1
		} else if Cfg.MaxAgentConcurrency > 20 { // Arbitrary upper limit, can be adjusted
			log.Printf("Warning: max_agent_concurrency is too high (%d), setting to maximum (20)", Cfg.MaxAgentConcurrency)
			Cfg.MaxAgentConcurrency = 20
		}

		if Cfg.LogLevel != "" && !isValidLogLevel(Cfg.LogLevel) {
			log.Printf("Warning: invalid log_level '%s', setting to default (info)", Cfg.LogLevel)
			Cfg.LogLevel = "info"
		}
	})
	return loadErr
}

// isValidLogLevel checks if the provided log level is valid
func isValidLogLevel(level string) bool {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	return validLevels[strings.ToLower(level)]
}

// ViperConfigFileNotFoundError is specifically for viper's own ConfigFileNotFoundError
// This helps differentiate it from a general os.ErrNotExist if we were checking that directly.
type ViperConfigFileNotFoundError = viper.ConfigFileNotFoundError

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
