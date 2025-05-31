package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestTool runs tests with progress reporting
type TestTool struct {
	workspaceRoot string
}

// NewTestTool creates a new test tool
func NewTestTool(workspaceRoot string) *TestTool {
	return &TestTool{
		workspaceRoot: workspaceRoot,
	}
}

// Name returns the tool name
func (t *TestTool) Name() string {
	return "run_tests"
}

// Description returns the tool description
func (t *TestTool) Description() string {
	return "Runs tests in the specified directory or package with progress reporting"
}

// Parameters returns the JSON schema for the tool parameters
func (t *TestTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"package": map[string]interface{}{
				"type":        "string",
				"description": "Package or directory to test (default: ./...)",
				"default":     "./...",
			},
			"verbose": map[string]interface{}{
				"type":        "boolean",
				"description": "Run tests in verbose mode",
				"default":     false,
			},
			"timeout": map[string]interface{}{
				"type":        "string",
				"description": "Test timeout (e.g., '30s', '5m')",
				"default":     "5m",
			},
		},
	}

	schemaBytes, _ := json.Marshal(schema)
	return json.RawMessage(schemaBytes)
}

// Execute runs the tests without progress reporting (fallback)
func (t *TestTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	return t.ExecuteWithProgress(ctx, params, nil)
}

// ExecuteWithProgress runs tests with progress reporting
func (t *TestTool) ExecuteWithProgress(ctx context.Context, params json.RawMessage, progressCallback ProgressCallback) (*ToolResult, error) {
	var testParams struct {
		Package string `json:"package"`
		Verbose bool   `json:"verbose"`
		Timeout string `json:"timeout"`
	}

	// Set defaults
	testParams.Package = "./..."
	testParams.Verbose = false
	testParams.Timeout = "5m"

	if err := json.Unmarshal(params, &testParams); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid parameters: %v", err),
		}, nil
	}

	// Report initial progress
	if progressCallback != nil {
		progressCallback(0.0, "Preparing test environment...", 0, 4)
	}

	// Change to workspace directory
	originalDir, err := os.Getwd()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to get current directory: %v", err),
		}, nil
	}

	if err := os.Chdir(t.workspaceRoot); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to change to workspace directory: %v", err),
		}, nil
	}
	defer os.Chdir(originalDir)

	if progressCallback != nil {
		progressCallback(0.25, "Building test packages...", 1, 4)
	}

	// Build the test command
	args := []string{"test"}

	if testParams.Verbose {
		args = append(args, "-v")
	}

	args = append(args, "-timeout", testParams.Timeout)
	args = append(args, testParams.Package)

	if progressCallback != nil {
		progressCallback(0.5, "Running tests...", 2, 4)
	}

	// Execute the test command
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = t.workspaceRoot

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if progressCallback != nil {
		progressCallback(0.75, "Processing test results...", 3, 4)
	}

	// Parse test results for more detailed progress
	testResults := t.parseTestOutput(outputStr)

	if progressCallback != nil {
		progressCallback(1.0, "Tests completed", 4, 4)
	}

	// Determine success based on exit code and output
	success := err == nil
	if !success && err != nil {
		// Check if it's just test failures vs build errors
		if exitError, ok := err.(*exec.ExitError); ok {
			// Exit code 1 usually means test failures, which we still want to report
			success = exitError.ExitCode() == 1
		}
	}

	result := &ToolResult{
		Success: success,
		Data: map[string]interface{}{
			"command": fmt.Sprintf("go %s", strings.Join(args, " ")),
			"output":  outputStr,
			"results": testResults,
			"exit_code": func() int {
				if err != nil {
					if e, ok := err.(*exec.ExitError); ok {
						return e.ExitCode()
					}
				}
				return 0
			}(),
			"duration": time.Since(time.Now()).String(), // This should be tracked properly
		},
	}

	if err != nil && !success {
		result.Error = fmt.Sprintf("Test execution failed: %v", err)
	}

	return result, nil
}

// parseTestOutput parses Go test output to extract test results
func (t *TestTool) parseTestOutput(output string) map[string]interface{} {
	lines := strings.Split(output, "\n")
	results := map[string]interface{}{
		"total_tests":   0,
		"passed_tests":  0,
		"failed_tests":  0,
		"skipped_tests": 0,
		"packages":      []string{},
		"failures":      []string{},
	}

	totalTests := 0
	passedTests := 0
	failedTests := 0
	skippedTests := 0
	packages := make(map[string]bool)
	var failures []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Package detection
		if strings.HasPrefix(line, "=== RUN") {
			totalTests++
		} else if strings.HasPrefix(line, "--- PASS:") {
			passedTests++
		} else if strings.HasPrefix(line, "--- FAIL:") {
			failedTests++
			failures = append(failures, line)
		} else if strings.HasPrefix(line, "--- SKIP:") {
			skippedTests++
		} else if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") {
			// Package result line
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pkg := parts[1]
				packages[pkg] = true
			}
		}
	}

	// Convert packages map to slice
	var packageList []string
	for pkg := range packages {
		packageList = append(packageList, pkg)
	}

	results["total_tests"] = totalTests
	results["passed_tests"] = passedTests
	results["failed_tests"] = failedTests
	results["skipped_tests"] = skippedTests
	results["packages"] = packageList
	results["failures"] = failures

	return results
}
