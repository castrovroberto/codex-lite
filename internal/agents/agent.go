package agents

import (
	"context"
	"fmt"
)

// Result holds the output of an agent's analysis.
type Result struct {
	AgentName string
	File      string
	Output    string
	Error     error // Optional: If an agent can produce partial results alongside an error
}

// Agent defines the interface for different analysis agents.
type Agent interface {
	Name() string
	Description() string
	// Analyze performs the agent's specific analysis logic.
	// It takes the model name, file path, and file content as input.
	Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error)
}

// AgentError wraps errors specific to an agent's execution.
// It allows attaching the agent's name and a specific message, and an underlying error.
type AgentError struct {
	AgentName string
	Message   string
	Err       error // Underlying error
}

// Error implements the error interface for AgentError.
func (e *AgentError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("agent '%s': %s: %v", e.AgentName, e.Message, e.Err)
	}
	return fmt.Sprintf("agent '%s': %s", e.AgentName, e.Message)
}

// Unwrap returns the underlying error, supporting errors.Is and errors.As.
func (e *AgentError) Unwrap() error {
	return e.Err
}