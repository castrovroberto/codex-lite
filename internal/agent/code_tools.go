package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/castrovroberto/codex-lite/internal/analyzer"
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

	// Get all files to search in
	var matches []map[string]interface{}
	err := filepath.WalkDir(t.workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-regular files
		if d.IsDir() {
			// Skip common directories
			if analyzer.IsSkippableDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary and very large files
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > 1024*1024 { // Skip files larger than 1MB
			return nil
		}

		// Skip files with non-text extensions
		ext := strings.ToLower(filepath.Ext(path))
		if !analyzer.IsSourceFile(ext) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Simple relevance scoring based on:
		// 1. Exact matches (case insensitive)
		// 2. Word matches
		// 3. Substring matches
		score := calculateRelevance(string(content), p.Query)
		if score > 0 {
			// Get the context around matches
			context := extractContext(string(content), p.Query)

			// Add to matches if relevant
			relPath, err := filepath.Rel(t.workspaceRoot, path)
			if err != nil {
				relPath = path
			}
			matches = append(matches, map[string]interface{}{
				"file":    relPath,
				"score":   score,
				"context": context,
			})
		}

		return nil
	})

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to search codebase: %v", err),
		}, nil
	}

	// Sort matches by score
	sort.Slice(matches, func(i, j int) bool {
		return matches[i]["score"].(float64) > matches[j]["score"].(float64)
	})

	// Limit to top 10 matches
	if len(matches) > 10 {
		matches = matches[:10]
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"matches": matches,
		},
	}, nil
}

// calculateRelevance scores the relevance of content to a query
func calculateRelevance(content, query string) float64 {
	content = strings.ToLower(content)
	query = strings.ToLower(query)

	var score float64

	// Exact match (case insensitive)
	if strings.Contains(content, query) {
		score += 1.0
	}

	// Word matches
	queryWords := strings.Fields(query)
	contentWords := strings.Fields(content)
	wordMatches := 0
	for _, qw := range queryWords {
		for _, cw := range contentWords {
			if qw == cw {
				wordMatches++
			}
		}
	}
	score += float64(wordMatches) / float64(len(queryWords)) * 0.5

	// Substring matches
	substringMatches := 0
	for _, qw := range queryWords {
		if strings.Contains(content, qw) {
			substringMatches++
		}
	}
	score += float64(substringMatches) / float64(len(queryWords)) * 0.3

	return score
}

// extractContext gets the surrounding lines around matches
func extractContext(content, query string) []string {
	lines := strings.Split(content, "\n")
	query = strings.ToLower(query)
	var contexts []string

	// Find matching lines and include context
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			// Get context (2 lines before and after)
			start := max(0, i-2)
			end := min(len(lines), i+3)

			context := strings.Join(lines[start:end], "\n")
			contexts = append(contexts, context)
		}
	}

	// Limit number of contexts
	if len(contexts) > 3 {
		contexts = contexts[:3]
	}

	return contexts
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
