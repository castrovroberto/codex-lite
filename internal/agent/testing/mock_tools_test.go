package testing

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMockToolFramework(t *testing.T) {
	t.Run("mock_tool_basic_functionality", func(t *testing.T) {
		// Test the mock tool framework itself
		mockTool := NewMockReadFileTool("test content", true)

		// Test interface methods
		if mockTool.Name() != "read_file" {
			t.Errorf("Expected name 'read_file', got '%s'", mockTool.Name())
		}

		if mockTool.Description() == "" {
			t.Errorf("Expected non-empty description")
		}

		// Test execution
		params := CreateTestParameters(map[string]interface{}{
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

	t.Run("mock_tool_failure", func(t *testing.T) {
		// Test mock tool configured to fail
		mockTool := NewMockReadFileTool("", false)

		params := CreateTestParameters(map[string]interface{}{
			"target_file": "test.txt",
		})

		result, err := mockTool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Mock tool execution failed: %v", err)
		}

		if result.Success {
			t.Errorf("Expected mock tool to fail")
		}

		if result.Error == "" {
			t.Errorf("Expected error message when tool fails")
		}
	})

	t.Run("mock_tool_registry", func(t *testing.T) {
		registry := NewMockToolRegistry()

		// Register some mock tools
		readTool := NewMockReadFileTool("content", true)
		writeTool := NewMockWriteFileTool(true)

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
		params := CreateTestParameters(map[string]interface{}{
			"target_file": "test.txt",
		})

		tool.Execute(context.Background(), params)
		tool.Execute(context.Background(), params)

		counts := registry.GetCallCounts()
		if counts["read_file"] != 2 {
			t.Errorf("Expected 2 calls to read_file, got %d", counts["read_file"])
		}

		// Test reset
		registry.ResetAllCallCounts()
		counts = registry.GetCallCounts()
		if counts["read_file"] != 0 {
			t.Errorf("Expected 0 calls after reset, got %d", counts["read_file"])
		}
	})

	t.Run("mock_tool_with_execution_time", func(t *testing.T) {
		config := MockToolConfig{
			Name:        "slow_tool",
			Description: "A slow tool for testing",
			Parameters:  json.RawMessage(`{"type": "object"}`),
			Behavior: MockBehavior{
				ShouldSucceed: true,
				ExecutionTime: 100 * time.Millisecond,
				ReturnData:    "slow result",
			},
		}

		mockTool := NewMockTool(config)

		start := time.Now()
		result, err := mockTool.Execute(context.Background(), json.RawMessage(`{}`))
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected tool to succeed")
		}

		if duration < 100*time.Millisecond {
			t.Errorf("Expected execution to take at least 100ms, took %v", duration)
		}
	})

	t.Run("mock_tool_parameter_validation", func(t *testing.T) {
		config := MockToolConfig{
			Name:        "strict_tool",
			Description: "A tool with strict parameter validation",
			Parameters:  json.RawMessage(`{"type": "object"}`),
			Behavior: MockBehavior{
				ShouldSucceed:  true,
				ValidateParams: true,
				ExpectedParams: map[string]interface{}{"key": "value"},
				ReturnData:     "validated result",
			},
		}

		mockTool := NewMockTool(config)

		// Test with correct parameters
		correctParams := json.RawMessage(`{"key": "value"}`)
		result, err := mockTool.Execute(context.Background(), correctParams)
		if err != nil {
			t.Fatalf("Tool execution failed with correct params: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected tool to succeed with correct params")
		}

		// Test with incorrect parameters
		incorrectParams := json.RawMessage(`{"wrong": "params"}`)
		result, err = mockTool.Execute(context.Background(), incorrectParams)
		if err == nil {
			t.Errorf("Expected tool to fail with incorrect params")
		}
	})
}

func TestTestHelpers(t *testing.T) {
	t.Run("setup_test_workspace", func(t *testing.T) {
		workspace := SetupTestWorkspace(t)

		// Check that workspace directory exists
		if workspace.Dir == "" {
			t.Errorf("Expected non-empty workspace directory")
		}

		// Check that sample files were created
		if !workspace.FileExists("main.go") {
			t.Errorf("Expected main.go to exist in workspace")
		}

		if !workspace.FileExists("README.md") {
			t.Errorf("Expected README.md to exist in workspace")
		}

		if !workspace.FileExists("src/utils.go") {
			t.Errorf("Expected src/utils.go to exist in workspace")
		}

		// Test file content
		content := workspace.ReadFile("main.go")
		if content == "" {
			t.Errorf("Expected main.go to have content")
		}

		if !contains(content, "package main") {
			t.Errorf("Expected main.go to contain 'package main'")
		}
	})

	t.Run("create_sample_project", func(t *testing.T) {
		workspace := CreateSampleProject(t, "go")

		// Check Go-specific files
		if !workspace.FileExists("go.mod") {
			t.Errorf("Expected go.mod to exist")
		}

		if !workspace.FileExists("handler.go") {
			t.Errorf("Expected handler.go to exist")
		}

		// Check content
		goMod := workspace.ReadFile("go.mod")
		if !contains(goMod, "module testproject") {
			t.Errorf("Expected go.mod to contain module declaration")
		}
	})

	t.Run("test_fixtures", func(t *testing.T) {
		fixtures := NewTestFixtures()

		if fixtures.SampleGoCode == "" {
			t.Errorf("Expected non-empty Go code sample")
		}

		if fixtures.SampleTestOutput == "" {
			t.Errorf("Expected non-empty test output sample")
		}

		if !contains(fixtures.SampleTestOutput, "FAIL") {
			t.Errorf("Expected test output to contain failures")
		}
	})
}

func TestTestCases(t *testing.T) {
	t.Run("read_file_test_cases", func(t *testing.T) {
		testCases := GetReadFileTestCases()

		if len(testCases) == 0 {
			t.Errorf("Expected non-empty test cases")
		}

		// Check that we have both success and failure cases
		hasSuccess := false
		hasFailure := false

		for _, tc := range testCases {
			if tc.ShouldFail {
				hasFailure = true
			} else {
				hasSuccess = true
			}

			// Validate test case structure
			if tc.Name == "" {
				t.Errorf("Test case should have a name")
			}

			if tc.Parameters == nil {
				t.Errorf("Test case should have parameters")
			}
		}

		if !hasSuccess {
			t.Errorf("Expected at least one success test case")
		}

		if !hasFailure {
			t.Errorf("Expected at least one failure test case")
		}
	})

	t.Run("write_file_test_cases", func(t *testing.T) {
		testCases := GetWriteFileTestCases()

		if len(testCases) == 0 {
			t.Errorf("Expected non-empty test cases")
		}

		// Check for required parameters
		for _, tc := range testCases {
			if _, ok := tc.Parameters["file_path"]; !ok {
				t.Errorf("Write file test case should have file_path parameter")
			}

			if _, ok := tc.Parameters["content"]; !ok {
				t.Errorf("Write file test case should have content parameter")
			}
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
