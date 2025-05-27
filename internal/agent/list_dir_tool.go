package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/analyzer"
)

// ListDirTool implements directory listing capabilities
type ListDirTool struct {
	workspaceRoot string
}

// NewListDirTool creates a new list directory tool
func NewListDirTool(workspaceRoot string) *ListDirTool {
	return &ListDirTool{
		workspaceRoot: workspaceRoot,
	}
}

func (t *ListDirTool) Name() string {
	return "list_directory"
}

func (t *ListDirTool) Description() string {
	return "Lists files and subdirectories within a specified directory."
}

func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"directory_path": {
				"type": "string",
				"description": "The path to the directory to list (relative to workspace root, or '.' for workspace root)"
			},
			"recursive": {
				"type": "boolean",
				"description": "Whether to list files recursively in subdirectories",
				"default": false
			},
			"include_hidden": {
				"type": "boolean",
				"description": "Whether to include hidden files and directories",
				"default": false
			},
			"max_depth": {
				"type": "integer",
				"description": "Maximum depth for recursive listing (ignored if recursive is false)",
				"default": 3
			}
		},
		"required": ["directory_path"]
	}`)
}

type ListDirParams struct {
	DirectoryPath string `json:"directory_path"`
	Recursive     bool   `json:"recursive"`
	IncludeHidden bool   `json:"include_hidden"`
	MaxDepth      int    `json:"max_depth"`
}

type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsDirectory  bool      `json:"is_directory"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Permissions  string    `json:"permissions"`
	IsSourceFile bool      `json:"is_source_file,omitempty"`
}

func (t *ListDirTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p ListDirParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Default values
	if p.MaxDepth == 0 {
		p.MaxDepth = 3
	}

	// Handle special case for workspace root
	if p.DirectoryPath == "." || p.DirectoryPath == "" {
		p.DirectoryPath = "."
	}

	// Build full path and validate
	var fullPath string
	if p.DirectoryPath == "." {
		fullPath = t.workspaceRoot
	} else {
		fullPath = filepath.Join(t.workspaceRoot, p.DirectoryPath)
	}

	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(t.workspaceRoot)) {
		return &ToolResult{
			Success: false,
			Error:   "directory path is outside workspace root",
		}, nil
	}

	// Check if directory exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access directory: %v", err),
		}, nil
	}

	if !info.IsDir() {
		return &ToolResult{
			Success: false,
			Error:   "path is not a directory",
		}, nil
	}

	var files []FileInfo
	var totalFiles, totalDirs int

	if p.Recursive {
		err = t.walkDirectory(cleanPath, 0, p.MaxDepth, p.IncludeHidden, &files, &totalFiles, &totalDirs)
	} else {
		err = t.listDirectory(cleanPath, p.IncludeHidden, &files, &totalFiles, &totalDirs)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list directory: %v", err),
		}, nil
	}

	// Sort files: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDirectory != files[j].IsDirectory {
			return files[i].IsDirectory
		}
		return files[i].Name < files[j].Name
	})

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"directory_path": p.DirectoryPath,
			"files":          files,
			"total_files":    totalFiles,
			"total_dirs":     totalDirs,
			"recursive":      p.Recursive,
			"message":        fmt.Sprintf("Listed %d files and %d directories in %s", totalFiles, totalDirs, p.DirectoryPath),
		},
	}, nil
}

func (t *ListDirTool) listDirectory(dirPath string, includeHidden bool, files *[]FileInfo, totalFiles, totalDirs *int) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if not requested
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip common uninteresting directories
		if entry.IsDir() && analyzer.IsSkippableDir(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		relPath, err := filepath.Rel(t.workspaceRoot, filepath.Join(dirPath, name))
		if err != nil {
			relPath = name
		}

		fileInfo := FileInfo{
			Name:        name,
			Path:        relPath,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode().String(),
		}

		if !entry.IsDir() {
			fileInfo.IsSourceFile = analyzer.IsSourceFile(strings.ToLower(filepath.Ext(name)))
			*totalFiles++
		} else {
			*totalDirs++
		}

		*files = append(*files, fileInfo)
	}

	return nil
}

func (t *ListDirTool) walkDirectory(dirPath string, currentDepth, maxDepth int, includeHidden bool, files *[]FileInfo, totalFiles, totalDirs *int) error {
	if currentDepth > maxDepth {
		return nil
	}

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files/dirs we can't access
		}

		// Calculate depth relative to starting directory
		relToStart, err := filepath.Rel(dirPath, path)
		if err != nil {
			return nil
		}

		depth := strings.Count(relToStart, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()

		// Skip hidden files if not requested
		if !includeHidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common uninteresting directories
		if d.IsDir() && analyzer.IsSkippableDir(name) {
			return filepath.SkipDir
		}

		// Skip the root directory itself
		if path == dirPath {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		relPath, err := filepath.Rel(t.workspaceRoot, path)
		if err != nil {
			relPath = name
		}

		fileInfo := FileInfo{
			Name:        name,
			Path:        relPath,
			IsDirectory: d.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode().String(),
		}

		if !d.IsDir() {
			fileInfo.IsSourceFile = analyzer.IsSourceFile(strings.ToLower(filepath.Ext(name)))
			*totalFiles++
		} else {
			*totalDirs++
		}

		*files = append(*files, fileInfo)
		return nil
	})

	return err
}
