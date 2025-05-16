package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/analyzer"
)

// CodebaseAnalyzeTool implements codebase analysis capability
type CodebaseAnalyzeTool struct {
	workspaceRoot string
}

func NewCodebaseAnalyzeTool(workspaceRoot string) *CodebaseAnalyzeTool {
	return &CodebaseAnalyzeTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *CodebaseAnalyzeTool) Name() string {
	return "analyze_codebase"
}

func (t *CodebaseAnalyzeTool) Description() string {
	return "Analyze the codebase structure, including Git status, file types, and line counts"
}

func (t *CodebaseAnalyzeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"custom_extensions": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Additional file extensions to analyze (e.g. ['.vue', '.svelte'])"
			}
		}
	}`)
}

type CodebaseAnalyzeParams struct {
	CustomExtensions []string `json:"custom_extensions,omitempty"`
}

func (t *CodebaseAnalyzeTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p CodebaseAnalyzeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Run analysis
	info, err := analyzer.AnalyzeCodebase(t.workspaceRoot, p.CustomExtensions)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to analyze codebase: %v", err),
		}, nil
	}

	// Format results
	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"analysis_text": info.FormatAnalysis(),
			"details": map[string]interface{}{
				"is_git_repo":   info.IsGitRepo,
				"file_count":    info.FileCount,
				"total_lines":   info.TotalLines,
				"files_by_type": info.FilesByType,
				"lines_by_type": info.LinesByType,
				"skipped_dirs":  info.SkippedDirs,
				"errors":        info.Errors,
			},
		},
	}, nil
}
