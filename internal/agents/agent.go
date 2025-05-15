// internal/agents/agent.go
package agents

import "context"

type Result struct {
	File   string
	Output string
	Agent  string
}

type Agent interface {
	Name() string
	// Updated Analyze signature
	Analyze(ctx context.Context, modelName string, path string, code string) (Result, error)
}