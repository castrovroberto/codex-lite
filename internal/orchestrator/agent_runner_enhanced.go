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

// EnhancedAgentRunner manages the orchestration between LLM and tools with enhanced error handling
type EnhancedAgentRunner struct {
	llmClient      llm.Client
	toolRegistry   *agent.Registry
	systemPrompt   string
	maxIterations  int
	model          string
	config         *RunConfig
	sessionManager *SessionManager
	currentSession *SessionState

	// Enhanced error tracking
	toolAttempts   []ToolCallAttempt `json:"tool_attempts,omitempty"`
	errorHistory   map[string]int    `json:"error_history,omitempty"`   // Error code -> count
	currentRetries map[string]int    `json:"current_retries,omitempty"` // Tool call signature -> retry count
}

// NewEnhancedAgentRunner creates a new enhanced agent runner
func NewEnhancedAgentRunner(llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt string, model string) *EnhancedAgentRunner {
	return &EnhancedAgentRunner{
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

// NewEnhancedAgentRunnerWithSession creates a new enhanced agent runner with session management
func NewEnhancedAgentRunnerWithSession(llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt string, model string, sessionManager *SessionManager) *EnhancedAgentRunner {
	return &EnhancedAgentRunner{
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
func (ar *EnhancedAgentRunner) SetConfig(config *RunConfig) {
	ar.config = config
	ar.maxIterations = config.MaxIterations
}

// Run executes the agent orchestration loop with enhanced error handling
func (ar *EnhancedAgentRunner) Run(ctx context.Context, initialPrompt string) (*RunResult, error) {
	return ar.RunWithCommand(ctx, initialPrompt, "unknown")
}

// RunWithCommand executes the agent orchestration loop with command tracking and enhanced error handling
func (ar *EnhancedAgentRunner) RunWithCommand(ctx context.Context, initialPrompt string, command string) (*RunResult, error) {
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

	log.Info("Starting enhanced agent orchestration", "max_iterations", ar.maxIterations, "session_id", func() string {
		if ar.currentSession != nil {
			return ar.currentSession.SessionID
		}
		return "none"
	}())

	// Main orchestration loop with enhanced error handling
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

// Helper methods (reusing from original AgentRunner)

// prepareToolDefinitions converts registry tools to LLM tool definitions
func (ar *EnhancedAgentRunner) prepareToolDefinitions() []llm.ToolDefinition {
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
func (ar *EnhancedAgentRunner) buildPromptFromMessages(messages []Message) string {
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
func (ar *EnhancedAgentRunner) executeTool(ctx context.Context, functionCall *llm.FunctionCall) (*agent.ToolResult, error) {
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
func (ar *EnhancedAgentRunner) formatToolResult(result *agent.ToolResult) string {
	if !result.Success {
		if result.StandardizedError != nil {
			return result.StandardizedError.FormatForLLM()
		}
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
func (ar *EnhancedAgentRunner) isFinalAnswer(content string, iteration int) bool {
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

// Enhanced error handling methods

// getToolCallSignature returns a unique signature for a tool call
func (ar *EnhancedAgentRunner) getToolCallSignature(functionCall *llm.FunctionCall) string {
	return fmt.Sprintf("%s:%s", functionCall.Name, string(functionCall.Arguments))
}

// buildRetryPrompt builds a retry prompt for the LLM
func (ar *EnhancedAgentRunner) buildRetryPrompt(functionCall *llm.FunctionCall, toolResult *agent.ToolResult, attempt int) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("Your previous attempt to use the tool '%s' failed (attempt %d of %d).\n\n",
		functionCall.Name, attempt, ar.config.MaxToolRetries+1))

	if toolResult.StandardizedError != nil {
		prompt.WriteString("ERROR DETAILS:\n")
		prompt.WriteString(toolResult.StandardizedError.FormatForLLM())
		prompt.WriteString("\n\n")
	} else {
		prompt.WriteString(fmt.Sprintf("Error: %s\n\n", toolResult.Error))
	}

	prompt.WriteString("INSTRUCTIONS FOR RETRY:\n")
	prompt.WriteString("1. Carefully review the error message above\n")
	prompt.WriteString("2. Identify what went wrong with your parameters\n")
	prompt.WriteString("3. Provide a corrected tool call with the proper parameters\n")
	prompt.WriteString("4. If you cannot fix the issue, explain why and suggest an alternative approach\n\n")

	prompt.WriteString("Please provide your corrected response:")

	return prompt.String()
}

// shouldRetryToolCall determines if a tool call should be retried
func (ar *EnhancedAgentRunner) shouldRetryToolCall(toolResult *agent.ToolResult, retryCount int, callSignature string) bool {
	// Don't retry if we've exceeded max retries
	if retryCount >= ar.config.MaxToolRetries {
		return false
	}

	// Don't retry if retry modification is disabled
	if !ar.config.RetryWithModification {
		return false
	}

	// Don't retry certain types of errors
	if toolResult.StandardizedError != nil {
		errorCode := toolResult.StandardizedError.Code

		// Don't retry these error types as they're unlikely to be fixed by retry
		nonRetriableErrors := []agent.ToolErrorCode{
			agent.ErrorCodeUnsupportedOperation,
			agent.ErrorCodeInternalError,
			agent.ErrorCodeFileAlreadyExists, // Usually intentional
		}

		for _, nonRetriable := range nonRetriableErrors {
			if errorCode == nonRetriable {
				return false
			}
		}

		// Check for repeated errors if abort on repeated errors is enabled
		if ar.config.AbortOnRepeatedErrors {
			if count := ar.errorHistory[string(errorCode)]; count > 2 {
				return false // Same error happened too many times
			}
		}
	}

	return true
}

// GetMessageHistory returns the current message history
func (ar *EnhancedAgentRunner) GetMessageHistory() []Message {
	if ar.currentSession != nil {
		return ar.currentSession.Messages
	}
	return []Message{}
}

// GetErrorAnalytics returns error analytics for the current run
func (ar *EnhancedAgentRunner) GetErrorAnalytics() map[string]interface{} {
	return map[string]interface{}{
		"tool_attempts":   ar.toolAttempts,
		"error_history":   ar.errorHistory,
		"current_retries": ar.currentRetries,
		"total_attempts":  len(ar.toolAttempts),
		"failed_attempts": ar.countFailedAttempts(),
		"retry_rate":      ar.calculateRetryRate(),
	}
}

// countFailedAttempts counts the number of failed tool attempts
func (ar *EnhancedAgentRunner) countFailedAttempts() int {
	count := 0
	for _, attempt := range ar.toolAttempts {
		if attempt.ErrorMessage != "" {
			count++
		}
	}
	return count
}

// calculateRetryRate calculates the retry rate
func (ar *EnhancedAgentRunner) calculateRetryRate() float64 {
	if len(ar.toolAttempts) == 0 {
		return 0.0
	}

	retries := 0
	for _, attempt := range ar.toolAttempts {
		if attempt.Attempt > 1 {
			retries++
		}
	}

	return float64(retries) / float64(len(ar.toolAttempts))
}
