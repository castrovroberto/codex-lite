package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// FileWriteToolEnhanced implements file writing capabilities with dependency injection
type FileWriteToolEnhanced struct {
	workspaceRoot string
	validator     *ToolValidator
	fileSystem    FileSystemService
}

// NewFileWriteToolEnhanced creates a new enhanced file write tool with dependency injection
func NewFileWriteToolEnhanced(workspaceRoot string, fs FileSystemService) *FileWriteToolEnhanced {
	return &FileWriteToolEnhanced{
		workspaceRoot: workspaceRoot,
		validator:     NewToolValidator(workspaceRoot),
		fileSystem:    fs,
	}
}

func (t *FileWriteToolEnhanced) Name() string {
	return "write_file"
}

func (t *FileWriteToolEnhanced) Description() string {
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

func (t *FileWriteToolEnhanced) Parameters() json.RawMessage {
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

func (t *FileWriteToolEnhanced) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
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

	// Check if parent directory exists or needs creation using injected file system
	parentDir := filepath.Dir(fullPath)
	if !t.fileSystem.Exists(parentDir) {
		if !createDirs {
			return NewErrorResult(NewDirectoryNotFoundError(filepath.Dir(p.FilePath)).
				WithDetail("suggestion", "Set create_dirs_if_needed=true or create parent directories first")), nil
		}

		// Create parent directories using injected file system
		if err := t.fileSystem.MkdirAll(parentDir, 0750); err != nil {
			return NewErrorResult(NewStandardizedError(
				ErrorCodePermissionDenied,
				fmt.Sprintf("Failed to create parent directories for %s", p.FilePath),
				"Check write permissions for the workspace directory",
			).WithDetail("file_path", p.FilePath).WithDetail("parent_dir", parentDir).WithDetail("os_error", err.Error())), nil
		}
	} else if !t.fileSystem.IsDir(parentDir) {
		return NewErrorResult(NewStandardizedError(
			ErrorCodeInvalidPathFormat,
			fmt.Sprintf("Parent path is not a directory: %s", filepath.Dir(p.FilePath)),
			"Ensure the parent path points to a directory, not a file",
		).WithDetail("file_path", p.FilePath).WithDetail("parent_path", filepath.Dir(p.FilePath))), nil
	}

	// Check if file already exists (for informational purposes) using injected file system
	var fileExisted bool
	var originalSize int64
	if t.fileSystem.Exists(fullPath) {
		if stat, err := t.fileSystem.Stat(fullPath); err == nil {
			fileExisted = true
			originalSize = stat.Size()
		}
	}

	// Write the file using injected file system
	if err := t.fileSystem.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
		return NewErrorResult(NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Failed to write file: %s", p.FilePath),
			"Check write permissions for the file and its parent directory",
		).WithDetail("file_path", p.FilePath).WithDetail("os_error", err.Error())), nil
	}

	// Get file info for response using injected file system
	info, err := t.fileSystem.Stat(fullPath)
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
