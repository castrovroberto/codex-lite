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
	llmClient     llm.Client
	toolRegistry  *agent.Registry
	systemPrompt  string
	maxIterations int
	model         string
	config        *RunConfig // Add configuration
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

// SetConfig sets the run configuration
func (ar *AgentRunner) SetConfig(config *RunConfig) {
	ar.config = config
	ar.maxIterations = config.MaxIterations
}

// Run executes the agent orchestration loop
func (ar *AgentRunner) Run(ctx context.Context, initialPrompt string) (*RunResult, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Apply timeout from configuration
	if ar.config.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(ar.config.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Initialize message history
	messages := []Message{
		{Role: "system", Content: ar.systemPrompt},
		{Role: "user", Content: initialPrompt},
	}

	toolCalls := 0
	iterations := 0

	log.Info("Starting agent orchestration", "max_iterations", ar.maxIterations)

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

			// Execute the tool
			toolResult, err := ar.executeTool(ctx, functionCall)
			if err != nil {
				log.Error("Tool execution failed", "tool", functionCall.Name, "error", err)

				// Add error message to history
				messages = append(messages, Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Error executing tool: %v", err),
					ToolCallID: functionCall.ID,
					Name:       functionCall.Name,
				})
				continue
			}

			// Add tool result to message history
			resultContent := ar.formatToolResult(toolResult)
			messages = append(messages, Message{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: functionCall.ID,
				Name:       functionCall.Name,
			})

			log.Debug("Tool executed successfully", "tool", functionCall.Name, "success", toolResult.Success)
		}
	}

	// Max iterations reached
	log.Warn("Agent orchestration reached max iterations", "max_iterations", ar.maxIterations)

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
	// This would be populated during Run() - for now return empty
	return []Message{}
}
