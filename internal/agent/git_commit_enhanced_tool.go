package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/audit"
)

// EnhancedGitCommitTool implements enhanced Git commit operations with automation features
type EnhancedGitCommitTool struct {
	workspaceRoot string
	auditLogger   *audit.AuditLogger
}

func NewEnhancedGitCommitTool(workspaceRoot string, auditLogger *audit.AuditLogger) *EnhancedGitCommitTool {
	return &EnhancedGitCommitTool{
		workspaceRoot: workspaceRoot,
		auditLogger:   auditLogger,
	}
}

func (t *EnhancedGitCommitTool) Name() string {
	return "git_commit_enhanced"
}

func (t *EnhancedGitCommitTool) Description() string {
	return "Enhanced Git commit tool with automated workflows, smart staging, and audit logging. Supports auto-commit patterns and commit message templates."
}

func (t *EnhancedGitCommitTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"commit_message": {
				"type": "string",
				"description": "The commit message"
			},
			"files_to_stage": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of file paths to stage. If empty, stages all changes."
			},
			"allow_empty": {
				"type": "boolean",
				"description": "Allow empty commits (default: false)"
			},
			"auto_generate_message": {
				"type": "boolean",
				"description": "Auto-generate commit message based on changes (default: false)"
			},
			"commit_type": {
				"type": "string",
				"enum": ["feat", "fix", "docs", "style", "refactor", "test", "chore"],
				"description": "Type of commit for conventional commits format"
			},
			"scope": {
				"type": "string",
				"description": "Scope of the commit (for conventional commits)"
			},
			"breaking_change": {
				"type": "boolean",
				"description": "Whether this is a breaking change (default: false)"
			},
			"co_authors": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of co-authors in format 'Name <email>'"
			},
			"skip_hooks": {
				"type": "boolean",
				"description": "Skip Git hooks during commit (default: false)"
			}
		},
		"required": ["commit_message"]
	}`)
}

type EnhancedGitCommitParams struct {
	CommitMessage       string   `json:"commit_message"`
	FilesToStage        []string `json:"files_to_stage,omitempty"`
	AllowEmpty          bool     `json:"allow_empty,omitempty"`
	AutoGenerateMessage bool     `json:"auto_generate_message,omitempty"`
	CommitType          string   `json:"commit_type,omitempty"`
	Scope               string   `json:"scope,omitempty"`
	BreakingChange      bool     `json:"breaking_change,omitempty"`
	CoAuthors           []string `json:"co_authors,omitempty"`
	SkipHooks           bool     `json:"skip_hooks,omitempty"`
}

func (t *EnhancedGitCommitTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	startTime := time.Now()
	var p EnhancedGitCommitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Check if Git is initialized
	if !t.isGitRepo() {
		err := fmt.Errorf("not a Git repository")
		if t.auditLogger != nil {
			t.auditLogger.LogError(audit.OpCommit, "git_commit_enhanced", err, map[string]interface{}{
				"workspace_root": t.workspaceRoot,
			})
		}
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Auto-generate commit message if requested
	if p.AutoGenerateMessage {
		generatedMessage, err := t.generateCommitMessage(p.FilesToStage)
		if err == nil && generatedMessage != "" {
			p.CommitMessage = generatedMessage
		}
	}

	// Format commit message with conventional commits if type is specified
	if p.CommitType != "" {
		p.CommitMessage = t.formatConventionalCommit(p.CommitType, p.Scope, p.CommitMessage, p.BreakingChange)
	}

	// Add co-authors to commit message
	if len(p.CoAuthors) > 0 {
		p.CommitMessage = t.addCoAuthors(p.CommitMessage, p.CoAuthors)
	}

	// Validate commit message
	if strings.TrimSpace(p.CommitMessage) == "" {
		err := fmt.Errorf("commit message cannot be empty")
		if t.auditLogger != nil {
			t.auditLogger.LogError(audit.OpCommit, "git_commit_enhanced", err, map[string]interface{}{
				"auto_generate": p.AutoGenerateMessage,
			})
		}
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Stage files
	stagedFiles, err := t.stageFiles(p.FilesToStage)
	if err != nil {
		if t.auditLogger != nil {
			t.auditLogger.LogError(audit.OpCommit, "git_commit_enhanced", err, map[string]interface{}{
				"files_to_stage": p.FilesToStage,
			})
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to stage files: %v", err),
		}, nil
	}

	// Check if there are changes to commit (unless allow_empty is true)
	if !p.AllowEmpty {
		hasChanges, err := t.hasChangesToCommit()
		if err != nil {
			if t.auditLogger != nil {
				t.auditLogger.LogError(audit.OpCommit, "git_commit_enhanced", err, nil)
			}
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to check for changes: %v", err),
			}, nil
		}
		if !hasChanges {
			return &ToolResult{
				Success: false,
				Error:   "no changes to commit",
			}, nil
		}
	}

	// Create commit
	commitHash, err := t.createCommit(p.CommitMessage, p.AllowEmpty, p.SkipHooks)
	duration := time.Since(startTime)

	if err != nil {
		if t.auditLogger != nil {
			t.auditLogger.LogGitCommit("", p.CommitMessage, stagedFiles, false, err)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create commit: %v", err),
		}, nil
	}

	// Log successful commit
	if t.auditLogger != nil {
		t.auditLogger.LogGitCommit(commitHash, p.CommitMessage, stagedFiles, true, nil)
	}

	responseData := map[string]interface{}{
		"commit_hash":     commitHash,
		"commit_message":  p.CommitMessage,
		"files_staged":    stagedFiles,
		"files_count":     len(stagedFiles),
		"duration_ms":     duration.Milliseconds(),
		"conventional":    p.CommitType != "",
		"auto_generated":  p.AutoGenerateMessage,
		"breaking_change": p.BreakingChange,
	}

	if len(p.CoAuthors) > 0 {
		responseData["co_authors"] = p.CoAuthors
	}

	return &ToolResult{
		Success: true,
		Data:    responseData,
	}, nil
}

// generateCommitMessage auto-generates a commit message based on staged changes
func (t *EnhancedGitCommitTool) generateCommitMessage(filesToStage []string) (string, error) {
	// Get diff summary
	cmd := exec.Command("git", "diff", "--cached", "--stat")
	cmd.Dir = t.workspaceRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	if len(output) == 0 {
		return "", fmt.Errorf("no staged changes found")
	}

	// Simple heuristic-based message generation
	statLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(statLines) == 0 {
		return "", fmt.Errorf("no changes detected")
	}

	// Count file types and changes
	var (
		addedFiles    int
		modifiedFiles int
		deletedFiles  int
		goFiles       int
		testFiles     int
		docFiles      int
	)

	for _, line := range statLines {
		if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
			continue // Skip summary line
		}

		parts := strings.Fields(line)
		if len(parts) > 0 {
			fileName := parts[0]

			// Check file types
			if strings.HasSuffix(fileName, ".go") {
				goFiles++
				if strings.Contains(fileName, "_test.go") {
					testFiles++
				}
			} else if strings.HasSuffix(fileName, ".md") || strings.HasSuffix(fileName, ".txt") {
				docFiles++
			}

			// Determine operation type (simplified)
			if strings.Contains(line, "+++") {
				addedFiles++
			} else if strings.Contains(line, "---") {
				deletedFiles++
			} else {
				modifiedFiles++
			}
		}
	}

	// Generate message based on patterns
	if testFiles > 0 && goFiles == testFiles {
		return "test: add/update test cases", nil
	} else if docFiles > 0 && goFiles == 0 {
		return "docs: update documentation", nil
	} else if addedFiles > modifiedFiles {
		return "feat: add new functionality", nil
	} else if deletedFiles > 0 {
		return "refactor: remove unused code", nil
	} else {
		return "fix: update implementation", nil
	}
}

// formatConventionalCommit formats a commit message according to conventional commits
func (t *EnhancedGitCommitTool) formatConventionalCommit(commitType, scope, message string, breakingChange bool) string {
	var formatted strings.Builder

	formatted.WriteString(commitType)

	if scope != "" {
		formatted.WriteString("(")
		formatted.WriteString(scope)
		formatted.WriteString(")")
	}

	if breakingChange {
		formatted.WriteString("!")
	}

	formatted.WriteString(": ")
	formatted.WriteString(message)

	return formatted.String()
}

// addCoAuthors adds co-author information to the commit message
func (t *EnhancedGitCommitTool) addCoAuthors(message string, coAuthors []string) string {
	var result strings.Builder
	result.WriteString(message)
	result.WriteString("\n\n")

	for _, author := range coAuthors {
		result.WriteString("Co-authored-by: ")
		result.WriteString(author)
		result.WriteString("\n")
	}

	return result.String()
}

// isGitRepo checks if the workspace is a Git repository
func (t *EnhancedGitCommitTool) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = t.workspaceRoot
	return cmd.Run() == nil
}

// stageFiles stages the specified files and returns the list of actually staged files
func (t *EnhancedGitCommitTool) stageFiles(files []string) ([]string, error) {
	if len(files) == 0 {
		// Stage all changes
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = t.workspaceRoot
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to stage all files: %w", err)
		}
	} else {
		// Stage specific files
		args := append([]string{"add"}, files...)
		cmd := exec.Command("git", args...)
		cmd.Dir = t.workspaceRoot
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to stage files %v: %w", files, err)
		}
	}

	// Get list of actually staged files
	return t.getStagedFiles()
}

// getStagedFiles returns the list of currently staged files
func (t *EnhancedGitCommitTool) getStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = t.workspaceRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, file := range files {
		if strings.TrimSpace(file) != "" {
			result = append(result, file)
		}
	}
	return result, nil
}

// hasChangesToCommit checks if there are staged changes ready to commit
func (t *EnhancedGitCommitTool) hasChangesToCommit() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = t.workspaceRoot
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means there are differences (changes to commit)
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	// Exit code 0 means no differences
	return false, nil
}

// createCommit creates a Git commit with the specified message
func (t *EnhancedGitCommitTool) createCommit(message string, allowEmpty bool, skipHooks bool) (string, error) {
	args := []string{"commit", "-m", message}

	if allowEmpty {
		args = append(args, "--allow-empty")
	}

	if skipHooks {
		args = append(args, "--no-verify")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = t.workspaceRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	// Extract commit hash from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			// Look for pattern like "[main abc1234] commit message"
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, "]") && len(part) > 1 {
					hash := strings.TrimSuffix(part, "]")
					if len(hash) >= 7 { // Git short hash is typically 7+ chars
						return hash, nil
					}
				}
			}
		}
	}

	// Fallback: get the latest commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.workspaceRoot
	hashOutput, err := cmd.Output()
	if err != nil {
		return "unknown", nil // Commit was created but we couldn't get the hash
	}

	return strings.TrimSpace(string(hashOutput))[:8], nil // Return short hash
}
