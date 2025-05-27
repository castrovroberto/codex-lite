package templates

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateToolCall validates a tool call against security and format requirements
func ValidateToolCall(toolName string, params json.RawMessage, workspaceRoot string) error {
	switch toolName {
	case "read_file", "write_file":
		return validateFileOperation(params, workspaceRoot)
	case "run_shell_command":
		return validateShellCommand(params)
	case "apply_patch_to_file":
		return validatePatchOperation(params, workspaceRoot)
	case "list_directory":
		return validateDirectoryOperation(params, workspaceRoot)
	default:
		return nil // Other tools handle their own validation
	}
}

// validateFileOperation validates file read/write operations
func validateFileOperation(params json.RawMessage, workspaceRoot string) error {
	var fileParams struct {
		FilePath   string `json:"file_path"`
		TargetFile string `json:"target_file"`
	}

	if err := json.Unmarshal(params, &fileParams); err != nil {
		return fmt.Errorf("invalid file operation parameters: %w", err)
	}

	filePath := fileParams.FilePath
	if filePath == "" {
		filePath = fileParams.TargetFile
	}

	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Validate path is within workspace
	if filepath.IsAbs(filePath) {
		return fmt.Errorf("absolute paths not allowed: %s", filePath)
	}

	cleanPath := filepath.Clean(filepath.Join(workspaceRoot, filePath))
	if !strings.HasPrefix(cleanPath, filepath.Clean(workspaceRoot)) {
		return fmt.Errorf("path outside workspace: %s", filePath)
	}

	return nil
}

// validateShellCommand validates shell command execution
func validateShellCommand(params json.RawMessage) error {
	var shellParams struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal(params, &shellParams); err != nil {
		return fmt.Errorf("invalid shell command parameters: %w", err)
	}

	if shellParams.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Basic security checks for dangerous commands
	dangerousCommands := []string{"rm -rf", "sudo", "chmod 777", "dd if=", "> /dev/"}
	command := strings.ToLower(shellParams.Command)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(command, dangerous) {
			return fmt.Errorf("potentially dangerous command detected: %s", dangerous)
		}
	}

	return nil
}

// validatePatchOperation validates patch application operations
func validatePatchOperation(params json.RawMessage, workspaceRoot string) error {
	var patchParams struct {
		FilePath     string `json:"file_path"`
		PatchContent string `json:"patch_content"`
	}

	if err := json.Unmarshal(params, &patchParams); err != nil {
		return fmt.Errorf("invalid patch operation parameters: %w", err)
	}

	if patchParams.FilePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if patchParams.PatchContent == "" {
		return fmt.Errorf("patch content cannot be empty")
	}

	// Validate file path
	return validateFileOperation(params, workspaceRoot)
}

// validateDirectoryOperation validates directory listing operations
func validateDirectoryOperation(params json.RawMessage, workspaceRoot string) error {
	var dirParams struct {
		DirectoryPath string `json:"directory_path"`
	}

	if err := json.Unmarshal(params, &dirParams); err != nil {
		return fmt.Errorf("invalid directory operation parameters: %w", err)
	}

	dirPath := dirParams.DirectoryPath
	if dirPath == "" || dirPath == "." {
		return nil // Current directory is always allowed
	}

	// Validate path is within workspace
	if filepath.IsAbs(dirPath) {
		return fmt.Errorf("absolute paths not allowed: %s", dirPath)
	}

	cleanPath := filepath.Clean(filepath.Join(workspaceRoot, dirPath))
	if !strings.HasPrefix(cleanPath, filepath.Clean(workspaceRoot)) {
		return fmt.Errorf("path outside workspace: %s", dirPath)
	}

	return nil
}

// SafetyGuidelines returns a list of safety guidelines for function calling
func SafetyGuidelines() []string {
	return []string{
		"Always use relative paths from the workspace root",
		"Validate file paths before performing operations",
		"Create backups before making destructive changes",
		"Use read_file before modifying existing files",
		"Prefer apply_patch_to_file over write_file for modifications",
		"Run tests after making changes to validate functionality",
		"Handle errors gracefully and provide clear feedback",
		"Avoid running potentially dangerous shell commands",
	}
}
