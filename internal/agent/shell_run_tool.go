package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ShellRunTool implements shell command execution capabilities
type ShellRunTool struct {
	workspaceRoot   string
	allowedCommands []string
}

// NewShellRunTool creates a new shell run tool with security restrictions
func NewShellRunTool(workspaceRoot string) *ShellRunTool {
	// Default allowed commands - can be expanded based on needs
	allowedCommands := []string{
		"go", "git", "ls", "cat", "grep", "find", "wc", "head", "tail",
		"npm", "yarn", "node", "python", "python3", "pip", "pip3",
		"make", "cmake", "cargo", "rustc", "javac", "java", "mvn",
		"docker", "kubectl", "helm", "terraform",
		"test", "echo", "pwd", "which", "whoami",
	}

	return &ShellRunTool{
		workspaceRoot:   workspaceRoot,
		allowedCommands: allowedCommands,
	}
}

func (t *ShellRunTool) Name() string {
	return "run_shell_command"
}

func (t *ShellRunTool) Description() string {
	return "Executes a shell command and returns its standard output and standard error. Use with caution - only allowed commands can be executed."
}

func (t *ShellRunTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"working_directory": {
				"type": "string",
				"description": "Working directory for the command (relative to workspace root, defaults to workspace root)"
			},
			"timeout_seconds": {
				"type": "integer",
				"description": "Timeout for command execution in seconds",
				"default": 30
			}
		},
		"required": ["command"]
	}`)
}

type ShellRunParams struct {
	Command          string `json:"command"`
	WorkingDirectory string `json:"working_directory"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
}

func (t *ShellRunTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p ShellRunParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Default timeout
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = 30
	}

	// Validate command
	if p.Command == "" {
		return &ToolResult{
			Success: false,
			Error:   "command cannot be empty",
		}, nil
	}

	// Security check: validate command against allowed list
	if !t.isCommandAllowed(p.Command) {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("command not allowed: %s. Allowed commands: %s",
				strings.Fields(p.Command)[0], strings.Join(t.allowedCommands, ", ")),
		}, nil
	}

	// Set working directory
	workDir := t.workspaceRoot
	if p.WorkingDirectory != "" {
		workDir = filepath.Join(t.workspaceRoot, p.WorkingDirectory)
		// Security check: ensure working directory is within workspace
		cleanWorkDir := filepath.Clean(workDir)
		if !strings.HasPrefix(cleanWorkDir, filepath.Clean(t.workspaceRoot)) {
			return &ToolResult{
				Success: false,
				Error:   "working directory is outside workspace root",
			}, nil
		}
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutSeconds)*time.Second)
	defer cancel()

	// Parse command and arguments
	parts := strings.Fields(p.Command)
	if len(parts) == 0 {
		return &ToolResult{
			Success: false,
			Error:   "invalid command format",
		}, nil
	}

	// Create and configure command
	cmd := exec.CommandContext(cmdCtx, parts[0], parts[1:]...)
	cmd.Dir = workDir

	// Execute command and capture output
	output, err := cmd.CombinedOutput()

	// Determine if command was successful
	success := err == nil
	var errorMsg string
	var exitCode int

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			errorMsg = fmt.Sprintf("command timed out after %d seconds", p.TimeoutSeconds)
		} else if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
			errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
		} else {
			errorMsg = fmt.Sprintf("command execution failed: %v", err)
		}
	}

	result := map[string]interface{}{
		"command":           p.Command,
		"working_directory": p.WorkingDirectory,
		"output":            string(output),
		"success":           success,
		"exit_code":         exitCode,
	}

	if errorMsg != "" {
		result["error_message"] = errorMsg
	}

	return &ToolResult{
		Success: success,
		Data:    result,
		Error:   errorMsg,
	}, nil
}

// isCommandAllowed checks if a command is in the allowed list
func (t *ShellRunTool) isCommandAllowed(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	baseCommand := parts[0]

	// Remove path if present (e.g., "/usr/bin/git" -> "git")
	baseCommand = filepath.Base(baseCommand)

	// Check against allowed commands
	for _, allowed := range t.allowedCommands {
		if baseCommand == allowed {
			return true
		}
	}

	return false
}

// AddAllowedCommand adds a command to the allowed list
func (t *ShellRunTool) AddAllowedCommand(command string) {
	for _, existing := range t.allowedCommands {
		if existing == command {
			return // Already exists
		}
	}
	t.allowedCommands = append(t.allowedCommands, command)
}

// RemoveAllowedCommand removes a command from the allowed list
func (t *ShellRunTool) RemoveAllowedCommand(command string) {
	for i, existing := range t.allowedCommands {
		if existing == command {
			t.allowedCommands = append(t.allowedCommands[:i], t.allowedCommands[i+1:]...)
			return
		}
	}
}

// GetAllowedCommands returns the list of allowed commands
func (t *ShellRunTool) GetAllowedCommands() []string {
	return append([]string(nil), t.allowedCommands...) // Return a copy
}
