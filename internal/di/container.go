package di

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
	"github.com/castrovroberto/CGE/internal/tui/chat"
)

// Container holds all application dependencies
type Container struct {
	config       *config.AppConfig
	fileSystem   FileSystemService
	cmdExecutor  CommandExecutor
	httpClient   HTTPClient
	sessionStore SessionStore

	// Services (built lazily)
	llmClient    llm.Client
	toolRegistry *agent.Registry
	agentRunner  *orchestrator.AgentRunner
}

// NewContainer creates a new dependency injection container
func NewContainer(cfg *config.AppConfig) *Container {
	return &Container{
		config: cfg,
		// Use real implementations by default
		fileSystem:   &RealFileSystemService{},
		cmdExecutor:  &RealCommandExecutor{},
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		sessionStore: NewRealSessionStore(),
	}
}

// WithFileSystem allows injection of custom FileSystemService (for testing)
func (c *Container) WithFileSystem(fs FileSystemService) *Container {
	c.fileSystem = fs
	return c
}

// WithCommandExecutor allows injection of custom CommandExecutor (for testing)
func (c *Container) WithCommandExecutor(exec CommandExecutor) *Container {
	c.cmdExecutor = exec
	return c
}

// WithHTTPClient allows injection of custom HTTP client (for testing)
func (c *Container) WithHTTPClient(client HTTPClient) *Container {
	c.httpClient = client
	return c
}

// WithSessionStore allows injection of custom SessionStore (for testing)
func (c *Container) WithSessionStore(store SessionStore) *Container {
	c.sessionStore = store
	return c
}

// GetLLMClient returns the configured LLM client
func (c *Container) GetLLMClient() llm.Client {
	if c.llmClient == nil {
		c.llmClient = c.buildLLMClient()
	}
	return c.llmClient
}

// GetToolRegistry returns the configured tool registry
func (c *Container) GetToolRegistry() *agent.Registry {
	if c.toolRegistry == nil {
		c.toolRegistry = c.buildToolRegistry()
	}
	return c.toolRegistry
}

// GetAgentRunner returns the configured agent runner
func (c *Container) GetAgentRunner(systemPrompt, modelName string) *orchestrator.AgentRunner {
	if c.agentRunner == nil {
		c.agentRunner = c.buildAgentRunner(systemPrompt, modelName)
	}
	return c.agentRunner
}

// GetChatService returns a configured chat service
func (c *Container) GetChatService(modelName, systemPrompt string) chat.ChatService {
	return chat.NewRealChatService(
		c.GetLLMClient(),
		c.GetToolRegistry(),
		modelName,
		systemPrompt,
	)
}

// GetChatPresenter returns a configured chat presenter
func (c *Container) GetChatPresenter(ctx context.Context, modelName, systemPrompt string) chat.MessageProvider {
	return chat.NewChatPresenter(
		ctx,
		c.GetLLMClient(),
		c.GetToolRegistry(),
		systemPrompt,
		modelName,
	)
}

// buildLLMClient creates the appropriate LLM client based on configuration
func (c *Container) buildLLMClient() llm.Client {
	switch c.config.LLM.Provider {
	case "ollama":
		config := c.config.GetOllamaConfig()
		return llm.NewOllamaClient(config)
	case "openai":
		config := c.config.GetOpenAIConfig()
		return llm.NewOpenAIClient(config)
	default:
		// Fallback to ollama
		config := c.config.GetOllamaConfig()
		return llm.NewOllamaClient(config)
	}
}

// buildToolRegistry creates a tool registry with dependency injection
func (c *Container) buildToolRegistry() *agent.Registry {
	workspaceRoot := c.config.Project.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = "."
	}

	absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absWorkspaceRoot = workspaceRoot
	}

	// Create enhanced tool factory with dependency injection
	enhancedFactory := agent.NewEnhancedToolFactory(
		absWorkspaceRoot,
		c.fileSystem,
		c.cmdExecutor,
	)

	return enhancedFactory.CreateGenerationRegistry()
}

// buildAgentRunner creates an agent runner with session management
func (c *Container) buildAgentRunner(systemPrompt, modelName string) *orchestrator.AgentRunner {
	// For now, use the existing AgentRunner constructor
	// We'll enhance it with session management later
	return orchestrator.NewAgentRunner(
		c.GetLLMClient(),
		c.GetToolRegistry(),
		systemPrompt,
		modelName,
	)
}

// Config returns the application configuration
func (c *Container) Config() *config.AppConfig {
	return c.config
}
