package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ProgressCallback is a function type for reporting progress
type ProgressCallback func(progress float64, status string, step, totalSteps int)

// ProgressAwareTool extends the Tool interface with progress reporting capabilities
type ProgressAwareTool interface {
	Tool
	ExecuteWithProgress(ctx context.Context, params json.RawMessage, progressCallback ProgressCallback) (*ToolResult, error)
}

// ProgressToolWrapper wraps a regular tool to add progress reporting capabilities
type ProgressToolWrapper struct {
	tool    Tool
	program *tea.Program // TUI program for sending progress messages
	callID  string       // Unique call ID for this execution
}

// NewProgressToolWrapper creates a new progress-aware tool wrapper
func NewProgressToolWrapper(tool Tool, program *tea.Program, callID string) *ProgressToolWrapper {
	return &ProgressToolWrapper{
		tool:    tool,
		program: program,
		callID:  callID,
	}
}

// Name returns the wrapped tool's name
func (ptw *ProgressToolWrapper) Name() string {
	return ptw.tool.Name()
}

// Description returns the wrapped tool's description
func (ptw *ProgressToolWrapper) Description() string {
	return ptw.tool.Description()
}

// Parameters returns the wrapped tool's parameters
func (ptw *ProgressToolWrapper) Parameters() json.RawMessage {
	return ptw.tool.Parameters()
}

// Execute wraps the tool execution with progress reporting
func (ptw *ProgressToolWrapper) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	// Send start message
	if ptw.program != nil {
		var toolParams map[string]interface{}
		json.Unmarshal(params, &toolParams)

		ptw.program.Send(toolStartMsg{
			toolCallID: ptw.callID,
			toolName:   ptw.tool.Name(),
			params:     toolParams,
		})
	}

	// Create progress callback
	progressCallback := func(progress float64, status string, step, totalSteps int) {
		if ptw.program != nil {
			ptw.program.Send(toolProgressMsg{
				toolCallID: ptw.callID,
				toolName:   ptw.tool.Name(),
				progress:   progress,
				status:     status,
				step:       step,
				totalSteps: totalSteps,
			})
		}
	}

	// Execute the tool
	var result *ToolResult
	var err error

	// Check if tool supports progress reporting
	if progressTool, ok := ptw.tool.(ProgressAwareTool); ok {
		result, err = progressTool.ExecuteWithProgress(ctx, params, progressCallback)
	} else {
		// For tools that don't support progress, simulate basic progress
		progressCallback(0.0, "Starting...", 0, 1)
		result, err = ptw.tool.Execute(ctx, params)
		if err == nil {
			progressCallback(1.0, "Completed", 1, 1)
		}
	}

	// Send completion message
	if ptw.program != nil {
		var resultText string
		if result != nil && result.Data != nil {
			if resultJSON, jsonErr := json.MarshalIndent(result.Data, "", "  "); jsonErr == nil {
				resultText = string(resultJSON)
			} else {
				resultText = fmt.Sprintf("%v", result.Data)
			}
		}

		ptw.program.Send(toolCompleteMsg{
			toolCallID: ptw.callID,
			toolName:   ptw.tool.Name(),
			success:    err == nil && (result == nil || result.Success),
			result:     resultText,
			duration:   time.Since(time.Now()), // This should be tracked properly
			error: func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		})
	}

	return result, err
}

// Message types for progress communication (these should match the ones in chat/model.go)
type toolProgressMsg struct {
	toolCallID string
	toolName   string
	progress   float64
	status     string
	step       int
	totalSteps int
}

type toolStartMsg struct {
	toolCallID string
	toolName   string
	params     map[string]interface{}
}

type toolCompleteMsg struct {
	toolCallID string
	toolName   string
	success    bool
	result     string
	duration   time.Duration
	error      string
}
