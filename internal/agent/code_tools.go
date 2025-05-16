package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CodeSearchTool implements semantic code search
type CodeSearchTool struct {
	workspaceRoot string
}

func NewCodeSearchTool(workspaceRoot string) *CodeSearchTool {
	return &CodeSearchTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *CodeSearchTool) Name() string {
	return "codebase_search"
}

func (t *CodeSearchTool) Description() string {
	return "Find snippets of code from the codebase most relevant to the search query"
}

func (t *CodeSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query to find relevant code"
			},
			"target_directories": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Glob patterns for directories to search over"
			}
		},
		"required": ["query"]
	}`)
}

type CodeSearchParams struct {
	Query             string   `json:"query"`
	TargetDirectories []string `json:"target_directories,omitempty"`
}

func (t *CodeSearchTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p CodeSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// TODO: Implement actual semantic search
	// For now, just return a mock result
	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"matches": []map[string]interface{}{
				{
					"file":    "example.go",
					"snippet": "func main() { ... }",
					"score":   0.95,
				},
			},
		},
	}, nil
}

// FileReadTool implements file reading capability
type FileReadTool struct {
	workspaceRoot string
}

func NewFileReadTool(workspaceRoot string) *FileReadTool {
	return &FileReadTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *FileReadTool) Name() string {
	return "read_file"
}

func (t *FileReadTool) Description() string {
	return "Read the contents of a file"
}

func (t *FileReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_file": {
				"type": "string",
				"description": "The path of the file to read"
			},
			"start_line": {
				"type": "integer",
				"description": "The line number to start reading from (1-based)"
			},
			"end_line": {
				"type": "integer",
				"description": "The line number to end reading at (1-based, inclusive)"
			}
		},
		"required": ["target_file"]
	}`)
}

type FileReadParams struct {
	TargetFile string `json:"target_file"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
}

func (t *FileReadTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p FileReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Resolve file path
	filePath := p.TargetFile
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workspaceRoot, filePath)
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"content": string(content),
		},
	}, nil
}
