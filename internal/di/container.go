package di

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	contextutil "github.com/castrovroberto/CGE/internal/context"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
	"github.com/castrovroberto/CGE/internal/tui/chat"
)

// Container holds all application dependencies
type Container struct {
	config           *config.AppConfig
	fileSystem       FileSystemService
	cmdExecutor      CommandExecutor
	httpClient       HTTPClient
	sessionStore     SessionStore
	absWorkspaceRoot string // Always absolute workspace root

	// Services (built lazily)
	llmClient         llm.Client
	toolRegistry      *agent.Registry
	agentRunner       *orchestrator.AgentRunner
	contextIntegrator *contextutil.ContextIntegrator
}

// NewContainer creates a new dependency injection container
func NewContainer(cfg *config.AppConfig) *Container {
	// Ensure workspace root is absolute
	workspaceRoot := cfg.Project.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = "."
	}

	absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		// Fallback to original if conversion fails
		absWorkspaceRoot = workspaceRoot
	}

	return &Container{
		config:           cfg,
		absWorkspaceRoot: absWorkspaceRoot,
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

// GetContextIntegrator returns the configured context integrator
func (c *Container) GetContextIntegrator() *contextutil.ContextIntegrator {
	if c.contextIntegrator == nil {
		c.contextIntegrator = c.buildContextIntegrator()
	}
	return c.contextIntegrator
}

// GetAbsoluteWorkspaceRoot returns the absolute workspace root path
func (c *Container) GetAbsoluteWorkspaceRoot() string {
	return c.absWorkspaceRoot
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

// GetChatPresenterWithContext returns a configured chat presenter with workspace context
func (c *Container) GetChatPresenterWithContext(ctx context.Context, modelName, systemPrompt string) chat.MessageProvider {
	// Gather workspace context and prepend to system prompt
	contextIntegrator := c.GetContextIntegrator()
	workspaceContext, err := contextIntegrator.GatherWorkspaceContext(ctx)

	enhancedSystemPrompt := systemPrompt
	if err == nil {
		contextualInfo := contextIntegrator.FormatContextForPrompt(workspaceContext)
		enhancedSystemPrompt = contextualInfo + "\n" + systemPrompt
	}

	return chat.NewChatPresenter(
		ctx,
		c.GetLLMClient(),
		c.GetToolRegistry(),
		enhancedSystemPrompt,
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
	case "gemini":
		config := c.config.GetGeminiConfig()
		return llm.NewGeminiClient(config)
	default:
		// Fallback to ollama
		config := c.config.GetOllamaConfig()
		return llm.NewOllamaClient(config)
	}
}

// buildToolRegistry creates a tool registry with dependency injection
func (c *Container) buildToolRegistry() *agent.Registry {
	// Create enhanced tool factory with dependency injection using absolute workspace root
	enhancedFactory := agent.NewEnhancedToolFactory(
		c.absWorkspaceRoot,
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

// buildContextIntegrator creates a context integrator with the tool registry
func (c *Container) buildContextIntegrator() *contextutil.ContextIntegrator {
	return contextutil.NewContextIntegrator(c.absWorkspaceRoot, c.GetToolRegistry())
}

// Config returns the application configuration
func (c *Container) Config() *config.AppConfig {
	return c.config
}
