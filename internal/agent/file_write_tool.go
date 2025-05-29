package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool implements file writing capabilities with enhanced validation and error handling
type FileWriteTool struct {
	workspaceRoot string
	validator     *ToolValidator
}

// NewFileWriteTool creates a new file write tool
func NewFileWriteTool(workspaceRoot string) *FileWriteTool {
	return &FileWriteTool{
		workspaceRoot: workspaceRoot,
		validator:     NewToolValidator(workspaceRoot),
	}
}

func (t *FileWriteTool) Name() string {
	return "write_file"
}

func (t *FileWriteTool) Description() string {
	return `Writes content to a specified file, creating the file and parent directories if needed.

USAGE EXAMPLES:
- write_file({"file_path": "src/main.go", "content": "package main\n..."})
- write_file({"file_path": "docs/README.md", "content": "# Project", "create_dirs_if_needed": false})

IMPORTANT NOTES:
- File path must be relative to workspace root
- Will overwrite existing files by default
- Creates parent directories unless create_dirs_if_needed=false
- Use with caution on existing files - always check file existence first if unsure

PRE-CONDITIONS:
- Workspace must be writable
- Parent directory must exist (unless create_dirs_if_needed=true)
- File path must be within workspace boundary`
}

func (t *FileWriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The path to the file to write (relative to workspace root)",
				"pattern": "^[^\\0<>:\"|?*]+$",
				"minLength": 1,
				"maxLength": 260
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file",
				"maxLength": 1048576
			},
			"create_dirs_if_needed": {
				"type": "boolean",
				"description": "Whether to create parent directories if they don't exist",
				"default": true
			}
		},
		"required": ["file_path", "content"],
		"additionalProperties": false
	}`)
}

type FileWriteParams struct {
	FilePath           string `json:"file_path"`
	Content            string `json:"content"`
	CreateDirsIfNeeded *bool  `json:"create_dirs_if_needed,omitempty"`
}

func (t *FileWriteTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	// Enhanced parameter validation
	if err := t.validator.ValidateJSONSchema(params, t.Parameters()); err != nil {
		return NewErrorResult(err.(*StandardizedToolError)), nil
	}

	var p FileWriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return NewErrorResult(NewStandardizedError(
			ErrorCodeInvalidParameters,
			"Failed to parse parameters",
			"Ensure all parameters are properly formatted JSON",
		).WithDetail("parse_error", err.Error())), nil
	}

	// Set default value for CreateDirsIfNeeded
	createDirs := true
	if p.CreateDirsIfNeeded != nil {
		createDirs = *p.CreateDirsIfNeeded
	}

	// Enhanced path validation
	if err := t.validator.ValidateFilePath(p.FilePath); err != nil {
		return NewErrorResult(err.(*StandardizedToolError)), nil
	}

	// Content size validation (1MB limit)
	if err := t.validator.ValidateContentSize(p.Content, 1048576); err != nil {
		return NewErrorResult(err.(*StandardizedToolError)), nil
	}

	// Get safe path
	fullPath, err := t.validator.GetSafePath(p.FilePath)
	if err != nil {
		return NewErrorResult(err.(*StandardizedToolError)), nil
	}

	// Check if parent directory exists or needs creation
	parentDir := filepath.Dir(fullPath)
	if stat, err := os.Stat(parentDir); os.IsNotExist(err) {
		if !createDirs {
			return NewErrorResult(NewDirectoryNotFoundError(filepath.Dir(p.FilePath)).
				WithDetail("suggestion", "Set create_dirs_if_needed=true or create parent directories first")), nil
		}

		// Create parent directories
		if err := os.MkdirAll(parentDir, 0750); err != nil {
			return NewErrorResult(NewStandardizedError(
				ErrorCodePermissionDenied,
				fmt.Sprintf("Failed to create parent directories for %s", p.FilePath),
				"Check write permissions for the workspace directory",
			).WithDetail("file_path", p.FilePath).WithDetail("parent_dir", parentDir).WithDetail("os_error", err.Error())), nil
		}
	} else if err != nil {
		return NewErrorResult(NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Cannot access parent directory for %s", p.FilePath),
			"Check directory permissions and ensure the path is accessible",
		).WithDetail("file_path", p.FilePath).WithDetail("parent_dir", parentDir)), nil
	} else if !stat.IsDir() {
		return NewErrorResult(NewStandardizedError(
			ErrorCodeInvalidPathFormat,
			fmt.Sprintf("Parent path is not a directory: %s", filepath.Dir(p.FilePath)),
			"Ensure the parent path points to a directory, not a file",
		).WithDetail("file_path", p.FilePath).WithDetail("parent_path", filepath.Dir(p.FilePath))), nil
	}

	// Check if file already exists (for informational purposes)
	var fileExisted bool
	var originalSize int64
	if stat, err := os.Stat(fullPath); err == nil {
		fileExisted = true
		originalSize = stat.Size()
	}

	// Write the file
	if err := os.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
		return NewErrorResult(NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Failed to write file: %s", p.FilePath),
			"Check write permissions for the file and its parent directory",
		).WithDetail("file_path", p.FilePath).WithDetail("os_error", err.Error())), nil
	}

	// Get file info for response
	info, err := os.Stat(fullPath)
	if err != nil {
		return NewErrorResult(NewStandardizedError(
			ErrorCodeInternalError,
			fmt.Sprintf("File written successfully but cannot read file info: %s", p.FilePath),
			"This is unusual - file was written but cannot be accessed immediately",
		).WithDetail("file_path", p.FilePath).WithDetail("os_error", err.Error())), nil
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"file_path":     p.FilePath,
		"bytes_written": len(p.Content),
		"file_size":     info.Size(),
		"created_dirs":  createDirs && !fileExisted,
		"overwritten":   fileExisted,
	}

	if fileExisted {
		responseData["original_size"] = originalSize
		responseData["message"] = fmt.Sprintf("Successfully overwrote %s (%d bytes written, was %d bytes)",
			p.FilePath, len(p.Content), originalSize)
	} else {
		responseData["message"] = fmt.Sprintf("Successfully created %s (%d bytes written)",
			p.FilePath, len(p.Content))
	}

	return NewSuccessResult(responseData), nil
}
