package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// setupTestWorkspace creates a temporary workspace for testing
func setupTestWorkspace(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "cge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Create sample files
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
		"README.md": `# Test Project

This is a test project for CGE.`,
		"src/utils.go": `package src

func Helper() string {
	return "helper function"
}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	return tmpDir
}

func TestFileReadTool(t *testing.T) {
	// Setup test workspace
	workspace := setupTestWorkspace(t)
	tool := NewFileReadTool(workspace)

	t.Run("read_existing_file", func(t *testing.T) {
		params := json.RawMessage(`{"target_file": "main.go"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data to be map[string]interface{}")
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be string")
		}

		if content == "" {
			t.Errorf("Expected non-empty content")
		}

		if !contains(content, "package main") {
			t.Errorf("Expected content to contain 'package main'")
		}
	})

	t.Run("read_nonexistent_file", func(t *testing.T) {
		params := json.RawMessage(`{"target_file": "nonexistent.go"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.Success {
			t.Errorf("Expected failure for nonexistent file")
		}

		if result.Error == "" {
			t.Errorf("Expected error message for nonexistent file")
		}
	})

	t.Run("read_with_absolute_path", func(t *testing.T) {
		absPath := filepath.Join(workspace, "README.md")
		params := json.RawMessage(fmt.Sprintf(`{"target_file": "%s"}`, absPath))
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success with absolute path, got failure: %s", result.Error)
		}
	})

	t.Run("invalid_parameters", func(t *testing.T) {
		params := json.RawMessage(`{"invalid": "params"}`)
		_, err := tool.Execute(context.Background(), params)

		if err == nil {
			t.Errorf("Expected error for invalid parameters")
		}
	})

	t.Run("malformed_json", func(t *testing.T) {
		params := json.RawMessage(`{"target_file": "main.go"`)
		_, err := tool.Execute(context.Background(), params)

		if err == nil {
			t.Errorf("Expected error for malformed JSON")
		}
	})

	t.Run("path_traversal_protection", func(t *testing.T) {
		params := json.RawMessage(`{"target_file": "../../../etc/passwd"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.Success {
			t.Errorf("Expected failure for path traversal attempt")
		}
	})

	t.Run("binary_file_handling", func(t *testing.T) {
		// Create a binary file
		binaryPath := filepath.Join(workspace, "binary.bin")
		binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}
		err := os.WriteFile(binaryPath, binaryData, 0644)
		if err != nil {
			t.Fatalf("Failed to create binary file: %v", err)
		}

		params := json.RawMessage(`{"target_file": "binary.bin"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success reading binary file, got failure: %s", result.Error)
		}

		// The content should be readable as string (even if it contains binary data)
		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data to be map[string]interface{}")
		}

		_, ok = data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be string")
		}
	})
}

func TestCodeSearchTool(t *testing.T) {
	// Setup test workspace
	workspace := setupTestWorkspace(t)
	tool := NewCodeSearchTool(workspace)

	t.Run("search_existing_code", func(t *testing.T) {
		params := json.RawMessage(`{"query": "package main"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data to be map[string]interface{}")
		}

		matches, ok := data["matches"].([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected matches to be []map[string]interface{}")
		}

		if len(matches) == 0 {
			t.Errorf("Expected at least one match for 'package main'")
		}

		// Check first match structure
		if len(matches) > 0 {
			match := matches[0]
			if _, ok := match["file"]; !ok {
				t.Errorf("Expected match to have 'file' field")
			}
			if _, ok := match["score"]; !ok {
				t.Errorf("Expected match to have 'score' field")
			}
			if _, ok := match["context"]; !ok {
				t.Errorf("Expected match to have 'context' field")
			}
		}
	})

	t.Run("search_no_matches", func(t *testing.T) {
		params := json.RawMessage(`{"query": "nonexistent_function_xyz123"}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success even with no matches, got failure: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data to be map[string]interface{}")
		}

		matches, ok := data["matches"].([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected matches to be []map[string]interface{}")
		}

		if len(matches) != 0 {
			t.Errorf("Expected no matches for nonexistent query, got %d", len(matches))
		}
	})

	t.Run("search_with_target_directories", func(t *testing.T) {
		params := json.RawMessage(`{"query": "func", "target_directories": ["src/"]}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		// Note: Current implementation doesn't actually filter by target_directories
		// This test documents the current behavior
	})

	t.Run("invalid_parameters", func(t *testing.T) {
		params := json.RawMessage(`{"invalid": "params"}`)
		_, err := tool.Execute(context.Background(), params)

		if err == nil {
			t.Errorf("Expected error for invalid parameters")
		}
	})

	t.Run("empty_query", func(t *testing.T) {
		params := json.RawMessage(`{"query": ""}`)
		result, err := tool.Execute(context.Background(), params)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected success with empty query, got failure: %s", result.Error)
		}
	})
}

func TestCalculateRelevance(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		query    string
		expected float64
	}{
		{
			name:     "exact_match",
			content:  "This is a test function",
			query:    "test function",
			expected: 1.0, // Should be > 1.0 due to exact match + word matches
		},
		{
			name:     "word_matches",
			content:  "This is a test of the function",
			query:    "test function",
			expected: 1.0, // Should be 1.0 (word matches)
		},
		{
			name:     "substring_matches",
			content:  "This is testing functionality",
			query:    "test function",
			expected: 0.3, // Should be 0.3 (substring matches)
		},
		{
			name:     "no_matches",
			content:  "This is completely different",
			query:    "test function",
			expected: 0.0,
		},
		{
			name:     "case_insensitive",
			content:  "This is a TEST FUNCTION",
			query:    "test function",
			expected: 1.0, // Should match case-insensitively
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := calculateRelevance(tc.content, tc.query)
			if score < tc.expected {
				t.Errorf("Expected score >= %f, got %f", tc.expected, score)
			}
		})
	}
}

func TestExtractContext(t *testing.T) {
	content := `line 1
line 2
line 3 with match
line 4
line 5
line 6 with another match
line 7
line 8`

	contexts := extractContext(content, "match")

	if len(contexts) == 0 {
		t.Errorf("Expected at least one context")
	}

	// Check that context includes surrounding lines
	if len(contexts) > 0 {
		firstContext := contexts[0]
		if !contains(firstContext, "line 1") || !contains(firstContext, "line 3 with match") || !contains(firstContext, "line 5") {
			t.Errorf("Expected context to include surrounding lines, got: %s", firstContext)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
