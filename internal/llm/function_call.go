package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FunctionCall represents a function call request from the LLM
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	ID        string          `json:"id,omitempty"`
}

// FunctionCallResponse represents the parsed response from an LLM
type FunctionCallResponse struct {
	IsTextResponse bool
	TextContent    string
	FunctionCall   *FunctionCall
}

// ToolDefinition represents a tool definition for the LLM
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition represents the function part of a tool definition
type ToolFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ParseFunctionCall attempts to parse a function call from LLM response
func ParseFunctionCall(response string) (*FunctionCallResponse, error) {
	response = strings.TrimSpace(response)

	// First, try to extract JSON from the response if it's mixed with text
	jsonPattern := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonPattern.FindAllString(response, -1)

	for _, match := range matches {
		// Try to parse as JSON function call
		var functionCall FunctionCall
		if err := json.Unmarshal([]byte(match), &functionCall); err == nil {
			// Validate that it has required fields for a function call
			if functionCall.Name != "" {
				// Validate arguments if present
				if functionCall.Arguments != nil {
					var args map[string]interface{}
					if json.Unmarshal(functionCall.Arguments, &args) != nil {
						continue // Skip if arguments are malformed
					}
				}
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   &functionCall,
				}, nil
			}
		}

		// Try alternative format: {"function_call": {...}}
		var wrapper struct {
			FunctionCall *FunctionCall `json:"function_call"`
			ToolCall     *FunctionCall `json:"tool_call"`
		}
		if err := json.Unmarshal([]byte(match), &wrapper); err == nil {
			if wrapper.FunctionCall != nil && wrapper.FunctionCall.Name != "" {
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   wrapper.FunctionCall,
				}, nil
			}
			if wrapper.ToolCall != nil && wrapper.ToolCall.Name != "" {
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   wrapper.ToolCall,
				}, nil
			}
		}
	}

	// Try direct parse if response looks like pure JSON
	if strings.HasPrefix(response, "{") && strings.HasSuffix(response, "}") {
		var functionCall FunctionCall
		if err := json.Unmarshal([]byte(response), &functionCall); err == nil {
			// Validate that it has required fields for a function call
			if functionCall.Name != "" {
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   &functionCall,
				}, nil
			}
		}

		// Try alternative format: {"function_call": {...}}
		var wrapper struct {
			FunctionCall *FunctionCall `json:"function_call"`
			ToolCall     *FunctionCall `json:"tool_call"`
		}
		if err := json.Unmarshal([]byte(response), &wrapper); err == nil {
			if wrapper.FunctionCall != nil && wrapper.FunctionCall.Name != "" {
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   wrapper.FunctionCall,
				}, nil
			}
			if wrapper.ToolCall != nil && wrapper.ToolCall.Name != "" {
				return &FunctionCallResponse{
					IsTextResponse: false,
					FunctionCall:   wrapper.ToolCall,
				}, nil
			}
		}
	}

	// If not a function call, treat as text response
	return &FunctionCallResponse{
		IsTextResponse: true,
		TextContent:    response,
	}, nil
}

// FormatToolDefinitions converts tool definitions to the format expected by LLM providers
func FormatToolDefinitions(tools []ToolDefinition) []map[string]interface{} {
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"type":     tool.Type,
			"function": tool.Function,
		}
	}
	return result
}

// CreateToolDefinition creates a ToolDefinition from basic components
func CreateToolDefinition(name, description string, parameters json.RawMessage) ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: ToolFunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// FormatToolCallForPrompt formats tool definitions for inclusion in prompts (for providers without native function calling)
func FormatToolCallForPrompt(tools []ToolDefinition) string {
	if len(tools) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n\nAvailable tools:\n")

	for _, tool := range tools {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
		builder.WriteString(fmt.Sprintf("  Parameters: %s\n", string(tool.Function.Parameters)))
	}

	builder.WriteString("\nTo use a tool, respond with JSON in this format:\n")
	builder.WriteString(`{"name": "tool_name", "arguments": {"param1": "value1", "param2": "value2"}}`)
	builder.WriteString("\n\nIf you don't need to use a tool, respond normally with text.\n")

	return builder.String()
}
