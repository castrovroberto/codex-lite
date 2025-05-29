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

// ListDirToolConfig contains configuration for the list directory tool
type ListDirToolConfig struct {
	// AllowOutsideWorkspace allows listing directories outside the workspace root
	AllowOutsideWorkspace bool
	// AllowedRoots specifies additional allowed root directories for listing
	AllowedRoots []string
	// MaxDepthLimit sets the maximum allowed recursion depth
	MaxDepthLimit int
	// MaxFilesLimit sets the maximum number of files to return
	MaxFilesLimit int
	// AutoResolveSymlinks determines whether to follow symbolic links
	AutoResolveSymlinks bool
	// SmartPathResolution enables intelligent path resolution
	SmartPathResolution bool
}

// DefaultListDirToolConfig returns default configuration with security-first settings
func DefaultListDirToolConfig() ListDirToolConfig {
	return ListDirToolConfig{
		AllowOutsideWorkspace: false,
		AllowedRoots:          []string{},
		MaxDepthLimit:         10,
		MaxFilesLimit:         1000,
		AutoResolveSymlinks:   false,
		SmartPathResolution:   true,
	}
}

// ListDirTool implements directory listing capabilities
type ListDirTool struct {
	workspaceRoot string
	config        ListDirToolConfig
}

// NewListDirTool creates a new list directory tool with default configuration
func NewListDirTool(workspaceRoot string) *ListDirTool {
	return &ListDirTool{
		workspaceRoot: workspaceRoot,
		config:        DefaultListDirToolConfig(),
	}
}

// NewListDirToolWithConfig creates a new list directory tool with custom configuration
func NewListDirToolWithConfig(workspaceRoot string, config ListDirToolConfig) *ListDirTool {
	return &ListDirTool{
		workspaceRoot: workspaceRoot,
		config:        config,
	}
}

func (t *ListDirTool) Name() string {
	return "list_directory"
}

func (t *ListDirTool) Description() string {
	return "Lists files and subdirectories within a specified directory with enhanced path resolution and security controls."
}

func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"directory_path": {
				"type": "string",
				"description": "The path to the directory to list. Can be relative to workspace root, absolute (if allowed), or use special notation like '~' for home directory"
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
			},
			"pattern": {
				"type": "string",
				"description": "Optional glob pattern to filter files (e.g., '*.go' for Go files)"
			},
			"sort_by": {
				"type": "string",
				"description": "Sort criterion: 'name', 'size', 'modified', 'type'",
				"default": "type_name"
			},
			"smart_resolve": {
				"type": "boolean",
				"description": "Enable smart path resolution to handle common path variations",
				"default": true
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
	Pattern       string `json:"pattern"`
	SortBy        string `json:"sort_by"`
	SmartResolve  bool   `json:"smart_resolve"`
}

type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	AbsolutePath string    `json:"absolute_path,omitempty"`
	IsDirectory  bool      `json:"is_directory"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Permissions  string    `json:"permissions"`
	IsSourceFile bool      `json:"is_source_file,omitempty"`
	IsSymlink    bool      `json:"is_symlink,omitempty"`
	LinkTarget   string    `json:"link_target,omitempty"`
}

// PathResolutionResult contains the result of path resolution
type PathResolutionResult struct {
	OriginalPath  string `json:"original_path"`
	ResolvedPath  string `json:"resolved_path"`
	IsAbsolute    bool   `json:"is_absolute"`
	IsInWorkspace bool   `json:"is_in_workspace"`
	AllowedByRule string `json:"allowed_by_rule,omitempty"`
}

func (t *ListDirTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p ListDirParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	// Set defaults
	if p.MaxDepth == 0 {
		p.MaxDepth = 3
	}
	if p.SortBy == "" {
		p.SortBy = "type_name"
	}

	// Apply configuration limits
	if p.MaxDepth > t.config.MaxDepthLimit {
		p.MaxDepth = t.config.MaxDepthLimit
	}

	// Resolve and validate the directory path
	pathResult, err := t.resolvePath(p.DirectoryPath, p.SmartResolve && t.config.SmartPathResolution)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path resolution failed: %v", err),
		}, nil
	}

	// Validate access permissions
	if err := t.validateAccess(pathResult); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("access denied: %v", err),
		}, nil
	}

	// Check if directory exists and is accessible
	info, err := os.Stat(pathResult.ResolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try smart resolution for common directory names
			if p.SmartResolve && t.config.SmartPathResolution {
				if resolved, resolveErr := t.trySmartResolution(p.DirectoryPath); resolveErr == nil {
					pathResult.ResolvedPath = resolved
					info, err = os.Stat(resolved)
				}
			}
		}

		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to access directory: %v", err),
			}, nil
		}
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
		err = t.walkDirectory(pathResult.ResolvedPath, 0, p.MaxDepth, p.IncludeHidden, p.Pattern, &files, &totalFiles, &totalDirs)
	} else {
		err = t.listDirectory(pathResult.ResolvedPath, p.IncludeHidden, p.Pattern, &files, &totalFiles, &totalDirs)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list directory: %v", err),
		}, nil
	}

	// Apply file limit
	if len(files) > t.config.MaxFilesLimit {
		files = files[:t.config.MaxFilesLimit]
	}

	// Sort files based on the specified criterion
	t.sortFiles(files, p.SortBy)

	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"directory_path":  p.DirectoryPath,
			"resolved_path":   pathResult.ResolvedPath,
			"path_resolution": pathResult,
			"files":           files,
			"total_files":     totalFiles,
			"total_dirs":      totalDirs,
			"recursive":       p.Recursive,
			"pattern":         p.Pattern,
			"truncated":       len(files) >= t.config.MaxFilesLimit,
			"message":         fmt.Sprintf("Listed %d files and %d directories in %s", totalFiles, totalDirs, pathResult.ResolvedPath),
		},
	}, nil
}

// resolvePath resolves the directory path using various strategies
func (t *ListDirTool) resolvePath(dirPath string, smartResolve bool) (*PathResolutionResult, error) {
	result := &PathResolutionResult{
		OriginalPath: dirPath,
	}

	// Handle special cases
	if dirPath == "." || dirPath == "" {
		result.ResolvedPath = t.workspaceRoot
		result.IsInWorkspace = true
		result.AllowedByRule = "workspace_root"
		return result, nil
	}

	// Handle home directory expansion
	if strings.HasPrefix(dirPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot resolve home directory: %v", err)
		}
		if dirPath == "~" {
			dirPath = homeDir
		} else {
			dirPath = filepath.Join(homeDir, dirPath[2:])
		}
	}

	// Determine if path is absolute
	result.IsAbsolute = filepath.IsAbs(dirPath)

	// Resolve relative paths
	if !result.IsAbsolute {
		result.ResolvedPath = filepath.Join(t.workspaceRoot, dirPath)
	} else {
		result.ResolvedPath = dirPath
	}

	// Clean the path
	result.ResolvedPath = filepath.Clean(result.ResolvedPath)

	// Check if within workspace - fixed to prevent false positives
	cleanWorkspace := filepath.Clean(t.workspaceRoot)
	cleanResolved := result.ResolvedPath

	// A path is within the workspace if:
	// 1. It's exactly the workspace root, OR
	// 2. It starts with the workspace root followed by a path separator
	if cleanResolved == cleanWorkspace {
		result.IsInWorkspace = true
	} else {
		result.IsInWorkspace = strings.HasPrefix(cleanResolved, cleanWorkspace+string(filepath.Separator))
	}

	return result, nil
}

// validateAccess checks if access to the resolved path is allowed
func (t *ListDirTool) validateAccess(pathResult *PathResolutionResult) error {
	// Always allow workspace paths
	if pathResult.IsInWorkspace {
		pathResult.AllowedByRule = "within_workspace"
		return nil
	}

	// If outside workspace access is not allowed, deny
	if !t.config.AllowOutsideWorkspace {
		return fmt.Errorf("directory path is outside workspace root and outside access is disabled")
	}

	// Check against allowed roots
	for _, allowedRoot := range t.config.AllowedRoots {
		cleanAllowedRoot := filepath.Clean(allowedRoot)
		if strings.HasPrefix(pathResult.ResolvedPath, cleanAllowedRoot) {
			pathResult.AllowedByRule = fmt.Sprintf("allowed_root: %s", allowedRoot)
			return nil
		}
	}

	// If we reach here, outside access is allowed but path is not in allowed roots
	pathResult.AllowedByRule = "outside_access_enabled"
	return nil
}

// trySmartResolution attempts to resolve common directory patterns
func (t *ListDirTool) trySmartResolution(dirPath string) (string, error) {
	commonDirs := map[string][]string{
		"docs":    {"docs", "documentation", "doc"},
		"src":     {"src", "source", "lib"},
		"test":    {"test", "tests", "__tests__", "spec"},
		"config":  {"config", "configs", "configuration", "settings"},
		"scripts": {"scripts", "script", "bin"},
		"build":   {"build", "dist", "target", "out"},
	}

	lowerPath := strings.ToLower(dirPath)

	if alternatives, exists := commonDirs[lowerPath]; exists {
		for _, alt := range alternatives {
			testPath := filepath.Join(t.workspaceRoot, alt)
			if info, err := os.Stat(testPath); err == nil && info.IsDir() {
				return testPath, nil
			}
		}
	}

	return "", fmt.Errorf("no smart resolution found for %s", dirPath)
}

func (t *ListDirTool) listDirectory(dirPath string, includeHidden bool, pattern string, files *[]FileInfo, totalFiles, totalDirs *int) error {
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

		// Apply pattern filter if specified
		if pattern != "" && !t.matchesPattern(name, pattern) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		fullPath := filepath.Join(dirPath, name)
		relPath, err := filepath.Rel(t.workspaceRoot, fullPath)
		if err != nil {
			relPath = name
		}

		fileInfo := FileInfo{
			Name:         name,
			Path:         relPath,
			AbsolutePath: fullPath,
			IsDirectory:  entry.IsDir(),
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			Permissions:  info.Mode().String(),
		}

		// Check for symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			fileInfo.IsSymlink = true
			if target, err := os.Readlink(fullPath); err == nil {
				fileInfo.LinkTarget = target
			}
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

func (t *ListDirTool) walkDirectory(dirPath string, currentDepth, maxDepth int, includeHidden bool, pattern string, files *[]FileInfo, totalFiles, totalDirs *int) error {
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

		// Apply pattern filter if specified
		if pattern != "" && !t.matchesPattern(name, pattern) {
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
			Name:         name,
			Path:         relPath,
			AbsolutePath: path,
			IsDirectory:  d.IsDir(),
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			Permissions:  info.Mode().String(),
		}

		// Check for symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			fileInfo.IsSymlink = true
			if target, err := os.Readlink(path); err == nil {
				fileInfo.LinkTarget = target
			}
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

// matchesPattern checks if a filename matches the given glob pattern
func (t *ListDirTool) matchesPattern(filename, pattern string) bool {
	if pattern == "" {
		return true
	}

	matched, err := filepath.Match(pattern, filename)
	if err != nil {
		// If pattern is invalid, fall back to simple substring matching
		return strings.Contains(strings.ToLower(filename), strings.ToLower(pattern))
	}
	return matched
}

func (t *ListDirTool) sortFiles(files []FileInfo, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
	case "size":
		sort.Slice(files, func(i, j int) bool {
			return files[i].Size < files[j].Size
		})
	case "modified":
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.Before(files[j].ModTime)
		})
	case "type":
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDirectory != files[j].IsDirectory {
				return files[i].IsDirectory
			}
			return files[i].Name < files[j].Name
		})
	default: // "type_name" or default
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDirectory != files[j].IsDirectory {
				return files[i].IsDirectory
			}
			return files[i].Name < files[j].Name
		})
	}
}
