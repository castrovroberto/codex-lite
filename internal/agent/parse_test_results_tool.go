package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ParseTestResultsTool implements test output parsing
type ParseTestResultsTool struct {
	workspaceRoot string
}

func NewParseTestResultsTool(workspaceRoot string) *ParseTestResultsTool {
	return &ParseTestResultsTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *ParseTestResultsTool) Name() string {
	return "parse_test_results"
}

func (t *ParseTestResultsTool) Description() string {
	return "Parses raw test output to extract structured information about failed tests, error messages, and file locations"
}

func (t *ParseTestResultsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"test_output": {
				"type": "string",
				"description": "Raw test output to parse"
			},
			"test_framework": {
				"type": "string",
				"enum": ["go", "jest", "pytest", "auto"],
				"description": "Test framework type (default: auto-detect)"
			}
		},
		"required": ["test_output"]
	}`)
}

type ParseTestResultsParams struct {
	TestOutput    string `json:"test_output"`
	TestFramework string `json:"test_framework,omitempty"`
}

type ParsedTestFailure struct {
	TestName     string `json:"test_name"`
	TestFile     string `json:"test_file,omitempty"`
	Line         int    `json:"line,omitempty"`
	ErrorMessage string `json:"error_message"`
	StackTrace   string `json:"stack_trace,omitempty"`
	FailureType  string `json:"failure_type"` // "assertion", "panic", "timeout", "compilation"
	Context      string `json:"context,omitempty"`
}

type ParsedTestSummary struct {
	TotalTests    int                 `json:"total_tests"`
	PassedTests   int                 `json:"passed_tests"`
	FailedTests   int                 `json:"failed_tests"`
	SkippedTests  int                 `json:"skipped_tests"`
	Duration      string              `json:"duration,omitempty"`
	Coverage      string              `json:"coverage,omitempty"`
	Failures      []ParsedTestFailure `json:"failures"`
	CompileErrors []string            `json:"compile_errors,omitempty"`
	Summary       string              `json:"summary"`
}

func (t *ParseTestResultsTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p ParseTestResultsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if p.TestOutput == "" {
		return &ToolResult{
			Success: false,
			Error:   "test_output parameter is required",
		}, nil
	}

	// Auto-detect framework if not specified
	if p.TestFramework == "" || p.TestFramework == "auto" {
		p.TestFramework = t.detectTestFramework(p.TestOutput)
	}

	var summary ParsedTestSummary

	switch p.TestFramework {
	case "go":
		summary = t.parseGoTestOutput(p.TestOutput)
	case "jest":
		summary = t.parseJestOutput(p.TestOutput)
	case "pytest":
		summary = t.parsePytestOutput(p.TestOutput)
	default:
		// Fallback to Go parsing as it's most common in this codebase
		summary = t.parseGoTestOutput(p.TestOutput)
	}

	return &ToolResult{
		Success: true,
		Data:    summary,
	}, nil
}

func (t *ParseTestResultsTool) detectTestFramework(output string) string {
	output = strings.ToLower(output)

	if strings.Contains(output, "=== run") || strings.Contains(output, "--- fail") || strings.Contains(output, "--- pass") {
		return "go"
	}
	if strings.Contains(output, "jest") || strings.Contains(output, "test suites") {
		return "jest"
	}
	if strings.Contains(output, "pytest") || strings.Contains(output, "collected") {
		return "pytest"
	}

	return "go" // Default fallback
}

func (t *ParseTestResultsTool) parseGoTestOutput(output string) ParsedTestSummary {
	summary := ParsedTestSummary{
		Failures: []ParsedTestFailure{},
	}

	lines := strings.Split(output, "\n")

	// Regex patterns for Go test output
	testRunPattern := regexp.MustCompile(`^=== RUN\s+(\S+)`)
	testPassPattern := regexp.MustCompile(`^\s*--- PASS:\s+(\S+)\s+\(([^)]+)\)`)
	testFailPattern := regexp.MustCompile(`^\s*--- FAIL:\s+(\S+)\s+\(([^)]+)\)`)
	testSkipPattern := regexp.MustCompile(`^\s*--- SKIP:\s+(\S+)\s+\(([^)]+)\)`)
	summaryPattern := regexp.MustCompile(`^(PASS|FAIL)\s+.*\s+([0-9.]+s)`)
	coveragePattern := regexp.MustCompile(`coverage:\s+([0-9.]+%)\s+of\s+statements`)
	panicPattern := regexp.MustCompile(`panic:(.*)`)
	fileLinePattern := regexp.MustCompile(`\s+([^:]+):(\d+):`)

	var currentFailure *ParsedTestFailure
	var collectingFailure bool

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Check for test start
		if match := testRunPattern.FindStringSubmatch(line); match != nil {
			collectingFailure = false
			continue
		}

		// Check for test results
		if match := testPassPattern.FindStringSubmatch(line); match != nil {
			summary.PassedTests++
			summary.TotalTests++
			collectingFailure = false
			continue
		}

		if match := testFailPattern.FindStringSubmatch(line); match != nil {
			testName := match[1]

			failure := ParsedTestFailure{
				TestName:    testName,
				FailureType: "assertion",
			}

			// Look for error details in surrounding lines
			errorLines := []string{}
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextLine, "---") || strings.HasPrefix(nextLine, "===") {
					break
				}
				if nextLine != "" {
					errorLines = append(errorLines, nextLine)
				}
			}

			if len(errorLines) > 0 {
				failure.ErrorMessage = strings.Join(errorLines, "\n")

				// Extract file and line info
				if match := fileLinePattern.FindStringSubmatch(failure.ErrorMessage); match != nil {
					failure.TestFile = match[1]
					if lineNum, err := parseIntValue(match[2]); err == nil {
						failure.Line = lineNum
					}
				}

				// Check for panic
				if panicMatch := panicPattern.FindStringSubmatch(failure.ErrorMessage); panicMatch != nil {
					failure.FailureType = "panic"
					failure.ErrorMessage = strings.TrimSpace(panicMatch[1])
				}
			}

			summary.Failures = append(summary.Failures, failure)
			summary.FailedTests++
			summary.TotalTests++
			collectingFailure = true
			currentFailure = &summary.Failures[len(summary.Failures)-1]
			continue
		}

		if match := testSkipPattern.FindStringSubmatch(line); match != nil {
			summary.SkippedTests++
			summary.TotalTests++
			collectingFailure = false
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

		// Collect additional error context if we're in a failure
		if collectingFailure && currentFailure != nil && line != "" {
			if currentFailure.Context == "" {
				currentFailure.Context = line
			} else {
				currentFailure.Context += "\n" + line
			}
		}
	}

	// Generate summary
	if summary.FailedTests > 0 {
		summary.Summary = fmt.Sprintf("%d test(s) failed out of %d total tests", summary.FailedTests, summary.TotalTests)
	} else {
		summary.Summary = fmt.Sprintf("All %d tests passed", summary.TotalTests)
	}

	return summary
}

func (t *ParseTestResultsTool) parseJestOutput(output string) ParsedTestSummary {
	// Basic Jest parsing - can be enhanced later
	summary := ParsedTestSummary{
		Failures: []ParsedTestFailure{},
		Summary:  "Jest output parsing not fully implemented",
	}

	// Look for basic Jest patterns
	if strings.Contains(output, "FAIL") {
		summary.FailedTests = 1
		summary.TotalTests = 1
		summary.Failures = append(summary.Failures, ParsedTestFailure{
			TestName:     "Jest Test",
			ErrorMessage: "Jest test failed - detailed parsing not implemented",
			FailureType:  "assertion",
		})
	}

	return summary
}

func (t *ParseTestResultsTool) parsePytestOutput(output string) ParsedTestSummary {
	// Basic Pytest parsing - can be enhanced later
	summary := ParsedTestSummary{
		Failures: []ParsedTestFailure{},
		Summary:  "Pytest output parsing not fully implemented",
	}

	// Look for basic Pytest patterns
	if strings.Contains(output, "FAILED") {
		summary.FailedTests = 1
		summary.TotalTests = 1
		summary.Failures = append(summary.Failures, ParsedTestFailure{
			TestName:     "Pytest Test",
			ErrorMessage: "Pytest test failed - detailed parsing not implemented",
			FailureType:  "assertion",
		})
	}

	return summary
}

func parseIntValue(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
