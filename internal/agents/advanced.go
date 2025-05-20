package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/agent"
)

// AdvancedAgent wraps the advanced analysis functionality
type AdvancedAgent struct {
	tool *agent.AdvancedAnalyzeTool
}

func NewAdvancedAgent(workspaceRoot string) *AdvancedAgent {
	return &AdvancedAgent{
		tool: agent.NewAdvancedAnalyzeTool(workspaceRoot),
	}
}

func (a *AdvancedAgent) Name() string {
	return "Advanced Analysis"
}

func (a *AdvancedAgent) Description() string {
	return "Performs comprehensive analysis including dependencies, complexity, and security"
}

func (a *AdvancedAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error) {
	// Run all analysis types by default
	params := json.RawMessage(`{
		"include_deps": true,
		"include_complexity": true,
		"include_security": true
	}`)

	result, err := a.tool.Execute(ctx, params)
	if err != nil {
		return Result{
			AgentName: a.Name(),
			File:      filePath,
			Error:     err,
		}, err
	}

	if !result.Success {
		return Result{
			AgentName: a.Name(),
			File:      filePath,
			Error:     fmt.Errorf(result.Error),
		}, fmt.Errorf(result.Error)
	}

	// Extract analysis text from result data
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		return Result{
			AgentName: a.Name(),
			File:      filePath,
			Error:     fmt.Errorf("unexpected result data format"),
		}, fmt.Errorf("unexpected result data format")
	}

	analysisText, ok := data["analysis_text"].(string)
	if !ok {
		return Result{
			AgentName: a.Name(),
			File:      filePath,
			Error:     fmt.Errorf("analysis text not found in result"),
		}, fmt.Errorf("analysis text not found in result")
	}

	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    analysisText,
	}, nil
}
