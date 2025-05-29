package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolResult represents the result of executing a tool
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`

	// Enhanced error information (optional)
	StandardizedError *StandardizedToolError `json:"standardized_error,omitempty"`
}

// SetError sets both the legacy error string and standardized error
func (tr *ToolResult) SetError(err error) {
	if standardizedErr, ok := err.(*StandardizedToolError); ok {
		tr.StandardizedError = standardizedErr
		tr.Error = standardizedErr.Error()
	} else {
		tr.Error = err.Error()
	}
	tr.Success = false
}

// SetErrorString sets just the legacy error string
func (tr *ToolResult) SetErrorString(errorMsg string) {
	tr.Error = errorMsg
	tr.Success = false
}

// GetFormattedError returns a formatted error message for the LLM
func (tr *ToolResult) GetFormattedError() string {
	if tr.StandardizedError != nil {
		return tr.StandardizedError.FormatForLLM()
	}
	return tr.Error
}

// NewSuccessResult creates a successful ToolResult
func NewSuccessResult(data interface{}) *ToolResult {
	return &ToolResult{
		Success: true,
		Data:    data,
	}
}

// NewErrorResult creates a failed ToolResult with a standardized error
func NewErrorResult(err *StandardizedToolError) *ToolResult {
	return &ToolResult{
		Success:           false,
		Error:             err.Error(),
		StandardizedError: err,
	}
}

// NewSimpleErrorResult creates a failed ToolResult with a simple error message
func NewSimpleErrorResult(errorMsg string) *ToolResult {
	return &ToolResult{
		Success: false,
		Error:   errorMsg,
	}
}

// Tool represents a capability that can be invoked by the agent
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Parameters returns the JSON schema for the tool's parameters
	Parameters() json.RawMessage

	// Execute runs the tool with the given parameters and returns the result
	Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error)
}

// Registry maintains the set of available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) error {
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = tool
	return nil
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolNames returns the names of all registered tools
func (r *Registry) GetToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered tools
func (r *Registry) Count() int {
	return len(r.tools)
}
