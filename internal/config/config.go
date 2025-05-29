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

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/security"
	"github.com/spf13/viper"
)

// AppConfig holds the application's global configuration.
type AppConfig struct {
	Version string `mapstructure:"version"` // Version of the codex.toml configuration

	LLM struct {
		Provider              string        `mapstructure:"provider"`
		Model                 string        `mapstructure:"model"`
		RequestTimeoutSeconds time.Duration `mapstructure:"request_timeout_seconds"`
		OllamaHostURL         string        `mapstructure:"ollama_host_url"`        // Specific to Ollama, might be refactored
		OllamaKeepAlive       string        `mapstructure:"ollama_keep_alive"`      // Specific to Ollama
		OpenAIAPIKey          string        `mapstructure:"openai_api_key"`         // Loaded from env typically
		MaxTokensPerRequest   int           `mapstructure:"max_tokens_per_request"` // New
		RequestsPerMinute     int           `mapstructure:"requests_per_minute"`    // New
	} `mapstructure:"llm"`

	KGM struct {
		Enabled        bool   `mapstructure:"enabled"`
		Address        string `mapstructure:"address"`
		GraphitiAPIURL string `mapstructure:"graphiti_api_url"`
	} `mapstructure:"kgm"`

	Project struct {
		DefaultIgnoreDirs       []string `mapstructure:"default_ignore_dirs"`
		DefaultSourceExtensions []string `mapstructure:"default_source_extensions"`
		WorkspaceRoot           string   `mapstructure:"workspace_root"`
	} `mapstructure:"project"`

	Logging struct {
		Level   string `mapstructure:"level"`
		LogFile string `mapstructure:"log_file"`
	} `mapstructure:"logging"`

	Budget struct {
		RunBudgetUSD float64 `mapstructure:"run_budget_usd"` // New
	} `mapstructure:"budget"`

	Commands struct {
		Review struct {
			TestCommand string `mapstructure:"test_command"`
			LintCommand string `mapstructure:"lint_command"`
			MaxCycles   int    `mapstructure:"max_cycles"`
		} `mapstructure:"review"`
	} `mapstructure:"commands"`

	// Tools configuration for enhanced tool behavior
	Tools struct {
		ListDirectory struct {
			AllowOutsideWorkspace bool     `mapstructure:"allow_outside_workspace"`
			AllowedRoots          []string `mapstructure:"allowed_roots"`
			MaxDepthLimit         int      `mapstructure:"max_depth_limit"`
			MaxFilesLimit         int      `mapstructure:"max_files_limit"`
			AutoResolveSymlinks   bool     `mapstructure:"auto_resolve_symlinks"`
			SmartPathResolution   bool     `mapstructure:"smart_path_resolution"`
		} `mapstructure:"list_directory"`
	} `mapstructure:"tools"`

	// Old fields - to be reviewed/migrated or removed
	ChatSystemPromptFile          string        `mapstructure:"chat_system_prompt_file"`
	MaxAgentConcurrency           int           `mapstructure:"max_agent_concurrency"`
	AgentTimeout                  time.Duration `mapstructure:"agent_timeout"`
	loadedChatSystemPromptContent string        // Unexported field to store the loaded content
}

// GetLoadedChatSystemPrompt returns the content of the system prompt file after it has been loaded.
// It provides safe access to the unexported loadedChatSystemPromptContent field.
func (ac *AppConfig) GetLoadedChatSystemPrompt() string {
	return ac.loadedChatSystemPromptContent
}

// Configuration sub-structs for dependency injection

// OllamaConfig holds configuration specific to Ollama LLM client
type OllamaConfig struct {
	HostURL           string        `json:"host_url"`
	KeepAlive         string        `json:"keep_alive"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxTokens         int           `json:"max_tokens"`
	RequestsPerMinute int           `json:"requests_per_minute"`
}

// OpenAIConfig holds configuration specific to OpenAI LLM client
type OpenAIConfig struct {
	APIKey            string        `json:"api_key"`
	BaseURL           string        `json:"base_url"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxTokens         int           `json:"max_tokens"`
	RequestsPerMinute int           `json:"requests_per_minute"`
}

// IntegratorConfig holds configuration for command integrator
type IntegratorConfig struct {
	WorkspaceRoot string `json:"workspace_root"`
	PromptsDir    string `json:"prompts_dir"`
}

// Convenience methods to extract sub-configs from AppConfig

// GetOllamaConfig extracts Ollama-specific configuration
func (ac *AppConfig) GetOllamaConfig() OllamaConfig {
	return OllamaConfig{
		HostURL:           ac.LLM.OllamaHostURL,
		KeepAlive:         ac.LLM.OllamaKeepAlive,
		RequestTimeout:    ac.LLM.RequestTimeoutSeconds,
		MaxTokens:         ac.LLM.MaxTokensPerRequest,
		RequestsPerMinute: ac.LLM.RequestsPerMinute,
	}
}

// GetOpenAIConfig extracts OpenAI-specific configuration
func (ac *AppConfig) GetOpenAIConfig() OpenAIConfig {
	return OpenAIConfig{
		APIKey:            ac.LLM.OpenAIAPIKey,
		BaseURL:           "https://api.openai.com/v1", // Default, could be configurable
		RequestTimeout:    ac.LLM.RequestTimeoutSeconds,
		MaxTokens:         ac.LLM.MaxTokensPerRequest,
		RequestsPerMinute: ac.LLM.RequestsPerMinute,
	}
}

// GetIntegratorConfig extracts command integrator configuration
func (ac *AppConfig) GetIntegratorConfig() IntegratorConfig {
	return IntegratorConfig{
		WorkspaceRoot: ac.Project.WorkspaceRoot,
		PromptsDir:    filepath.Join(ac.Project.WorkspaceRoot, "prompts"),
	}
}

// GetListDirectoryConfig extracts list directory tool configuration
func (ac *AppConfig) GetListDirectoryConfig() agent.ListDirToolConfig {
	return agent.ListDirToolConfig{
		AllowOutsideWorkspace: ac.Tools.ListDirectory.AllowOutsideWorkspace,
		AllowedRoots:          ac.Tools.ListDirectory.AllowedRoots,
		MaxDepthLimit:         ac.Tools.ListDirectory.MaxDepthLimit,
		MaxFilesLimit:         ac.Tools.ListDirectory.MaxFilesLimit,
		AutoResolveSymlinks:   ac.Tools.ListDirectory.AutoResolveSymlinks,
		SmartPathResolution:   ac.Tools.ListDirectory.SmartPathResolution,
	}
}

// GetToolFactoryConfig extracts complete tool factory configuration
func (ac *AppConfig) GetToolFactoryConfig() agent.ToolFactoryConfig {
	listDirConfig := ac.GetListDirectoryConfig()
	return agent.ToolFactoryConfig{
		ListDirectory: &listDirConfig,
		// Future tool configs will be added here
	}
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
		// Set default values for CGE
		viper.SetDefault("version", "0.1.0")

		viper.SetDefault("llm.provider", "ollama")
		viper.SetDefault("llm.model", "llama3:latest")
		viper.SetDefault("llm.request_timeout_seconds", "300s")
		viper.SetDefault("llm.ollama_host_url", "http://localhost:11434")
		viper.SetDefault("llm.ollama_keep_alive", "5m")
		viper.SetDefault("llm.max_tokens_per_request", 4096) // Default based on common models
		viper.SetDefault("llm.requests_per_minute", 20)      // Default sensible RPM

		viper.SetDefault("kgm.enabled", false)
		viper.SetDefault("kgm.address", "http://localhost:7474") // Example Neo4j
		viper.SetDefault("kgm.graphiti_api_url", "http://localhost:8000/api")

		viper.SetDefault("project.workspace_root", ".")
		viper.SetDefault("project.default_ignore_dirs", []string{".git", ".idea", "node_modules", "vendor", "target", "dist", "build", "__pycache__", "*.pyc", "*.DS_Store"})
		viper.SetDefault("project.default_source_extensions", []string{".go", ".py", ".js", ".ts", ".java", ".md", ".rs", ".cpp", ".c", ".h", ".hpp", ".json", ".toml", ".yaml", ".yml"})

		viper.SetDefault("logging.level", "info")
		viper.SetDefault("logging.log_file", "cge.log") // Default log file

		viper.SetDefault("budget.run_budget_usd", 0.0) // No budget by default

		viper.SetDefault("commands.review.test_command", "")
		viper.SetDefault("commands.review.lint_command", "")
		viper.SetDefault("commands.review.max_cycles", 3)

		// Tools configuration defaults
		viper.SetDefault("tools.list_directory.allow_outside_workspace", false)
		viper.SetDefault("tools.list_directory.allowed_roots", []string{})
		viper.SetDefault("tools.list_directory.max_depth_limit", 10)
		viper.SetDefault("tools.list_directory.max_files_limit", 1000)
		viper.SetDefault("tools.list_directory.auto_resolve_symlinks", false)
		viper.SetDefault("tools.list_directory.smart_path_resolution", true)

		// Defaults for old fields (to be reviewed)
		viper.SetDefault("chat_system_prompt_file", "")
		viper.SetDefault("max_agent_concurrency", 1)
		viper.SetDefault("agent_timeout", "60s") // Increased default

		if cfgFile != "" {
			// Use config file from the flag.
			viper.SetConfigFile(cfgFile)
			viper.SetConfigType("toml") // Explicitly set TOML type
		} else {
			// Search for config file in home directory and current directory.
			home, err := os.UserHomeDir()
			if err == nil {
				viper.AddConfigPath(filepath.Join(home, ".cge")) // ~/.cge/codex.toml
				viper.AddConfigPath(home)                        // ~/.codex.toml
			}
			viper.AddConfigPath(".") // ./codex.toml

			viper.SetConfigName("codex") // Name of config file (without extension)
			viper.SetConfigType("toml")  // Expect TOML
		}

		viper.AutomaticEnv()                                   // Read in environment variables that match
		viper.SetEnvPrefix("CGE")                              // New prefix for CGE
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Replace . with _ for nested keys

		// Bind environment variables for new CGE structure
		// Example: CGE_LLM_PROVIDER, CGE_KGM_ENABLED
		// Viper will automatically bind mapstructure tags if env vars match uppercased key + prefix
		// e.g., CGE_LLM_PROVIDER for AppConfig.LLM.Provider

		// Specific binding for OpenAI API Key as it's sensitive
		_ = viper.BindEnv("llm.openai_api_key", "OPENAI_API_KEY")

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
				log.Println("No configuration file found. Using default values and environment variables.")
			} else if os.IsPermission(err) {
				loadErr = fmt.Errorf("%w: %v", ErrConfigReadPermission, err)
				return
			} else {
				// For other types of read errors (e.g., parsing error)
				loadErr = fmt.Errorf("error reading config file '%s': %w", viper.ConfigFileUsed(), err)
				return
			}
		} else {
			log.Printf("Using configuration file: %s", viper.ConfigFileUsed())
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
				// Get directory containing the config file as allowed root
				configDir := filepath.Dir(viper.ConfigFileUsed())
				if configDir == "" {
					// If no config file was used, use current directory
					configDir, _ = os.Getwd()
				}

				// Create safe file operations with config directory as allowed root
				safeOps := security.NewSafeFileOps(configDir)

				content, err := safeOps.SafeReadFile(absPath)
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
			log.Printf("Warning: agent_timeout must be positive, setting to default (60s)")
			Cfg.AgentTimeout = 60 * time.Second
		}

		if Cfg.MaxAgentConcurrency < 1 {
			log.Printf("Warning: max_agent_concurrency must be at least 1, setting to 1 (sequential)")
			Cfg.MaxAgentConcurrency = 1
		} else if Cfg.MaxAgentConcurrency > 20 { // Arbitrary upper limit, can be adjusted
			log.Printf("Warning: max_agent_concurrency is too high (%d), setting to maximum (20)", Cfg.MaxAgentConcurrency)
			Cfg.MaxAgentConcurrency = 20
		}

		if Cfg.Logging.Level != "" && !isValidLogLevel(Cfg.Logging.Level) {
			log.Printf("Warning: invalid log_level '%s', setting to default (info)", Cfg.Logging.Level)
			Cfg.Logging.Level = "info"
		}

		// Validate LLM request timeout
		if Cfg.LLM.RequestTimeoutSeconds <= 0 {
			log.Printf("Warning: llm.request_timeout_seconds must be positive, setting to default (300s)")
			Cfg.LLM.RequestTimeoutSeconds = 300 * time.Second
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
	if Cfg.LLM.OllamaHostURL == "" { // A simple check to see if config is initialized.
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
