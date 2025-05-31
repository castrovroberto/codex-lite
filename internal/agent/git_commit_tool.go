package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GitCommitTool implements Git commit operations
type GitCommitTool struct {
	workspaceRoot string
}

func NewGitCommitTool(workspaceRoot string) *GitCommitTool {
	return &GitCommitTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *GitCommitTool) Name() string {
	return "git_commit"
}

func (t *GitCommitTool) Description() string {
	return "Stages specified files and creates a Git commit with the given message. If no files specified, stages all changes."
}

func (t *GitCommitTool) Parameters() json.RawMessage {
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
			}
		},
		"required": ["commit_message"]
	}`)
}

type GitCommitParams struct {
	CommitMessage string   `json:"commit_message"`
	FilesToStage  []string `json:"files_to_stage,omitempty"`
	AllowEmpty    bool     `json:"allow_empty,omitempty"`
}

func (t *GitCommitTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p GitCommitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate commit message
	if strings.TrimSpace(p.CommitMessage) == "" {
		return &ToolResult{
			Success: false,
			Error:   "commit message cannot be empty",
		}, nil
	}

	// Check if Git is initialized
	if !t.isGitRepo() {
		return &ToolResult{
			Success: false,
			Error:   "not a Git repository",
		}, nil
	}

	// Stage files
	if err := t.stageFiles(p.FilesToStage); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to stage files: %v", err),
		}, nil
	}

	// Check if there are changes to commit (unless allow_empty is true)
	if !p.AllowEmpty {
		hasChanges, err := t.hasChangesToCommit()
		if err != nil {
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
	commitHash, err := t.createCommit(p.CommitMessage, p.AllowEmpty)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create commit: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"commit_hash":    commitHash,
			"commit_message": p.CommitMessage,
			"files_staged":   len(p.FilesToStage),
		},
	}, nil
}

func (t *GitCommitTool) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = t.workspaceRoot
	return cmd.Run() == nil
}

func (t *GitCommitTool) stageFiles(files []string) error {
	if len(files) == 0 {
		// Stage all changes
		cmd := exec.Command("git", "add", ".")
		cmd.Dir = t.workspaceRoot
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stage all files: %w", err)
		}
	} else {
		// Stage specific files
		args := append([]string{"add"}, files...)
		cmd := exec.Command("git", args...)
		cmd.Dir = t.workspaceRoot
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stage files %v: %w", files, err)
		}
	}
	return nil
}

func (t *GitCommitTool) hasChangesToCommit() (bool, error) {
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

func (t *GitCommitTool) createCommit(message string, allowEmpty bool) (string, error) {
	args := []string{"commit", "-m", message}
	if allowEmpty {
		args = append(args, "--allow-empty")
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
