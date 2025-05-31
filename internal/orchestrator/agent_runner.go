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

	// Enhanced error tracking
	ToolRetries  int      `json:"tool_retries,omitempty"`
	ErrorDetails []string `json:"error_details,omitempty"`
}

// RunConfig represents configuration for a specific agent run
type RunConfig struct {
	MaxIterations     int
	AllowedTools      []string // If empty, all tools are allowed
	RequireTextOutput bool     // If true, ensures final response is text
	TimeoutSeconds    int      // Timeout for the entire run

	// Enhanced error handling configuration
	MaxToolRetries        int  `json:"max_tool_retries"`         // Max retries per tool call
	RetryWithModification bool `json:"retry_with_modification"`  // Enable retry prompting
	EnableErrorAnalysis   bool `json:"enable_error_analysis"`    // Enable enhanced error formatting
	AbortOnRepeatedErrors bool `json:"abort_on_repeated_errors"` // Abort if same error repeats
}

// DefaultRunConfig returns default configuration
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:         10,
		AllowedTools:          []string{}, // All tools allowed by default
		RequireTextOutput:     true,
		TimeoutSeconds:        300, // 5 minutes
		MaxToolRetries:        2,   // Allow 2 retries per tool call
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: false,
	}
}

// PlanRunConfig returns configuration optimized for planning
func PlanRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:         5,                                                           // Planning should be quick
		AllowedTools:          []string{"read_file", "list_directory", "retrieve_context"}, // Limited tools for planning
		RequireTextOutput:     true,
		TimeoutSeconds:        180, // 3 minutes
		MaxToolRetries:        1,   // Fewer retries for planning
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: true, // Abort quickly for planning
	}
}

// GenerateRunConfig returns configuration optimized for code generation
func GenerateRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:         15, // Generation might need more iterations
		AllowedTools:          []string{"read_file", "write_file", "list_directory", "apply_patch_to_file", "run_shell_command"},
		RequireTextOutput:     false, // Generation might end with tool calls
		TimeoutSeconds:        600,   // 10 minutes
		MaxToolRetries:        3,     // More retries for generation
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: false,
	}
}

// ReviewRunConfig returns configuration optimized for code review
func ReviewRunConfig() *RunConfig {
	return &RunConfig{
		MaxIterations:         20, // Review might need many iterations
		AllowedTools:          []string{"read_file", "apply_patch_to_file", "run_tests", "run_linter", "parse_test_results"},
		RequireTextOutput:     false,
		TimeoutSeconds:        900, // 15 minutes
		MaxToolRetries:        2,   // Standard retries for review
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: true, // Abort on repeated errors in review
	}
}

// ToolCallAttempt tracks individual tool call attempts for retry logic
type ToolCallAttempt struct {
	ToolName     string    `json:"tool_name"`
	Attempt      int       `json:"attempt"`
	Parameters   string    `json:"parameters"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
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

	// Enhanced error tracking
	toolAttempts   []ToolCallAttempt `json:"tool_attempts,omitempty"`
	errorHistory   map[string]int    `json:"error_history,omitempty"`   // Error code -> count
	currentRetries map[string]int    `json:"current_retries,omitempty"` // Tool call signature -> retry count
}

// NewAgentRunner creates a new agent runner
func NewAgentRunner(llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt string, model string) *AgentRunner {
	return &AgentRunner{
		llmClient:      llmClient,
		toolRegistry:   toolRegistry,
		systemPrompt:   systemPrompt,
		maxIterations:  10, // Default max iterations
		model:          model,
		config:         DefaultRunConfig(),
		toolAttempts:   make([]ToolCallAttempt, 0),
		errorHistory:   make(map[string]int),
		currentRetries: make(map[string]int),
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
		toolAttempts:   make([]ToolCallAttempt, 0),
		errorHistory:   make(map[string]int),
		currentRetries: make(map[string]int),
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
	totalRetries := 0
	iterations := 0
	errorDetails := make([]string, 0)

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
				FinalResponse: "",
				Messages:      messages,
				ToolCalls:     toolCalls,
				Iterations:    iterations,
				Success:       false,
				Error:         fmt.Sprintf("LLM generation failed: %v", err),
				ToolRetries:   totalRetries,
				ErrorDetails:  errorDetails,
			}, nil
		}

		// Process the response
		if response.IsTextResponse {
			// Text response
			newMessage := Message{
				Role:    "assistant",
				Content: response.TextContent,
			}
			messages = append(messages, newMessage)

			// Update session if available
			if ar.currentSession != nil {
				ar.currentSession.Messages = messages

				// TODO: Fix type conflicts between local ToolCallRecord and session_manager.ToolCallRecord
				// For now, just save messages
				ar.sessionManager.SaveSession(ar.currentSession)
			}

			// Check if this should be treated as final
			if ar.isFinalAnswer(response.TextContent, iterations) {
				log.Info("Agent completed with text response", "iterations", iterations, "tool_calls", toolCalls)
				return &RunResult{
					FinalResponse: response.TextContent,
					Messages:      messages,
					ToolCalls:     toolCalls,
					Iterations:    iterations,
					Success:       true,
					ToolRetries:   totalRetries,
					ErrorDetails:  errorDetails,
				}, nil
			}
		} else {
			// Function call - handle with enhanced error retry logic
			functionCall := response.FunctionCall
			callSignature := ar.getToolCallSignature(functionCall)

			// Add function call message
			callMessage := Message{
				Role:     "assistant",
				Content:  "",
				ToolCall: functionCall,
			}
			messages = append(messages, callMessage)

			// Track the attempt
			attempt := ToolCallAttempt{
				ToolName:   functionCall.Name,
				Attempt:    ar.currentRetries[callSignature] + 1,
				Parameters: string(functionCall.Arguments),
				Timestamp:  time.Now(),
			}

			// Execute tool with enhanced error handling
			toolResult, executionErr := ar.executeTool(ctx, functionCall)
			if executionErr != nil {
				// Internal execution error (tool not found, etc.)
				errorMsg := fmt.Sprintf("Tool execution error: %v", executionErr)
				attempt.ErrorMessage = errorMsg
				ar.toolAttempts = append(ar.toolAttempts, attempt)

				resultMessage := Message{
					Role:       "tool",
					ToolCallID: functionCall.ID,
					Name:       functionCall.Name,
					Content:    errorMsg,
				}
				messages = append(messages, resultMessage)
				errorDetails = append(errorDetails, errorMsg)
				continue
			}

			toolCalls++

			// Handle tool result with retry logic
			if !toolResult.Success {
				// Tool returned an error
				retryCount := ar.currentRetries[callSignature]

				// Update attempt with error information
				if toolResult.StandardizedError != nil {
					attempt.ErrorCode = string(toolResult.StandardizedError.Code)
					attempt.ErrorMessage = toolResult.StandardizedError.Error()

					// Track error frequency
					ar.errorHistory[string(toolResult.StandardizedError.Code)]++
				} else {
					attempt.ErrorMessage = toolResult.Error
				}
				ar.toolAttempts = append(ar.toolAttempts, attempt)

				// Check if we should retry
				if ar.shouldRetryToolCall(toolResult, retryCount, callSignature) {
					ar.currentRetries[callSignature] = retryCount + 1
					totalRetries++

					// Create retry prompt for LLM
					retryPrompt := ar.buildRetryPrompt(functionCall, toolResult, retryCount+1)

					resultMessage := Message{
						Role:       "tool",
						ToolCallID: functionCall.ID,
						Name:       functionCall.Name,
						Content:    retryPrompt,
					}
					messages = append(messages, resultMessage)

					log.Debug("Retrying tool call", "tool", functionCall.Name, "attempt", retryCount+1, "error", toolResult.Error)
					errorDetails = append(errorDetails, fmt.Sprintf("Retry %d for %s: %s", retryCount+1, functionCall.Name, toolResult.Error))
				} else {
					// No more retries, format error for LLM
					var errorContent string
					if ar.config.EnableErrorAnalysis && toolResult.StandardizedError != nil {
						errorContent = toolResult.StandardizedError.FormatForLLM()
					} else {
						errorContent = fmt.Sprintf("Error: %s", toolResult.Error)
					}

					resultMessage := Message{
						Role:       "tool",
						ToolCallID: functionCall.ID,
						Name:       functionCall.Name,
						Content:    errorContent,
					}
					messages = append(messages, resultMessage)
					errorDetails = append(errorDetails, fmt.Sprintf("Final error for %s: %s", functionCall.Name, toolResult.Error))
				}
			} else {
				// Successful tool execution
				attempt.ErrorCode = ""
				attempt.ErrorMessage = ""
				ar.toolAttempts = append(ar.toolAttempts, attempt)

				// Reset retry count for this tool call signature
				delete(ar.currentRetries, callSignature)

				resultMessage := Message{
					Role:       "tool",
					ToolCallID: functionCall.ID,
					Name:       functionCall.Name,
					Content:    ar.formatToolResult(toolResult),
				}
				messages = append(messages, resultMessage)
			}

			// Update session if available
			if ar.currentSession != nil {
				ar.currentSession.Messages = messages

				// TODO: Fix type conflicts between local ToolCallRecord and session_manager.ToolCallRecord
				// For now, just save messages
				ar.sessionManager.SaveSession(ar.currentSession)
			}
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			log.Info("Agent run cancelled", "reason", ctx.Err())
			return &RunResult{
				FinalResponse: "",
				Messages:      messages,
				ToolCalls:     toolCalls,
				Iterations:    iterations,
				Success:       false,
				Error:         fmt.Sprintf("cancelled: %v", ctx.Err()),
				ToolRetries:   totalRetries,
				ErrorDetails:  errorDetails,
			}, nil
		default:
		}
	}

	// Max iterations reached
	log.Warn("Agent reached max iterations", "max_iterations", ar.maxIterations)

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
		Error:         fmt.Sprintf("reached maximum iterations (%d)", ar.maxIterations),
		ToolRetries:   totalRetries,
		ErrorDetails:  errorDetails,
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

// getToolCallSignature returns a unique signature for a tool call
func (ar *AgentRunner) getToolCallSignature(functionCall *llm.FunctionCall) string {
	return fmt.Sprintf("%s:%s", functionCall.Name, string(functionCall.Arguments))
}

// buildRetryPrompt builds a retry prompt for the LLM
func (ar *AgentRunner) buildRetryPrompt(functionCall *llm.FunctionCall, toolResult *agent.ToolResult, attempt int) string {
	retryPrompt := fmt.Sprintf("Retry %d for %s", attempt, functionCall.Name)
	if toolResult.StandardizedError != nil {
		retryPrompt += fmt.Sprintf(": %s", toolResult.StandardizedError.Error())
	} else {
		retryPrompt += fmt.Sprintf(": %s", toolResult.Error)
	}
	retryPrompt += "\n\nPlease provide a revised response or additional information to proceed."
	return retryPrompt
}

// shouldRetryToolCall determines if a tool call should be retried
func (ar *AgentRunner) shouldRetryToolCall(toolResult *agent.ToolResult, retryCount int, callSignature string) bool {
	if retryCount >= ar.config.MaxToolRetries {
		return false
	}

	if ar.config.RetryWithModification && toolResult.StandardizedError != nil {
		return true
	}

	return false
}
