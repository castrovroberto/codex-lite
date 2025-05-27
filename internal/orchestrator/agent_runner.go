package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
)

// Message represents a message in the conversation history
type Message struct {
	Role       string            `json:"role"` // "system", "user", "assistant", "tool"
	Content    string            `json:"content"`
	ToolCall   *llm.FunctionCall `json:"tool_call,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Name       string            `json:"name,omitempty"` // Tool name for tool messages
}

// RunResult represents the result of an agent run
type RunResult struct {
	FinalResponse string    `json:"final_response"`
	Messages      []Message `json:"messages"`
	ToolCalls     int       `json:"tool_calls"`
	Iterations    int       `json:"iterations"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
}

// RunConfig represents configuration for a specific agent run
type RunConfig struct {
	MaxIterations     int
	AllowedTools      []string // If empty, all tools are allowed
	RequireTextOutput bool     // If true, ensures final response is text
	TimeoutSeconds    int      // Timeout for the entire run
}

// DefaultRunConfig returns default configuration
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:     10,
		AllowedTools:      []string{}, // All tools allowed by default
		RequireTextOutput: true,
		TimeoutSeconds:    300, // 5 minutes
	}
}

// PlanRunConfig returns configuration optimized for planning
func PlanRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:     5,                                                           // Planning should be quick
		AllowedTools:      []string{"read_file", "list_directory", "retrieve_context"}, // Limited tools for planning
		RequireTextOutput: true,
		TimeoutSeconds:    180, // 3 minutes
	}
}

// GenerateRunConfig returns configuration optimized for code generation
func GenerateRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:     15, // Generation might need more iterations
		AllowedTools:      []string{"read_file", "write_file", "list_directory", "apply_patch_to_file", "run_shell_command"},
		RequireTextOutput: false, // Generation might end with tool calls
		TimeoutSeconds:    600,   // 10 minutes
	}
}

// ReviewRunConfig returns configuration optimized for code review
func ReviewRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:     20, // Review might need many iterations
		AllowedTools:      []string{"read_file", "apply_patch_to_file", "run_tests", "run_linter", "parse_test_results"},
		RequireTextOutput: false,
		TimeoutSeconds:    900, // 15 minutes
	}
}

// AgentRunner manages the orchestration between LLM and tools
type AgentRunner struct {
	llmClient      llm.Client
	toolRegistry   *agent.Registry
	systemPrompt   string
	maxIterations  int
	model          string
	config         *RunConfig // Add configuration
	sessionManager *SessionManager
	currentSession *SessionState
}

// NewAgentRunner creates a new agent runner
func NewAgentRunner(llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt string, model string) *AgentRunner {
	return &AgentRunner{
		llmClient:     llmClient,
		toolRegistry:  toolRegistry,
		systemPrompt:  systemPrompt,
		maxIterations: 10, // Default max iterations
		model:         model,
		config:        DefaultRunConfig(),
	}
}

// NewAgentRunnerWithSession creates a new agent runner with session management
func NewAgentRunnerWithSession(llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt string, model string, sessionManager *SessionManager) *AgentRunner {
	return &AgentRunner{
		llmClient:      llmClient,
		toolRegistry:   toolRegistry,
		systemPrompt:   systemPrompt,
		maxIterations:  10, // Default max iterations
		model:          model,
		config:         DefaultRunConfig(),
		sessionManager: sessionManager,
	}
}

// SetConfig sets the run configuration
func (ar *AgentRunner) SetConfig(config *RunConfig) {
	ar.config = config
	ar.maxIterations = config.MaxIterations
}

// Run executes the agent orchestration loop
func (ar *AgentRunner) Run(ctx context.Context, initialPrompt string) (*RunResult, error) {
	return ar.RunWithCommand(ctx, initialPrompt, "unknown")
}

// RunWithCommand executes the agent orchestration loop with command tracking
func (ar *AgentRunner) RunWithCommand(ctx context.Context, initialPrompt string, command string) (*RunResult, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Apply timeout from configuration
	if ar.config.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(ar.config.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Initialize or resume session
	if ar.sessionManager != nil && ar.currentSession == nil {
		ar.currentSession = ar.sessionManager.CreateSession(ar.systemPrompt, ar.model, command, ar.config)
		log.Info("Created new session", "session_id", ar.currentSession.SessionID)
	}

	// Initialize message history
	messages := []Message{
		{Role: "system", Content: ar.systemPrompt},
		{Role: "user", Content: initialPrompt},
	}

	// If resuming a session, load existing messages
	if ar.currentSession != nil && len(ar.currentSession.Messages) > 0 {
		messages = ar.currentSession.Messages
		// Add new user prompt if different from last message
		if len(messages) == 0 || messages[len(messages)-1].Content != initialPrompt {
			messages = append(messages, Message{Role: "user", Content: initialPrompt})
		}
	}

	toolCalls := 0
	iterations := 0

	// Count existing tool calls if resuming
	if ar.currentSession != nil {
		toolCalls = len(ar.currentSession.ToolCalls)
		// Estimate iterations from message history
		for _, msg := range messages {
			if msg.Role == "assistant" {
				iterations++
			}
		}
	}

	log.Info("Starting agent orchestration", "max_iterations", ar.maxIterations, "session_id", func() string {
		if ar.currentSession != nil {
			return ar.currentSession.SessionID
		}
		return "none"
	}())

	// Main orchestration loop
	for iterations < ar.maxIterations {
		iterations++
		log.Debug("Agent iteration", "iteration", iterations)

		// Prepare tool definitions
		tools := ar.prepareToolDefinitions()

		// Call LLM with function calling support
		response, err := ar.llmClient.GenerateWithFunctions(
			ctx,
			ar.model,
			ar.buildPromptFromMessages(messages),
			"", // System prompt already in messages
			tools,
		)
		if err != nil {
			log.Error("LLM generation failed", "error", err, "iteration", iterations)
			return &RunResult{
				Messages:   messages,
				ToolCalls:  toolCalls,
				Iterations: iterations,
				Success:    false,
				Error:      fmt.Sprintf("LLM generation failed: %v", err),
			}, nil
		}

		// Process LLM response
		if response.IsTextResponse {
			// Text response - this might be the final answer
			log.Debug("Received text response from LLM", "content_length", len(response.TextContent))

			messages = append(messages, Message{
				Role:    "assistant",
				Content: response.TextContent,
			})

			// Check if this looks like a final answer
			if ar.isFinalAnswer(response.TextContent, iterations) {
				log.Info("Agent orchestration completed", "iterations", iterations, "tool_calls", toolCalls)

				// Update session state if available
				if ar.sessionManager != nil && ar.currentSession != nil {
					ar.sessionManager.UpdateSessionState(ar.currentSession, "completed")
					ar.currentSession.Messages = messages
					if err := ar.sessionManager.SaveSession(ar.currentSession); err != nil {
						log.Warn("Failed to save final session state", "error", err)
					}
				}

				return &RunResult{
					FinalResponse: response.TextContent,
					Messages:      messages,
					ToolCalls:     toolCalls,
					Iterations:    iterations,
					Success:       true,
				}, nil
			}
		} else {
			// Function call response
			functionCall := response.FunctionCall
			log.Debug("Received function call from LLM", "function", functionCall.Name)

			toolCalls++

			// Add the function call to message history
			messages = append(messages, Message{
				Role:     "assistant",
				Content:  "", // No content for function calls
				ToolCall: functionCall,
			})

			// Execute the tool with timing
			toolStartTime := time.Now()
			toolResult, err := ar.executeTool(ctx, functionCall)
			toolDuration := time.Since(toolStartTime)

			// Create tool call record for session tracking
			toolCallRecord := ToolCallRecord{
				ID:         functionCall.ID,
				Timestamp:  toolStartTime,
				ToolName:   functionCall.Name,
				Parameters: functionCall.Arguments,
				Duration:   toolDuration,
				Success:    err == nil && toolResult != nil && toolResult.Success,
				Iteration:  iterations,
			}

			if err != nil {
				log.Error("Tool execution failed", "tool", functionCall.Name, "error", err)
				toolCallRecord.Error = err.Error()

				// Add error message to history
				messages = append(messages, Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Error executing tool: %v", err),
					ToolCallID: functionCall.ID,
					Name:       functionCall.Name,
				})
			} else {
				// Add tool result to message history
				resultContent := ar.formatToolResult(toolResult)
				messages = append(messages, Message{
					Role:       "tool",
					Content:    resultContent,
					ToolCallID: functionCall.ID,
					Name:       functionCall.Name,
				})

				// Store tool result in record
				toolCallRecord.Result = &ToolCallResult{
					Success: toolResult.Success,
					Data:    toolResult.Data,
					Error:   toolResult.Error,
				}

				log.Debug("Tool executed successfully", "tool", functionCall.Name, "success", toolResult.Success)
			}

			// Add tool call to session if session manager is available
			if ar.sessionManager != nil && ar.currentSession != nil {
				ar.sessionManager.AddToolCall(ar.currentSession, toolCallRecord)
				// Update session messages
				ar.currentSession.Messages = messages
				// Save session state periodically
				if err := ar.sessionManager.SaveSession(ar.currentSession); err != nil {
					log.Warn("Failed to save session state", "error", err)
				}
			}
		}
	}

	// Max iterations reached
	log.Warn("Agent orchestration reached max iterations", "max_iterations", ar.maxIterations)

	// Update session state if available
	if ar.sessionManager != nil && ar.currentSession != nil {
		ar.sessionManager.UpdateSessionState(ar.currentSession, "failed")
		ar.currentSession.Messages = messages
		if err := ar.sessionManager.SaveSession(ar.currentSession); err != nil {
			log.Warn("Failed to save failed session state", "error", err)
		}
	}

	// Try to get a final response from the last assistant message
	finalResponse := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && messages[i].Content != "" {
			finalResponse = messages[i].Content
			break
		}
	}

	return &RunResult{
		FinalResponse: finalResponse,
		Messages:      messages,
		ToolCalls:     toolCalls,
		Iterations:    iterations,
		Success:       false,
		Error:         fmt.Sprintf("reached maximum iterations (%d) without final answer", ar.maxIterations),
	}, nil
}

// prepareToolDefinitions converts registry tools to LLM tool definitions
func (ar *AgentRunner) prepareToolDefinitions() []llm.ToolDefinition {
	allTools := ar.toolRegistry.List()

	// Filter tools based on configuration
	var filteredTools []agent.Tool
	if len(ar.config.AllowedTools) == 0 {
		// No filter, use all tools
		filteredTools = allTools
	} else {
		// Filter to only allowed tools
		allowedSet := make(map[string]bool)
		for _, toolName := range ar.config.AllowedTools {
			allowedSet[toolName] = true
		}

		for _, tool := range allTools {
			if allowedSet[tool.Name()] {
				filteredTools = append(filteredTools, tool)
			}
		}
	}

	definitions := make([]llm.ToolDefinition, len(filteredTools))
	for i, tool := range filteredTools {
		definitions[i] = llm.CreateToolDefinition(
			tool.Name(),
			tool.Description(),
			tool.Parameters(),
		)
	}

	return definitions
}

// buildPromptFromMessages builds a prompt string from message history
func (ar *AgentRunner) buildPromptFromMessages(messages []Message) string {
	var parts []string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// Skip system message as it's handled separately
			continue
		case "user":
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case "assistant":
			if msg.ToolCall != nil {
				parts = append(parts, fmt.Sprintf("Assistant: [Called tool: %s]", msg.ToolCall.Name))
			} else {
				parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
			}
		case "tool":
			parts = append(parts, fmt.Sprintf("Tool (%s): %s", msg.Name, msg.Content))
		}
	}

	return strings.Join(parts, "\n\n")
}

// executeTool executes a function call using the tool registry
func (ar *AgentRunner) executeTool(ctx context.Context, functionCall *llm.FunctionCall) (*agent.ToolResult, error) {
	// Look up tool in registry
	tool, exists := ar.toolRegistry.Get(functionCall.Name)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", functionCall.Name)
	}

	// Validate parameters (basic JSON validation)
	var params map[string]interface{}
	if err := json.Unmarshal(functionCall.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid tool parameters: %v", err)
	}

	// Execute tool with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := tool.Execute(toolCtx, functionCall.Arguments)
	if err != nil {
		return nil, fmt.Errorf("tool execution error: %v", err)
	}

	return result, nil
}

// formatToolResult formats a tool result for inclusion in message history
func (ar *AgentRunner) formatToolResult(result *agent.ToolResult) string {
	if !result.Success {
		return fmt.Sprintf("Error: %s", result.Error)
	}

	// Try to format the data nicely
	if result.Data != nil {
		if dataBytes, err := json.MarshalIndent(result.Data, "", "  "); err == nil {
			return string(dataBytes)
		}
		return fmt.Sprintf("%v", result.Data)
	}

	return "Tool executed successfully"
}

// isFinalAnswer determines if a text response should be treated as final
func (ar *AgentRunner) isFinalAnswer(content string, iteration int) bool {
	content = strings.ToLower(strings.TrimSpace(content))

	// If we're at max iterations, treat any text as final
	if iteration >= ar.maxIterations {
		return true
	}

	// Look for indicators that this is a final answer
	finalIndicators := []string{
		"task completed",
		"finished",
		"done",
		"complete",
		"successfully",
		"final result",
		"conclusion",
		"summary",
	}

	for _, indicator := range finalIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	// If the response is substantial (more than 100 chars) and doesn't seem to be asking for more tools
	if len(content) > 100 && !strings.Contains(content, "need to") && !strings.Contains(content, "should") {
		return true
	}

	return false
}

// GetMessageHistory returns the current message history
func (ar *AgentRunner) GetMessageHistory() []Message {
	if ar.currentSession != nil {
		return ar.currentSession.Messages
	}
	return []Message{}
}

// ResumeSession resumes an existing session
func (ar *AgentRunner) ResumeSession(sessionID string) error {
	if ar.sessionManager == nil {
		return fmt.Errorf("session manager not available")
	}

	session, err := ar.sessionManager.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Validate session compatibility
	if session.Model != ar.model {
		return fmt.Errorf("session model (%s) does not match current model (%s)", session.Model, ar.model)
	}

	if session.SystemPrompt != ar.systemPrompt {
		return fmt.Errorf("session system prompt does not match current system prompt")
	}

	// Update session state to running if it was paused
	if session.CurrentState == "paused" {
		ar.sessionManager.UpdateSessionState(session, "running")
	}

	ar.currentSession = session
	return nil
}

// PauseSession pauses the current session
func (ar *AgentRunner) PauseSession() error {
	if ar.sessionManager == nil || ar.currentSession == nil {
		return fmt.Errorf("no active session to pause")
	}

	ar.sessionManager.UpdateSessionState(ar.currentSession, "paused")
	return ar.sessionManager.SaveSession(ar.currentSession)
}

// GetCurrentSessionID returns the current session ID
func (ar *AgentRunner) GetCurrentSessionID() string {
	if ar.currentSession != nil {
		return ar.currentSession.SessionID
	}
	return ""
}

// GetSessionState returns the current session state
func (ar *AgentRunner) GetSessionState() *SessionState {
	return ar.currentSession
}
