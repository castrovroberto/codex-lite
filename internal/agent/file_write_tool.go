package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileWriteTool implements file writing capabilities
type FileWriteTool struct {
	workspaceRoot string
}

// NewFileWriteTool creates a new file write tool
func NewFileWriteTool(workspaceRoot string) *FileWriteTool {
	return &FileWriteTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *FileWriteTool) Name() string {
	return "write_file"
}

func (t *FileWriteTool) Description() string {
	return "Writes or overwrites content to a specified file. Creates the file and parent directories if they don't exist."
}

func (t *FileWriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The path to the file to write (relative to workspace root)"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			},
			"create_dirs_if_needed": {
				"type": "boolean",
				"description": "Whether to create parent directories if they don't exist",
				"default": true
			}
		},
		"required": ["file_path", "content"]
	}`)
}

type FileWriteParams struct {
	FilePath           string `json:"file_path"`
	Content            string `json:"content"`
	CreateDirsIfNeeded bool   `json:"create_dirs_if_needed"`
}

func (t *FileWriteTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p FileWriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Set default value for CreateDirsIfNeeded
	if p.CreateDirsIfNeeded == false {
		// Check if the field was explicitly set to false in the JSON
		var rawParams map[string]interface{}
		if err := json.Unmarshal(params, &rawParams); err == nil {
			if _, exists := rawParams["create_dirs_if_needed"]; !exists {
				// Field not specified, use default value of true
				p.CreateDirsIfNeeded = true
			}
		}
	}

	// Validate file path
	if p.FilePath == "" {
		return &ToolResult{
			Success: false,
			Error:   "file_path cannot be empty",
		}, nil
	}

	// Security check: ensure path is within workspace
	var fullPath string
	if filepath.IsAbs(p.FilePath) {
		// Absolute paths are not allowed
		return &ToolResult{
			Success: false,
			Error:   "absolute file paths are not allowed",
		}, nil
	} else {
		fullPath = filepath.Join(t.workspaceRoot, p.FilePath)
	}

	cleanPath := filepath.Clean(fullPath)
	cleanWorkspace := filepath.Clean(t.workspaceRoot)
	if !strings.HasPrefix(cleanPath, cleanWorkspace) {
		return &ToolResult{
			Success: false,
			Error:   "file path is outside workspace root",
		}, nil
	}

	// Create parent directories if needed
	if p.CreateDirsIfNeeded {
		dir := filepath.Dir(cleanPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create directories: %v", err),
			}, nil
		}
	}

	// Write the file
	if err := os.WriteFile(cleanPath, []byte(p.Content), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	// Get file info for response
	info, err := os.Stat(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get file info after write: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"file_path":     p.FilePath,
			"bytes_written": len(p.Content),
			"file_size":     info.Size(),
			"message":       fmt.Sprintf("Successfully wrote %d bytes to %s", len(p.Content), p.FilePath),
		},
	}, nil
}
