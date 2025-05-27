package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PatchApplyTool implements patch/diff application capabilities
type PatchApplyTool struct {
	workspaceRoot string
}

// NewPatchApplyTool creates a new patch apply tool
func NewPatchApplyTool(workspaceRoot string) *PatchApplyTool {
	return &PatchApplyTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *PatchApplyTool) Name() string {
	return "apply_patch_to_file"
}

func (t *PatchApplyTool) Description() string {
	return "Applies a diff/patch (in unified diff format) to a specified file. Creates backup before applying."
}

func (t *PatchApplyTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The path to the file to patch (relative to workspace root)"
			},
			"patch_content": {
				"type": "string",
				"description": "The patch content in unified diff format"
			},
			"backup_original": {
				"type": "boolean",
				"description": "Whether to create a backup of the original file",
				"default": true
			}
		},
		"required": ["file_path", "patch_content"]
	}`)
}

type PatchApplyParams struct {
	FilePath       string `json:"file_path"`
	PatchContent   string `json:"patch_content"`
	BackupOriginal bool   `json:"backup_original"`
}

type PatchHunk struct {
	OriginalStart int
	OriginalCount int
	NewStart      int
	NewCount      int
	Lines         []PatchLine
}

type PatchLine struct {
	Type    string // " " (context), "-" (remove), "+" (add)
	Content string
}

func (t *PatchApplyTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p PatchApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Default backup to true
	if !p.BackupOriginal {
		p.BackupOriginal = true
	}

	// Validate file path
	if p.FilePath == "" {
		return &ToolResult{
			Success: false,
			Error:   "file_path cannot be empty",
		}, nil
	}

	// Security check: ensure path is within workspace
	fullPath := filepath.Join(t.workspaceRoot, p.FilePath)
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(t.workspaceRoot)) {
		return &ToolResult{
			Success: false,
			Error:   "file path is outside workspace root",
		}, nil
	}

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file does not exist: %s", p.FilePath),
		}, nil
	}

	// Read original file
	originalContent, err := os.ReadFile(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read original file: %v", err),
		}, nil
	}

	// Create backup if requested
	var backupPath string
	if p.BackupOriginal {
		backupPath = cleanPath + ".bak." + strconv.FormatInt(time.Now().Unix(), 10)
		if err := os.WriteFile(backupPath, originalContent, 0644); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create backup: %v", err),
			}, nil
		}
	}

	// Parse patch
	hunks, err := t.parsePatch(p.PatchContent)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse patch: %v", err),
		}, nil
	}

	// Apply patch
	patchedContent, err := t.applyPatch(string(originalContent), hunks)
	if err != nil {
		// Clean up backup on failure
		if p.BackupOriginal && backupPath != "" {
			os.Remove(backupPath)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to apply patch: %v", err),
		}, nil
	}

	// Write patched content
	if err := os.WriteFile(cleanPath, []byte(patchedContent), 0644); err != nil {
		// Try to restore from backup on write failure
		if p.BackupOriginal && backupPath != "" {
			os.WriteFile(cleanPath, originalContent, 0644)
			os.Remove(backupPath)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write patched file: %v", err),
		}, nil
	}

	result := map[string]interface{}{
		"file_path":     p.FilePath,
		"hunks_applied": len(hunks),
		"original_size": len(originalContent),
		"patched_size":  len(patchedContent),
		"message":       fmt.Sprintf("Successfully applied patch to %s", p.FilePath),
	}

	if p.BackupOriginal {
		result["backup_path"] = strings.TrimPrefix(backupPath, t.workspaceRoot+string(filepath.Separator))
	}

	return &ToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// parsePatch parses a unified diff format patch
func (t *PatchApplyTool) parsePatch(patchContent string) ([]PatchHunk, error) {
	lines := strings.Split(patchContent, "\n")
	var hunks []PatchHunk
	var currentHunk *PatchHunk

	for i, line := range lines {
		// Skip header lines (--- and +++)
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		// Hunk header: @@ -start,count +start,count @@
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}

			hunk, err := t.parseHunkHeader(line)
			if err != nil {
				return nil, fmt.Errorf("invalid hunk header at line %d: %v", i+1, err)
			}
			currentHunk = &hunk
			continue
		}

		// Hunk content lines
		if currentHunk != nil {
			if len(line) == 0 {
				// Empty line is treated as context
				currentHunk.Lines = append(currentHunk.Lines, PatchLine{
					Type:    " ",
					Content: "",
				})
			} else {
				lineType := string(line[0])
				content := ""
				if len(line) > 1 {
					content = line[1:]
				}

				if lineType == " " || lineType == "-" || lineType == "+" {
					currentHunk.Lines = append(currentHunk.Lines, PatchLine{
						Type:    lineType,
						Content: content,
					})
				}
			}
		}
	}

	// Add the last hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks, nil
}

// parseHunkHeader parses a hunk header line like "@@ -1,4 +1,6 @@"
func (t *PatchApplyTool) parseHunkHeader(line string) (PatchHunk, error) {
	// Remove @@ from start and end
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "@@") || !strings.HasSuffix(line, "@@") {
		return PatchHunk{}, fmt.Errorf("invalid hunk header format")
	}

	line = strings.TrimPrefix(line, "@@")
	line = strings.TrimSuffix(line, "@@")
	line = strings.TrimSpace(line)

	// Split into old and new parts
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return PatchHunk{}, fmt.Errorf("invalid hunk header format")
	}

	oldPart := parts[0] // -start,count
	newPart := parts[1] // +start,count

	// Parse old part
	if !strings.HasPrefix(oldPart, "-") {
		return PatchHunk{}, fmt.Errorf("invalid old part format")
	}
	oldPart = strings.TrimPrefix(oldPart, "-")
	oldStart, oldCount, err := t.parseRange(oldPart)
	if err != nil {
		return PatchHunk{}, fmt.Errorf("invalid old range: %v", err)
	}

	// Parse new part
	if !strings.HasPrefix(newPart, "+") {
		return PatchHunk{}, fmt.Errorf("invalid new part format")
	}
	newPart = strings.TrimPrefix(newPart, "+")
	newStart, newCount, err := t.parseRange(newPart)
	if err != nil {
		return PatchHunk{}, fmt.Errorf("invalid new range: %v", err)
	}

	return PatchHunk{
		OriginalStart: oldStart,
		OriginalCount: oldCount,
		NewStart:      newStart,
		NewCount:      newCount,
		Lines:         []PatchLine{},
	}, nil
}

// parseRange parses a range like "1,4" or "1" (count defaults to 1)
func (t *PatchApplyTool) parseRange(rangeStr string) (start, count int, err error) {
	if strings.Contains(rangeStr, ",") {
		parts := strings.Split(rangeStr, ",")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid range format")
		}
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		count, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
	} else {
		start, err = strconv.Atoi(rangeStr)
		if err != nil {
			return 0, 0, err
		}
		count = 1
	}
	return start, count, nil
}

// applyPatch applies the parsed hunks to the original content
func (t *PatchApplyTool) applyPatch(originalContent string, hunks []PatchHunk) (string, error) {
	lines := strings.Split(originalContent, "\n")

	// Apply hunks in reverse order to maintain line numbers
	for i := len(hunks) - 1; i >= 0; i-- {
		hunk := hunks[i]

		// Validate hunk can be applied
		if hunk.OriginalStart < 1 || hunk.OriginalStart > len(lines)+1 {
			return "", fmt.Errorf("hunk original start line %d is out of range", hunk.OriginalStart)
		}

		// Apply the hunk
		newLines, err := t.applyHunk(lines, hunk)
		if err != nil {
			return "", fmt.Errorf("failed to apply hunk at line %d: %v", hunk.OriginalStart, err)
		}
		lines = newLines
	}

	return strings.Join(lines, "\n"), nil
}

// applyHunk applies a single hunk to the lines
func (t *PatchApplyTool) applyHunk(lines []string, hunk PatchHunk) ([]string, error) {
	// Convert to 0-based indexing
	startIdx := hunk.OriginalStart - 1

	// Collect the new lines for this hunk
	var newHunkLines []string
	originalIdx := startIdx

	for _, patchLine := range hunk.Lines {
		switch patchLine.Type {
		case " ": // Context line - should match original
			if originalIdx >= len(lines) {
				return nil, fmt.Errorf("context line beyond end of file")
			}
			if lines[originalIdx] != patchLine.Content {
				return nil, fmt.Errorf("context line mismatch at line %d: expected %q, got %q",
					originalIdx+1, patchLine.Content, lines[originalIdx])
			}
			newHunkLines = append(newHunkLines, patchLine.Content)
			originalIdx++
		case "-": // Remove line - should match original
			if originalIdx >= len(lines) {
				return nil, fmt.Errorf("remove line beyond end of file")
			}
			if lines[originalIdx] != patchLine.Content {
				return nil, fmt.Errorf("remove line mismatch at line %d: expected %q, got %q",
					originalIdx+1, patchLine.Content, lines[originalIdx])
			}
			// Don't add to newHunkLines (line is removed)
			originalIdx++
		case "+": // Add line
			newHunkLines = append(newHunkLines, patchLine.Content)
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
