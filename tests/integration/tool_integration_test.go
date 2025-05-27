//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	agenttesting "github.com/castrovroberto/CGE/internal/agent/testing"
)

func TestReadFileTool_Integration(t *testing.T) {
	// Setup test workspace
	workspace := agenttesting.SetupTestWorkspace(t)

	// Create the read file tool
	tool := agent.NewFileReadTool(workspace.Dir)

	// Test cases from our fixtures
	testCases := agenttesting.GetReadFileTestCases()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Create test parameters
			params := agenttesting.CreateTestParameters(tc.Parameters)

			// Execute the tool
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := tool.Execute(ctx, params)

			// Validate results based on test case expectations
			if tc.ShouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected tool to fail, but it succeeded")
				}
				if tc.ErrorContains != "" && (result == nil || !contains(result.Error, tc.ErrorContains)) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.ErrorContains, result.Error)
				}
			} else {
				if err != nil {
					t.Fatalf("Tool execution failed unexpectedly: %v", err)
				}
				if !result.Success {
					t.Errorf("Expected tool to succeed, but it failed: %s", result.Error)
				}

				// Validate specific data expectations
				if tc.ExpectedResult != nil && tc.ExpectedResult.Data != nil {
					if result.Data == nil {
						t.Errorf("Expected data but got nil")
					} else {
						// Check if content contains expected strings
						if expectedContent, ok := tc.ExpectedResult.Data["content"].(string); ok {
							if actualData, ok := result.Data.(map[string]interface{}); ok {
								if actualContent, ok := actualData["content"].(string); ok {
									if !contains(actualContent, expectedContent) {
										t.Errorf("Expected content to contain '%s', got '%s'", expectedContent, actualContent)
									}
								} else {
									t.Errorf("Expected content field in result data")
								}
							} else {
								t.Errorf("Expected result data to be a map")
							}
						}
					}
				}
			}
		})
	}
}

func TestReadFileTool_SpecificScenarios(t *testing.T) {
	workspace := agenttesting.SetupTestWorkspace(t)
	tool := agent.NewFileReadTool(workspace.Dir)

	t.Run("read_entire_file", func(t *testing.T) {
		// Test reading the entire main.go file
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file": "main.go",
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if !result.Success {
			t.Fatalf("Tool execution failed: %s", result.Error)
		}

		// Validate the content contains expected Go code
		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be a string")
		}

		expectedStrings := []string{"package main", "import", "func main"}
		for _, expected := range expectedStrings {
			if !contains(content, expected) {
				t.Errorf("Expected content to contain '%s'", expected)
			}
		}
	})

	t.Run("read_file_with_line_range", func(t *testing.T) {
		// Create a test file with known content
		testContent := `line 1
line 2
line 3
line 4
line 5`
		workspace.CreateFile("test_lines.txt", testContent)

		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file":                    "test_lines.txt",
			"start_line_one_indexed":         2,
			"end_line_one_indexed_inclusive": 4,
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Failed to read file with range: %v", err)
		}

		if !result.Success {
			t.Fatalf("Tool execution failed: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be a string")
		}

		// Should contain lines 2-4
		expectedLines := []string{"line 2", "line 3", "line 4"}
		for _, expected := range expectedLines {
			if !contains(content, expected) {
				t.Errorf("Expected content to contain '%s'", expected)
			}
		}

		// Should not contain line 1 or line 5
		unexpectedLines := []string{"line 1", "line 5"}
		for _, unexpected := range unexpectedLines {
			if contains(content, unexpected) {
				t.Errorf("Expected content to NOT contain '%s'", unexpected)
			}
		}
	})

	t.Run("read_large_file", func(t *testing.T) {
		// Create a large file (1000 lines)
		largeContent := ""
		for i := 1; i <= 1000; i++ {
			largeContent += fmt.Sprintf("This is line %d with some content to make it longer\n", i)
		}
		workspace.CreateFile("large.txt", largeContent)

		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file": "large.txt",
		})

		start := time.Now()
		result, err := tool.Execute(context.Background(), params)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Failed to read large file: %v", err)
		}

		if !result.Success {
			t.Fatalf("Tool execution failed: %s", result.Error)
		}

		// Should complete within reasonable time (less than 1 second)
		if duration > time.Second {
			t.Errorf("Reading large file took too long: %v", duration)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be a string")
		}

		// Verify content contains expected lines
		if !contains(content, "This is line 1") {
			t.Errorf("Expected content to contain first line")
		}
		if !contains(content, "This is line 1000") {
			t.Errorf("Expected content to contain last line")
		}
	})
}

func TestReadFileTool_ErrorHandling(t *testing.T) {
	workspace := agenttesting.SetupTestWorkspace(t)
	tool := agent.NewFileReadTool(workspace.Dir)

	t.Run("invalid_json_parameters", func(t *testing.T) {
		invalidParams := json.RawMessage(`{"target_file": 123}`) // number instead of string

		result, err := tool.Execute(context.Background(), invalidParams)

		// Should handle invalid parameters gracefully
		if err == nil && result.Success {
			t.Errorf("Expected tool to fail with invalid parameters")
		}
	})

	t.Run("missing_required_parameter", func(t *testing.T) {
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"wrong_field": "value",
		})

		result, err := tool.Execute(context.Background(), params)

		if err == nil && result.Success {
			t.Errorf("Expected tool to fail with missing required parameter")
		}
	})

	t.Run("path_traversal_attempt", func(t *testing.T) {
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file": "../../../etc/passwd",
		})

		result, err := tool.Execute(context.Background(), params)

		// Should reject path traversal attempts
		if err == nil && result.Success {
			t.Errorf("Expected tool to reject path traversal attempt")
		}
	})
}

func TestWriteFileTool_Integration(t *testing.T) {
	workspace := agenttesting.SetupTestWorkspace(t)
	tool := agent.NewFileWriteTool(workspace.Dir)

	testCases := agenttesting.GetWriteFileTestCases()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			params := agenttesting.CreateTestParameters(tc.Parameters)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := tool.Execute(ctx, params)

			if tc.ShouldFail {
				if err == nil && result.Success {
					t.Errorf("Expected tool to fail, but it succeeded")
				}
			} else {
				if err != nil {
					t.Fatalf("Tool execution failed unexpectedly: %v", err)
				}
				if !result.Success {
					t.Errorf("Expected tool to succeed, but it failed: %s", result.Error)
				}

				// Verify file was created/modified
				if filePath, ok := tc.Parameters["file_path"].(string); ok {
					if !workspace.FileExists(filePath) {
						t.Errorf("Expected file %s to exist after write", filePath)
					}

					// Verify content if provided
					if expectedContent, ok := tc.Parameters["content"].(string); ok {
						actualContent := workspace.ReadFile(filePath)
						if actualContent != expectedContent {
							t.Errorf("File content mismatch.\nExpected: %s\nActual: %s", expectedContent, actualContent)
						}
					}
				}
			}
		})
	}
}

func TestShellRunTool_Integration(t *testing.T) {
	workspace := agenttesting.SetupTestWorkspace(t)
	tool := agent.NewShellRunTool(workspace.Dir)

	t.Run("simple_echo_command", func(t *testing.T) {
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"command": "echo 'Hello, World!'",
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Tool execution failed: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		stdout, ok := data["stdout"].(string)
		if !ok {
			t.Fatalf("Expected stdout to be a string")
		}

		if !contains(stdout, "Hello, World!") {
			t.Errorf("Expected stdout to contain 'Hello, World!', got: %s", stdout)
		}

		exitCode, ok := data["exit_code"].(int)
		if !ok {
			t.Fatalf("Expected exit_code to be an int")
		}

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got: %d", exitCode)
		}
	})

	t.Run("command_with_error", func(t *testing.T) {
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"command": "ls /nonexistent/directory",
		})

		result, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		// Tool should succeed even if command fails
		if !result.Success {
			t.Fatalf("Tool execution failed: %s", result.Error)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		exitCode, ok := data["exit_code"].(int)
		if !ok {
			t.Fatalf("Expected exit_code to be an int")
		}

		if exitCode == 0 {
			t.Errorf("Expected non-zero exit code for failed command")
		}
	})
}

func TestMockToolFramework(t *testing.T) {
	t.Run("mock_tool_basic_functionality", func(t *testing.T) {
		// Test the mock tool framework itself
		mockTool := agenttesting.NewMockReadFileTool("test content", true)

		// Test interface methods
		if mockTool.Name() != "read_file" {
			t.Errorf("Expected name 'read_file', got '%s'", mockTool.Name())
		}

		if mockTool.Description() == "" {
			t.Errorf("Expected non-empty description")
		}

		// Test execution
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file": "test.txt",
		})

		result, err := mockTool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Mock tool execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected mock tool to succeed")
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result data to be a map")
		}

		content, ok := data["content"].(string)
		if !ok {
			t.Fatalf("Expected content to be a string")
		}

		if content != "test content" {
			t.Errorf("Expected content 'test content', got '%s'", content)
		}
	})

	t.Run("mock_tool_registry", func(t *testing.T) {
		registry := agenttesting.NewMockToolRegistry()

		// Register some mock tools
		readTool := agenttesting.NewMockReadFileTool("content", true)
		writeTool := agenttesting.NewMockWriteFileTool(true)

		err := registry.Register(readTool)
		if err != nil {
			t.Fatalf("Failed to register read tool: %v", err)
		}

		err = registry.Register(writeTool)
		if err != nil {
			t.Fatalf("Failed to register write tool: %v", err)
		}

		// Test retrieval
		tool, exists := registry.Get("read_file")
		if !exists {
			t.Errorf("Expected to find read_file tool")
		}
		if tool.Name() != "read_file" {
			t.Errorf("Expected tool name 'read_file', got '%s'", tool.Name())
		}

		// Test listing
		tools := registry.List()
		if len(tools) != 2 {
			t.Errorf("Expected 2 tools, got %d", len(tools))
		}

		// Test call counting
		params := agenttesting.CreateTestParameters(map[string]interface{}{
			"target_file": "test.txt",
		})

		tool.Execute(context.Background(), params)
		tool.Execute(context.Background(), params)

		counts := registry.GetCallCounts()
		if counts["read_file"] != 2 {
			t.Errorf("Expected 2 calls to read_file, got %d", counts["read_file"])
		}
	})
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
