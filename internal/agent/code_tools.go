package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/castrovroberto/CGE/internal/analyzer"
	"github.com/castrovroberto/CGE/internal/security"
)

// CodeSearchTool implements semantic code search
type CodeSearchTool struct {
	workspaceRoot string
	safeOps       *security.SafeFileOps
}

func NewCodeSearchTool(workspaceRoot string) *CodeSearchTool {
	// Create safe file operations with workspace root as allowed root
	safeOps := security.NewSafeFileOps(workspaceRoot)

	return &CodeSearchTool{
		workspaceRoot: workspaceRoot,
		safeOps:       safeOps,
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

	if p.Query == "" {
		return &ToolResult{
			Success: false,
			Error:   "query parameter is required",
		}, nil
	}

	var matches []map[string]interface{}

	// Walk through the codebase
	err := filepath.Walk(t.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories and non-source files
		if info.IsDir() {
			if analyzer.IsSkippableDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !analyzer.IsSourceFile(ext) {
			return nil
		}

		// Read file content using secure file operations
		content, err := t.safeOps.SafeReadFile(path)
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
	score += float64(wordMatches) / float64(len(queryWords)) * 1.0

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
	safeOps       *security.SafeFileOps
}

func NewFileReadTool(workspaceRoot string) *FileReadTool {
	// Create safe file operations with workspace root as allowed root
	safeOps := security.NewSafeFileOps(workspaceRoot)

	return &FileReadTool{
		workspaceRoot: workspaceRoot,
		safeOps:       safeOps,
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
			"start_line_one_indexed": {
				"type": "integer",
				"description": "The line number to start reading from (1-based)"
			},
			"end_line_one_indexed_inclusive": {
				"type": "integer",
				"description": "The line number to end reading at (1-based, inclusive)"
			}
		},
		"required": ["target_file"]
	}`)
}

type FileReadParams struct {
	TargetFile string `json:"target_file"`
	StartLine  int    `json:"start_line_one_indexed,omitempty"`
	EndLine    int    `json:"end_line_one_indexed_inclusive,omitempty"`
}

func (t *FileReadTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p FileReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate that the target_file field exists (check the raw JSON)
	var rawParams map[string]interface{}
	if err := json.Unmarshal(params, &rawParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	if _, exists := rawParams["target_file"]; !exists {
		return nil, fmt.Errorf("missing required parameter: target_file")
	}

	// Resolve file path
	filePath := p.TargetFile
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workspaceRoot, filePath)
	}

	// Read file using secure file operations
	content, err := t.safeOps.SafeReadFile(filePath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	// Handle line range if specified
	contentStr := string(content)
	if p.StartLine > 0 || p.EndLine > 0 {
		lines := strings.Split(contentStr, "\n")

		// Validate line range
		if p.StartLine > 0 && p.EndLine > 0 && p.StartLine > p.EndLine {
			return &ToolResult{
				Success: false,
				Error:   "invalid range: start_line must be less than or equal to end_line",
			}, nil
		}

		// Convert to 0-based indexing and apply range
		startIdx := 0
		if p.StartLine > 0 {
			startIdx = p.StartLine - 1
		}

		endIdx := len(lines)
		if p.EndLine > 0 {
			endIdx = p.EndLine
		}

		// Validate bounds
		if startIdx >= len(lines) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("start_line %d exceeds file length %d", p.StartLine, len(lines)),
			}, nil
		}

		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		// Extract the specified range
		selectedLines := lines[startIdx:endIdx]
		contentStr = strings.Join(selectedLines, "\n")
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"content": contentStr,
		},
	}, nil
}
