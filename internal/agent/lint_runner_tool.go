package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// LintRunnerTool implements linting with structured output
type LintRunnerTool struct {
	workspaceRoot string
}

func NewLintRunnerTool(workspaceRoot string) *LintRunnerTool {
	return &LintRunnerTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *LintRunnerTool) Name() string {
	return "run_linter"
}

func (t *LintRunnerTool) Description() string {
	return "Runs linting tools (golangci-lint, go vet, go fmt) with structured output parsing"
}

func (t *LintRunnerTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"target_path": {
				"type": "string",
				"description": "Directory or package path to lint (default: current directory)"
			},
			"linter": {
				"type": "string",
				"enum": ["golangci-lint", "go-vet", "go-fmt", "all"],
				"description": "Which linter to run (default: all)"
			},
			"fix": {
				"type": "boolean",
				"description": "Attempt to automatically fix issues where possible"
			},
			"timeout_seconds": {
				"type": "integer",
				"description": "Linting timeout in seconds (default: 120)"
			}
		}
	}`)
}

type LintRunnerParams struct {
	TargetPath     string `json:"target_path,omitempty"`
	Linter         string `json:"linter,omitempty"`
	Fix            bool   `json:"fix,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type LintIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Rule     string `json:"rule,omitempty"`
	Message  string `json:"message"`
	Linter   string `json:"linter"`
}

type LintSummary struct {
	TotalIssues   int                   `json:"total_issues"`
	ErrorCount    int                   `json:"error_count"`
	WarningCount  int                   `json:"warning_count"`
	InfoCount     int                   `json:"info_count"`
	Issues        []LintIssue           `json:"issues"`
	RawOutput     string                `json:"raw_output"`
	LinterResults map[string]LintResult `json:"linter_results"`
}

type LintResult struct {
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	IssueCount int    `json:"issue_count"`
}

func (t *LintRunnerTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p LintRunnerParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Set defaults
	if p.TargetPath == "" {
		p.TargetPath = "."
	}
	if p.Linter == "" {
		p.Linter = "all"
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = 120
	}

	// Create context with timeout
	lintCtx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutSeconds)*time.Second)
	defer cancel()

	summary := LintSummary{
		Issues:        []LintIssue{},
		LinterResults: make(map[string]LintResult),
	}

	// Run specified linters
	if p.Linter == "all" {
		t.runGoFmt(lintCtx, p, &summary)
		t.runGoVet(lintCtx, p, &summary)
		t.runGolangciLint(lintCtx, p, &summary)
	} else {
		switch p.Linter {
		case "go-fmt":
			t.runGoFmt(lintCtx, p, &summary)
		case "go-vet":
			t.runGoVet(lintCtx, p, &summary)
		case "golangci-lint":
			t.runGolangciLint(lintCtx, p, &summary)
		default:
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("unknown linter: %s", p.Linter),
			}, nil
		}
	}

	// Calculate totals
	for _, issue := range summary.Issues {
		summary.TotalIssues++
		switch issue.Severity {
		case "error":
			summary.ErrorCount++
		case "warning":
			summary.WarningCount++
		case "info":
			summary.InfoCount++
		}
	}

	// Determine overall success
	success := summary.ErrorCount == 0

	return &ToolResult{
		Success: success,
		Data:    summary,
		Error: func() string {
			if !success {
				return fmt.Sprintf("linting found %d error(s)", summary.ErrorCount)
			}
			return ""
		}(),
	}, nil
}

func (t *LintRunnerTool) runGoFmt(ctx context.Context, p LintRunnerParams, summary *LintSummary) {
	args := []string{"fmt"}
	if !p.Fix {
		args = append(args, "-n") // Don't actually format, just show what would be formatted
	}
	args = append(args, p.TargetPath)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = t.workspaceRoot
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result := LintResult{
		Success: err == nil,
		Output:  outputStr,
	}

	if err != nil {
		result.Error = err.Error()
	}

	// Parse go fmt output
	if outputStr != "" {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "go:") {
				issue := LintIssue{
					File:     line,
					Severity: "warning",
					Message:  "File needs formatting",
					Linter:   "go-fmt",
				}
				summary.Issues = append(summary.Issues, issue)
				result.IssueCount++
			}
		}
	}

	summary.LinterResults["go-fmt"] = result
	summary.RawOutput += fmt.Sprintf("=== go fmt ===\n%s\n\n", outputStr)
}

func (t *LintRunnerTool) runGoVet(ctx context.Context, p LintRunnerParams, summary *LintSummary) {
	args := []string{"vet", p.TargetPath}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = t.workspaceRoot
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result := LintResult{
		Success: err == nil,
		Output:  outputStr,
	}

	if err != nil {
		result.Error = err.Error()
	}

	// Parse go vet output
	if outputStr != "" {
		issues := t.parseGoVetOutput(outputStr)
		summary.Issues = append(summary.Issues, issues...)
		result.IssueCount = len(issues)
	}

	summary.LinterResults["go-vet"] = result
	summary.RawOutput += fmt.Sprintf("=== go vet ===\n%s\n\n", outputStr)
}

func (t *LintRunnerTool) runGolangciLint(ctx context.Context, p LintRunnerParams, summary *LintSummary) {
	args := []string{"run"}
	if p.Fix {
		args = append(args, "--fix")
	}
	args = append(args, "--out-format=line-number", p.TargetPath)

	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	cmd.Dir = t.workspaceRoot
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result := LintResult{
		Success: err == nil,
		Output:  outputStr,
	}

	if err != nil {
		// golangci-lint returns non-zero exit code when issues are found
		// Only treat it as an error if it's not just linting issues
		if !strings.Contains(outputStr, ":") {
			result.Error = err.Error()
		} else {
			result.Success = true // Issues found but tool ran successfully
		}
	}

	// Parse golangci-lint output
	if outputStr != "" {
		issues := t.parseGolangciLintOutput(outputStr)
		summary.Issues = append(summary.Issues, issues...)
		result.IssueCount = len(issues)
	}

	summary.LinterResults["golangci-lint"] = result
	summary.RawOutput += fmt.Sprintf("=== golangci-lint ===\n%s\n\n", outputStr)
}

func (t *LintRunnerTool) parseGoVetOutput(output string) []LintIssue {
	var issues []LintIssue

	// go vet output format: file:line:column: message
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+)$`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if match := pattern.FindStringSubmatch(line); match != nil {
			issue := LintIssue{
				File:     match[1],
				Line:     parseInt(match[2]),
				Column:   parseInt(match[3]),
				Severity: "error",
				Message:  match[4],
				Linter:   "go-vet",
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

func (t *LintRunnerTool) parseGolangciLintOutput(output string) []LintIssue {
	var issues []LintIssue

	// golangci-lint line-number format: file:line:column: message (rule)
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+?)\s*\(([^)]+)\)$`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if match := pattern.FindStringSubmatch(line); match != nil {
			severity := "warning"
			if strings.Contains(match[4], "error") {
				severity = "error"
			}

			issue := LintIssue{
				File:     match[1],
				Line:     parseInt(match[2]),
				Column:   parseInt(match[3]),
				Severity: severity,
				Message:  match[4],
				Rule:     match[5],
				Linter:   "golangci-lint",
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
