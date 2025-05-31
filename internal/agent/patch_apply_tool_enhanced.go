package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/castrovroberto/CGE/internal/patchutils"
)

// EnhancedPatchApplyTool implements enhanced patch/diff application capabilities
type EnhancedPatchApplyTool struct {
	workspaceRoot string
	applier       *patchutils.PatchApplier
}

// NewEnhancedPatchApplyTool creates a new enhanced patch apply tool
func NewEnhancedPatchApplyTool(workspaceRoot string) *EnhancedPatchApplyTool {
	options := patchutils.DefaultApplyOptions()
	applier := patchutils.NewPatchApplier(workspaceRoot, options)

	return &EnhancedPatchApplyTool{
		workspaceRoot: workspaceRoot,
		applier:       applier,
	}
}

func (t *EnhancedPatchApplyTool) Name() string {
	return "apply_patch_to_file_enhanced"
}

func (t *EnhancedPatchApplyTool) Description() string {
	return "Applies a diff/patch (in unified diff format) to a specified file using robust go-diff library. Creates backup before applying and supports rollback."
}

func (t *EnhancedPatchApplyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The path to the file to patch (relative to workspace root)"
			},
			"patch_content": {
				"type": "string",
				"description": "The patch content in unified diff format"
			},
			"backup_original": {
				"type": "boolean",
				"description": "Whether to create a backup of the original file",
				"default": true
			},
			"dry_run": {
				"type": "boolean",
				"description": "Preview changes without applying them",
				"default": false
			},
			"ignore_whitespace": {
				"type": "boolean",
				"description": "Ignore whitespace differences when applying patch",
				"default": false
			}
		},
		"required": ["file_path", "patch_content"]
	}`)
}

type EnhancedPatchApplyParams struct {
	FilePath         string `json:"file_path"`
	PatchContent     string `json:"patch_content"`
	BackupOriginal   bool   `json:"backup_original"`
	DryRun           bool   `json:"dry_run"`
	IgnoreWhitespace bool   `json:"ignore_whitespace"`
}

func (t *EnhancedPatchApplyTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p EnhancedPatchApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Validate required parameters
	if p.FilePath == "" {
		return &ToolResult{
			Success: false,
			Error:   "file_path cannot be empty",
		}, nil
	}

	if p.PatchContent == "" {
		return &ToolResult{
			Success: false,
			Error:   "patch_content cannot be empty",
		}, nil
	}

	// Configure applier options
	options := patchutils.ApplyOptions{
		CreateBackup:     p.BackupOriginal,
		BackupSuffix:     ".bak",
		DryRun:           p.DryRun,
		IgnoreWhitespace: p.IgnoreWhitespace,
	}

	// Create a new applier with the specific options
	applier := patchutils.NewPatchApplier(t.workspaceRoot, options)

	// Apply the patch
	result, err := applier.ApplyPatch(p.FilePath, p.PatchContent)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   result.Error,
		}, nil
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"file_path":     result.FilePath,
		"hunks_applied": result.HunksApplied,
		"lines_changed": result.LinesChanged,
		"original_size": result.OriginalSize,
		"new_size":      result.NewSize,
		"dry_run":       p.DryRun,
	}

	if result.BackupPath != "" {
		responseData["backup_path"] = result.BackupPath
	}

	if p.DryRun {
		responseData["message"] = fmt.Sprintf("Dry run: Would apply patch to %s (%d hunks, %d lines changed)",
			result.FilePath, result.HunksApplied, result.LinesChanged)
	} else {
		responseData["message"] = fmt.Sprintf("Successfully applied patch to %s (%d hunks, %d lines changed)",
			result.FilePath, result.HunksApplied, result.LinesChanged)
	}

	return &ToolResult{
		Success: true,
		Data:    responseData,
	}, nil
}
