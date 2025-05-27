package patchutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/go-diff/diff"
)

// ApplyOptions configures patch application behavior
type ApplyOptions struct {
	CreateBackup     bool
	BackupSuffix     string
	DryRun           bool
	IgnoreWhitespace bool
}

// DefaultApplyOptions returns sensible defaults for patch application
func DefaultApplyOptions() ApplyOptions {
	return ApplyOptions{
		CreateBackup:     true,
		BackupSuffix:     ".bak",
		DryRun:           false,
		IgnoreWhitespace: false,
	}
}

// ApplyResult contains information about the patch application
type ApplyResult struct {
	FilePath     string `json:"file_path"`
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	BackupPath   string `json:"backup_path,omitempty"`
	HunksApplied int    `json:"hunks_applied"`
	LinesChanged int    `json:"lines_changed"`
	OriginalSize int    `json:"original_size"`
	NewSize      int    `json:"new_size"`
}

// PatchApplier handles robust patch application using go-diff
type PatchApplier struct {
	workspaceRoot string
	options       ApplyOptions
}

// NewPatchApplier creates a new patch applier
func NewPatchApplier(workspaceRoot string, options ApplyOptions) *PatchApplier {
	return &PatchApplier{
		workspaceRoot: workspaceRoot,
		options:       options,
	}
}

// ApplyPatch applies a unified diff patch to a file
func (pa *PatchApplier) ApplyPatch(filePath, patchContent string) (*ApplyResult, error) {
	result := &ApplyResult{
		FilePath: filePath,
	}

	// Validate and resolve file path
	fullPath := filepath.Join(pa.workspaceRoot, filePath)
	cleanPath := filepath.Clean(fullPath)

	// Security check: ensure path is within workspace
	if !strings.HasPrefix(cleanPath, filepath.Clean(pa.workspaceRoot)) {
		result.Error = "file path is outside workspace root"
		return result, fmt.Errorf(result.Error)
	}

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		result.Error = fmt.Sprintf("file does not exist: %s", filePath)
		return result, fmt.Errorf(result.Error)
	}

	// Read original file
	originalContent, err := os.ReadFile(cleanPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read original file: %v", err)
		return result, fmt.Errorf(result.Error)
	}
	result.OriginalSize = len(originalContent)

	// Parse the patch using go-diff
	fileDiffs, err := diff.ParseMultiFileDiff([]byte(patchContent))
	if err != nil {
		result.Error = fmt.Sprintf("failed to parse patch: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	if len(fileDiffs) == 0 {
		result.Error = "no file diffs found in patch"
		return result, fmt.Errorf(result.Error)
	}

	// Find the relevant file diff (should match our target file)
	var targetDiff *diff.FileDiff
	for _, fileDiff := range fileDiffs {
		// Match by filename (handle both relative and absolute paths)
		if strings.HasSuffix(fileDiff.NewName, filePath) ||
			strings.HasSuffix(fileDiff.OrigName, filePath) ||
			fileDiff.NewName == filePath ||
			fileDiff.OrigName == filePath {
			targetDiff = fileDiff
			break
		}
	}

	if targetDiff == nil {
		result.Error = fmt.Sprintf("no diff found for file %s in patch", filePath)
		return result, fmt.Errorf(result.Error)
	}

	// Create backup if requested and not in dry run mode
	var backupPath string
	if pa.options.CreateBackup && !pa.options.DryRun {
		timestamp := time.Now().Unix()
		backupPath = fmt.Sprintf("%s%s.%d", cleanPath, pa.options.BackupSuffix, timestamp)
		if err := os.WriteFile(backupPath, originalContent, 0600); err != nil {
			result.Error = fmt.Sprintf("failed to create backup: %v", err)
			return result, fmt.Errorf(result.Error)
		}
		result.BackupPath = strings.TrimPrefix(backupPath, pa.workspaceRoot+string(filepath.Separator))
	}

	// Apply the patch
	patchedContent, err := pa.applyFileDiff(string(originalContent), targetDiff)
	if err != nil {
		// Clean up backup on failure
		if backupPath != "" {
			os.Remove(backupPath)
		}
		result.Error = fmt.Sprintf("failed to apply patch: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	result.NewSize = len(patchedContent)
	result.HunksApplied = len(targetDiff.Hunks)
	result.LinesChanged = pa.countChangedLines(targetDiff)

	// Write patched content (unless dry run)
	if !pa.options.DryRun {
		if err := os.WriteFile(cleanPath, []byte(patchedContent), 0600); err != nil {
			// Try to restore from backup on write failure
			if backupPath != "" {
				os.WriteFile(cleanPath, originalContent, 0600)
				os.Remove(backupPath)
			}
			result.Error = fmt.Sprintf("failed to write patched file: %v", err)
			return result, fmt.Errorf(result.Error)
		}
	}

	result.Success = true
	return result, nil
}

// applyFileDiff applies a single file diff to content
func (pa *PatchApplier) applyFileDiff(content string, fileDiff *diff.FileDiff) (string, error) {
	lines := strings.Split(content, "\n")

	// Apply hunks in reverse order to maintain line numbers
	for i := len(fileDiff.Hunks) - 1; i >= 0; i-- {
		hunk := fileDiff.Hunks[i]
		newLines, err := pa.applyHunk(lines, hunk)
		if err != nil {
			return "", fmt.Errorf("failed to apply hunk at line %d: %v", hunk.OrigStartLine, err)
		}
		lines = newLines
	}

	return strings.Join(lines, "\n"), nil
}

// applyHunk applies a single hunk to the lines
func (pa *PatchApplier) applyHunk(lines []string, hunk *diff.Hunk) ([]string, error) {
	// Convert to 0-based indexing
	startIdx := int(hunk.OrigStartLine) - 1
	if startIdx < 0 {
		startIdx = 0
	}

	var newHunkLines []string
	originalIdx := startIdx
	hunkLines := strings.Split(string(hunk.Body), "\n")

	for _, line := range hunkLines {
		if len(line) == 0 {
			continue
		}

		lineType := line[0]
		content := ""
		if len(line) > 1 {
			content = line[1:]
		}

		switch lineType {
		case ' ': // Context line - should match original
			if originalIdx >= len(lines) {
				return nil, fmt.Errorf("context line beyond end of file")
			}
			if !pa.options.IgnoreWhitespace && lines[originalIdx] != content {
				return nil, fmt.Errorf("context line mismatch at line %d: expected %q, got %q",
					originalIdx+1, content, lines[originalIdx])
			}
			newHunkLines = append(newHunkLines, content)
			originalIdx++
		case '-': // Remove line - should match original
			if originalIdx >= len(lines) {
				return nil, fmt.Errorf("remove line beyond end of file")
			}
			if !pa.options.IgnoreWhitespace && lines[originalIdx] != content {
				return nil, fmt.Errorf("remove line mismatch at line %d: expected %q, got %q",
					originalIdx+1, content, lines[originalIdx])
			}
			// Don't add to newHunkLines (line is removed)
			originalIdx++
		case '+': // Add line
			newHunkLines = append(newHunkLines, content)
			// Don't increment originalIdx (line is added)
		}
	}

	// Reconstruct the file
	result := make([]string, 0, len(lines)+len(newHunkLines))
	result = append(result, lines[:startIdx]...)
	result = append(result, newHunkLines...)
	result = append(result, lines[originalIdx:]...)

	return result, nil
}

// countChangedLines counts the number of lines changed in a file diff
func (pa *PatchApplier) countChangedLines(fileDiff *diff.FileDiff) int {
	count := 0
	for _, hunk := range fileDiff.Hunks {
		hunkLines := strings.Split(string(hunk.Body), "\n")
		for _, line := range hunkLines {
			if len(line) > 0 && (line[0] == '+' || line[0] == '-') {
				count++
			}
		}
	}
	return count
}

// ApplyMultiplePatches applies multiple patches with rollback on failure
func (pa *PatchApplier) ApplyMultiplePatches(patches map[string]string) ([]ApplyResult, error) {
	var results []ApplyResult
	var appliedFiles []string

	// Apply patches one by one
	for filePath, patchContent := range patches {
		result, err := pa.ApplyPatch(filePath, patchContent)
		results = append(results, *result)

		if err != nil {
			// Rollback all previously applied patches
			pa.rollbackPatches(appliedFiles)
			return results, fmt.Errorf("patch application failed for %s: %v", filePath, err)
		}

		if result.Success {
			appliedFiles = append(appliedFiles, filePath)
		}
	}

	return results, nil
}

// rollbackPatches restores files from their backups
func (pa *PatchApplier) rollbackPatches(filePaths []string) {
	for _, filePath := range filePaths {
		fullPath := filepath.Join(pa.workspaceRoot, filePath)

		// Look for backup files
		pattern := fmt.Sprintf("%s%s.*", fullPath, pa.options.BackupSuffix)
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}

		// Use the most recent backup
		backupPath := matches[len(matches)-1]
		if backupContent, err := os.ReadFile(backupPath); err == nil {
			os.WriteFile(fullPath, backupContent, 0600)
			os.Remove(backupPath) // Clean up backup after restore
		}
	}
}
