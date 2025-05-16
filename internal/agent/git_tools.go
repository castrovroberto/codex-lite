package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GitTool implements Git operations
type GitTool struct {
	workspaceRoot string
}

func NewGitTool(workspaceRoot string) *GitTool {
	return &GitTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *GitTool) Name() string {
	return "git_info"
}

func (t *GitTool) Description() string {
	return "Get Git repository information including current branch, status, and recent commits"
}

func (t *GitTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"include_commits": {
				"type": "boolean",
				"description": "Whether to include recent commit history"
			},
			"commit_count": {
				"type": "integer",
				"description": "Number of recent commits to include (default: 5)"
			}
		}
	}`)
}

type GitParams struct {
	IncludeCommits bool `json:"include_commits,omitempty"`
	CommitCount    int  `json:"commit_count,omitempty"`
}

func (t *GitTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p GitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if p.CommitCount == 0 {
		p.CommitCount = 5
	}

	// Check if Git is initialized
	if !t.isGitRepo() {
		return &ToolResult{
			Success: false,
			Error:   "Not a Git repository",
		}, nil
	}

	// Get current branch
	branch, err := t.getCurrentBranch()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get current branch: %v", err),
		}, nil
	}

	// Get repository status
	status, err := t.getStatus()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get status: %v", err),
		}, nil
	}

	result := map[string]interface{}{
		"branch": branch,
		"status": status,
	}

	// Get recent commits if requested
	if p.IncludeCommits {
		commits, err := t.getRecentCommits(p.CommitCount)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to get commits: %v", err),
			}, nil
		}
		result["recent_commits"] = commits
	}

	return &ToolResult{
		Success: true,
		Data:    result,
	}, nil
}

func (t *GitTool) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = t.workspaceRoot
	return cmd.Run() == nil
}

func (t *GitTool) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = t.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *GitTool) getStatus() (map[string]interface{}, error) {
	// Get status in porcelain format
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = t.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	status := map[string]interface{}{
		"clean": len(out) == 0,
	}

	if len(out) > 0 {
		changes := make(map[string]int)
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			if len(line) < 2 {
				continue
			}
			code := line[:2]
			changes[code]++
		}
		status["changes"] = changes
	}

	return status, nil
}

func (t *GitTool) getRecentCommits(count int) ([]map[string]string, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%h|%an|%s", fmt.Sprintf("-%d", count))
	cmd.Dir = t.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, map[string]string{
			"hash":    parts[0],
			"author":  parts[1],
			"message": parts[2],
		})
	}

	return commits, nil
}
