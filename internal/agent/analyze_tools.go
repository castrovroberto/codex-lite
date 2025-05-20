package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// AdvancedAnalyzeTool implements comprehensive codebase analysis
type AdvancedAnalyzeTool struct {
	workspaceRoot string
}

func NewAdvancedAnalyzeTool(workspaceRoot string) *AdvancedAnalyzeTool {
	return &AdvancedAnalyzeTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *AdvancedAnalyzeTool) Name() string {
	return "analyze_advanced"
}

func (t *AdvancedAnalyzeTool) Description() string {
	return "Perform advanced analysis including dependencies, complexity, and security"
}

func (t *AdvancedAnalyzeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"include_deps": {
				"type": "boolean",
				"description": "Include dependency analysis"
			},
			"include_complexity": {
				"type": "boolean",
				"description": "Include code complexity analysis"
			},
			"include_security": {
				"type": "boolean",
				"description": "Include security analysis"
			}
		}
	}`)
}

type AdvancedAnalyzeParams struct {
	IncludeDeps       bool `json:"include_deps,omitempty"`
	IncludeComplexity bool `json:"include_complexity,omitempty"`
	IncludeSecurity   bool `json:"include_security,omitempty"`
}

func (t *AdvancedAnalyzeTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p AdvancedAnalyzeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	var analysis strings.Builder
	var details = make(map[string]interface{})

	// Run dependency analysis
	if p.IncludeDeps {
		deps, err := analyzer.AnalyzeDependencies(t.workspaceRoot)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("dependency analysis failed: %v", err),
			}, nil
		}
		analysis.WriteString(analyzer.FormatDependencyAnalysis(deps))
		analysis.WriteString("\n")
		details["dependencies"] = deps
	}

	// Run complexity analysis
	if p.IncludeComplexity {
		complexity, err := analyzer.AnalyzeComplexity(t.workspaceRoot)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("complexity analysis failed: %v", err),
			}, nil
		}
		analysis.WriteString(analyzer.FormatComplexityAnalysis(complexity))
		analysis.WriteString("\n")
		details["complexity"] = complexity
	}

	// Run security analysis
	if p.IncludeSecurity {
		issues, err := analyzer.AnalyzeSecurity(t.workspaceRoot)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("security analysis failed: %v", err),
			}, nil
		}
		analysis.WriteString(analyzer.FormatSecurityAnalysis(issues))
		analysis.WriteString("\n")
		details["security"] = issues
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"analysis_text": analysis.String(),
			"details":       details,
		},
	}, nil
}
