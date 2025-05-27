package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TestRunnerTool implements test execution with structured output
type TestRunnerTool struct {
	workspaceRoot string
}

func NewTestRunnerTool(workspaceRoot string) *TestRunnerTool {
	return &TestRunnerTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *TestRunnerTool) Name() string {
	return "run_tests"
}

func (t *TestRunnerTool) Description() string {
	return "Runs tests in the specified directory or package with structured output parsing"
}

func (t *TestRunnerTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_path": {
				"type": "string",
				"description": "Directory or package path to run tests in (default: current directory)"
			},
			"test_pattern": {
				"type": "string",
				"description": "Pattern to match specific test names"
			},
			"verbose": {
				"type": "boolean",
				"description": "Enable verbose test output"
			},
			"timeout_seconds": {
				"type": "integer",
				"description": "Test execution timeout in seconds (default: 300)"
			},
			"coverage": {
				"type": "boolean",
				"description": "Enable coverage reporting"
			}
		}
	}`)
}

type TestRunnerParams struct {
	TargetPath     string `json:"target_path,omitempty"`
	TestPattern    string `json:"test_pattern,omitempty"`
	Verbose        bool   `json:"verbose,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Coverage       bool   `json:"coverage,omitempty"`
}

type TestResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "PASS", "FAIL", "SKIP"
	Duration string `json:"duration,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

type TestSummary struct {
	TotalTests   int          `json:"total_tests"`
	PassedTests  int          `json:"passed_tests"`
	FailedTests  int          `json:"failed_tests"`
	SkippedTests int          `json:"skipped_tests"`
	Duration     string       `json:"duration"`
	Coverage     string       `json:"coverage,omitempty"`
	Results      []TestResult `json:"results"`
	RawOutput    string       `json:"raw_output"`
}

func (t *TestRunnerTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p TestRunnerParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if p.TargetPath == "" {
		p.TargetPath = "."
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = 300
	}

	// Resolve target path (for potential future use in validation)
	_ = filepath.Join(t.workspaceRoot, p.TargetPath)

	// Build test command
	args := []string{"test"}

	if p.Verbose {
		args = append(args, "-v")
	}

	if p.Coverage {
		args = append(args, "-cover")
	}

	if p.TestPattern != "" {
		args = append(args, "-run", p.TestPattern)
	}

	// Add target path
	args = append(args, p.TargetPath)

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutSeconds)*time.Second)
	defer cancel()

	// Execute test command
	cmd := exec.CommandContext(testCtx, "go", args...)
	cmd.Dir = t.workspaceRoot

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Parse test results
	summary := t.parseTestOutput(outputStr)

	// Determine overall success
	success := err == nil && summary.FailedTests == 0

	if err != nil {
		// Check if it's a timeout
		if testCtx.Err() == context.DeadlineExceeded {
			summary.RawOutput += "\n\nTest execution timed out"
		}
	}

	return &ToolResult{
		Success: success,
		Data:    summary,
		Error: func() string {
			if !success && summary.FailedTests > 0 {
				return fmt.Sprintf("%d test(s) failed", summary.FailedTests)
			}
			if err != nil {
				return fmt.Sprintf("test execution failed: %v", err)
			}
			return ""
		}(),
	}, nil
}

func (t *TestRunnerTool) parseTestOutput(output string) TestSummary {
	summary := TestSummary{
		RawOutput: output,
		Results:   []TestResult{},
	}

	lines := strings.Split(output, "\n")

	// Regex patterns for Go test output
	testResultPattern := regexp.MustCompile(`^=== RUN\s+(\S+)`)
	testPassPattern := regexp.MustCompile(`^\s*--- PASS:\s+(\S+)\s+\(([^)]+)\)`)
	testFailPattern := regexp.MustCompile(`^\s*--- FAIL:\s+(\S+)\s+\(([^)]+)\)`)
	testSkipPattern := regexp.MustCompile(`^\s*--- SKIP:\s+(\S+)\s+\(([^)]+)\)`)
	summaryPattern := regexp.MustCompile(`^(PASS|FAIL)\s+.*\s+([0-9.]+s)`)
	coveragePattern := regexp.MustCompile(`coverage:\s+([0-9.]+%)\s+of\s+statements`)

	var currentTest string
	var currentOutput strings.Builder

	for _, line := range lines {
		// Check for test start
		if match := testResultPattern.FindStringSubmatch(line); match != nil {
			currentTest = match[1]
			currentOutput.Reset()
			continue
		}

		// Check for test results
		if match := testPassPattern.FindStringSubmatch(line); match != nil {
			summary.Results = append(summary.Results, TestResult{
				Name:     match[1],
				Status:   "PASS",
				Duration: match[2],
				Output:   currentOutput.String(),
			})
			summary.PassedTests++
			summary.TotalTests++
			continue
		}

		if match := testFailPattern.FindStringSubmatch(line); match != nil {
			summary.Results = append(summary.Results, TestResult{
				Name:     match[1],
				Status:   "FAIL",
				Duration: match[2],
				Output:   currentOutput.String(),
				Error:    line,
			})
			summary.FailedTests++
			summary.TotalTests++
			continue
		}

		if match := testSkipPattern.FindStringSubmatch(line); match != nil {
			summary.Results = append(summary.Results, TestResult{
				Name:     match[1],
				Status:   "SKIP",
				Duration: match[2],
				Output:   currentOutput.String(),
			})
			summary.SkippedTests++
			summary.TotalTests++
			continue
		}

		// Check for overall summary
		if match := summaryPattern.FindStringSubmatch(line); match != nil {
			summary.Duration = match[2]
			continue
		}

		// Check for coverage
		if match := coveragePattern.FindStringSubmatch(line); match != nil {
			summary.Coverage = match[1]
			continue
		}

		// Accumulate output for current test
		if currentTest != "" {
			currentOutput.WriteString(line + "\n")
		}
	}

	return summary
}
