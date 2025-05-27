package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ParseLintResultsTool implements lint output parsing
type ParseLintResultsTool struct {
	workspaceRoot string
}

func NewParseLintResultsTool(workspaceRoot string) *ParseLintResultsTool {
	return &ParseLintResultsTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *ParseLintResultsTool) Name() string {
	return "parse_lint_results"
}

func (t *ParseLintResultsTool) Description() string {
	return "Parses raw lint output to extract structured information about linting issues, file locations, and severity levels"
}

func (t *ParseLintResultsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"lint_output": {
				"type": "string",
				"description": "Raw lint output to parse"
			},
			"linter_type": {
				"type": "string",
				"enum": ["golangci-lint", "go-vet", "go-fmt", "eslint", "auto"],
				"description": "Linter type (default: auto-detect)"
			}
		},
		"required": ["lint_output"]
	}`)
}

type ParseLintResultsParams struct {
	LintOutput string `json:"lint_output"`
	LinterType string `json:"linter_type,omitempty"`
}

type ParsedLintIssue struct {
	File       string `json:"file"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	Rule       string `json:"rule,omitempty"`
	Message    string `json:"message"`
	Linter     string `json:"linter"`
	Suggestion string `json:"suggestion,omitempty"`
	Context    string `json:"context,omitempty"`
}

type ParsedLintSummary struct {
	TotalIssues   int               `json:"total_issues"`
	ErrorCount    int               `json:"error_count"`
	WarningCount  int               `json:"warning_count"`
	InfoCount     int               `json:"info_count"`
	Issues        []ParsedLintIssue `json:"issues"`
	Summary       string            `json:"summary"`
	LinterResults map[string]int    `json:"linter_results"` // linter name -> issue count
}

func (t *ParseLintResultsTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p ParseLintResultsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if p.LintOutput == "" {
		return &ToolResult{
			Success: false,
			Error:   "lint_output parameter is required",
		}, nil
	}

	// Auto-detect linter if not specified
	if p.LinterType == "" || p.LinterType == "auto" {
		p.LinterType = t.detectLinterType(p.LintOutput)
	}

	summary := ParsedLintSummary{
		Issues:        []ParsedLintIssue{},
		LinterResults: make(map[string]int),
	}

	switch p.LinterType {
	case "golangci-lint":
		t.parseGolangciLintOutput(p.LintOutput, &summary)
	case "go-vet":
		t.parseGoVetOutput(p.LintOutput, &summary)
	case "go-fmt":
		t.parseGoFmtOutput(p.LintOutput, &summary)
	case "eslint":
		t.parseESLintOutput(p.LintOutput, &summary)
	default:
		// Try to parse as generic format
		t.parseGenericLintOutput(p.LintOutput, &summary)
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
		summary.LinterResults[issue.Linter]++
	}

	// Generate summary
	if summary.TotalIssues == 0 {
		summary.Summary = "No linting issues found"
	} else {
		summary.Summary = fmt.Sprintf("Found %d linting issue(s): %d error(s), %d warning(s), %d info",
			summary.TotalIssues, summary.ErrorCount, summary.WarningCount, summary.InfoCount)
	}

	return &ToolResult{
		Success: true,
		Data:    summary,
	}, nil
}

func (t *ParseLintResultsTool) detectLinterType(output string) string {
	output = strings.ToLower(output)

	if strings.Contains(output, "golangci-lint") {
		return "golangci-lint"
	}
	if strings.Contains(output, "go vet") || strings.Contains(output, "vet:") {
		return "go-vet"
	}
	if strings.Contains(output, "gofmt") || strings.Contains(output, "go fmt") {
		return "go-fmt"
	}
	if strings.Contains(output, "eslint") {
		return "eslint"
	}

	return "golangci-lint" // Default fallback for Go projects
}

func (t *ParseLintResultsTool) parseGolangciLintOutput(output string, summary *ParsedLintSummary) {
	lines := strings.Split(output, "\n")

	// golangci-lint line-number format: file:line:column: message (rule)
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+?)\s*\(([^)]+)\)$`)
	// Alternative format without rule: file:line:column: message
	patternNoRule := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var issue ParsedLintIssue
		issue.Linter = "golangci-lint"

		if match := pattern.FindStringSubmatch(line); match != nil {
			issue.File = match[1]
			issue.Line = t.parseIntSafe(match[2])
			issue.Column = t.parseIntSafe(match[3])
			issue.Message = strings.TrimSpace(match[4])
			issue.Rule = match[5]

			// Determine severity based on rule or message content
			issue.Severity = t.determineSeverity(issue.Message, issue.Rule)

			summary.Issues = append(summary.Issues, issue)
		} else if match := patternNoRule.FindStringSubmatch(line); match != nil {
			issue.File = match[1]
			issue.Line = t.parseIntSafe(match[2])
			issue.Column = t.parseIntSafe(match[3])
			issue.Message = strings.TrimSpace(match[4])
			issue.Severity = t.determineSeverity(issue.Message, "")

			summary.Issues = append(summary.Issues, issue)
		}
	}
}

func (t *ParseLintResultsTool) parseGoVetOutput(output string, summary *ParsedLintSummary) {
	lines := strings.Split(output, "\n")

	// go vet output format: file:line:column: message
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if match := pattern.FindStringSubmatch(line); match != nil {
			issue := ParsedLintIssue{
				File:     match[1],
				Line:     t.parseIntSafe(match[2]),
				Column:   t.parseIntSafe(match[3]),
				Message:  strings.TrimSpace(match[4]),
				Severity: "error", // go vet issues are typically errors
				Linter:   "go-vet",
			}

			summary.Issues = append(summary.Issues, issue)
		}
	}
}

func (t *ParseLintResultsTool) parseGoFmtOutput(output string, summary *ParsedLintSummary) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "go:") {
			continue
		}

		// go fmt typically just lists files that need formatting
		issue := ParsedLintIssue{
			File:     line,
			Message:  "File needs formatting",
			Severity: "warning",
			Linter:   "go-fmt",
			Rule:     "formatting",
		}

		summary.Issues = append(summary.Issues, issue)
	}
}

func (t *ParseLintResultsTool) parseESLintOutput(output string, summary *ParsedLintSummary) {
	lines := strings.Split(output, "\n")

	// ESLint format: file:line:column: severity message rule
	pattern := regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(error|warning|info)\s+(.+?)\s+([^\s]+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if match := pattern.FindStringSubmatch(line); match != nil {
			issue := ParsedLintIssue{
				File:     match[1],
				Line:     t.parseIntSafe(match[2]),
				Column:   t.parseIntSafe(match[3]),
				Severity: match[4],
				Message:  strings.TrimSpace(match[5]),
				Rule:     match[6],
				Linter:   "eslint",
			}

			summary.Issues = append(summary.Issues, issue)
		}
	}
}

func (t *ParseLintResultsTool) parseGenericLintOutput(output string, summary *ParsedLintSummary) {
	lines := strings.Split(output, "\n")

	// Generic format: try to match file:line:column: message patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^(.+):(\d+):(\d+):\s*(.+)$`),
		regexp.MustCompile(`^(.+):(\d+):\s*(.+)$`),
		regexp.MustCompile(`^(.+):\s*(.+)$`),
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matched := false
		for i, pattern := range patterns {
			if match := pattern.FindStringSubmatch(line); match != nil {
				issue := ParsedLintIssue{
					Linter:   "generic",
					Severity: "warning", // Default severity
				}

				switch i {
				case 0: // file:line:column: message
					issue.File = match[1]
					issue.Line = t.parseIntSafe(match[2])
					issue.Column = t.parseIntSafe(match[3])
					issue.Message = strings.TrimSpace(match[4])
				case 1: // file:line: message
					issue.File = match[1]
					issue.Line = t.parseIntSafe(match[2])
					issue.Message = strings.TrimSpace(match[3])
				case 2: // file: message
					issue.File = match[1]
					issue.Message = strings.TrimSpace(match[2])
				}

				issue.Severity = t.determineSeverity(issue.Message, "")
				summary.Issues = append(summary.Issues, issue)
				matched = true
				break
			}
		}

		// If no pattern matched but line looks like an issue, add it as generic
		if !matched && (strings.Contains(line, "error") || strings.Contains(line, "warning")) {
			issue := ParsedLintIssue{
				Message:  line,
				Severity: t.determineSeverity(line, ""),
				Linter:   "generic",
			}
			summary.Issues = append(summary.Issues, issue)
		}
	}
}

func (t *ParseLintResultsTool) parseIntSafe(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func (t *ParseLintResultsTool) determineSeverity(message, rule string) string {
	message = strings.ToLower(message)
	rule = strings.ToLower(rule)

	// Check for explicit severity indicators
	if strings.Contains(message, "error") || strings.Contains(rule, "error") {
		return "error"
	}
	if strings.Contains(message, "warning") || strings.Contains(rule, "warning") {
		return "warning"
	}
	if strings.Contains(message, "info") || strings.Contains(rule, "info") {
		return "info"
	}

	// Check for severity based on rule types
	errorRules := []string{"deadcode", "unused", "ineffassign", "misspell"}
	for _, errorRule := range errorRules {
		if strings.Contains(rule, errorRule) {
			return "error"
		}
	}

	// Check for severity based on message content
	if strings.Contains(message, "undefined") || strings.Contains(message, "not found") ||
		strings.Contains(message, "cannot") || strings.Contains(message, "invalid") {
		return "error"
	}

	return "warning" // Default to warning
}
