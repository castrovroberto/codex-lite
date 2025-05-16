package analyzer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Common directories to skip during analysis
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".venv":        true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
}

// Default source file extensions to analyze
var defaultSourceExts = map[string]bool{
	".go":    true,
	".js":    true,
	".ts":    true,
	".jsx":   true,
	".tsx":   true,
	".py":    true,
	".java":  true,
	".cpp":   true,
	".c":     true,
	".h":     true,
	".hpp":   true,
	".rs":    true,
	".rb":    true,
	".php":   true,
	".swift": true,
}

// CodebaseInfo holds information about the analyzed codebase
type CodebaseInfo struct {
	RootPath    string         // Absolute path to the codebase root
	IsGitRepo   bool           // Whether the codebase is a Git repository
	FileCount   int            // Total number of source files
	TotalLines  int            // Total lines of code
	FilesByType map[string]int // Count of files by extension
	LinesByType map[string]int // Count of lines by file extension
	SkippedDirs []string       // List of skipped directories
	Errors      []string       // Any errors encountered during analysis
}

// AnalyzeCodebase performs a recursive analysis of the codebase
func AnalyzeCodebase(rootPath string, customExts []string) (*CodebaseInfo, error) {
	// Convert rootPath to absolute path
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Initialize codebase info
	info := &CodebaseInfo{
		RootPath:    absPath,
		FilesByType: make(map[string]int),
		LinesByType: make(map[string]int),
	}

	// Check if it's a Git repository
	if _, err := os.Stat(filepath.Join(absPath, ".git")); err == nil {
		info.IsGitRepo = true
	}

	// Build extension map
	exts := make(map[string]bool)
	for ext := range defaultSourceExts {
		exts[ext] = true
	}
	for _, ext := range customExts {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts[ext] = true
	}

	// Walk the directory tree
	err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			info.Errors = append(info.Errors, fmt.Sprintf("Error accessing %s: %v", path, err))
			return fs.SkipDir
		}

		// Skip if directory and in skipDirs
		if d.IsDir() {
			if skipDirs[d.Name()] {
				info.SkippedDirs = append(info.SkippedDirs, path)
				return fs.SkipDir
			}
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if !exts[ext] {
			return nil
		}

		// Count file
		info.FileCount++
		info.FilesByType[ext]++

		// Count lines
		content, err := os.ReadFile(path)
		if err != nil {
			info.Errors = append(info.Errors, fmt.Sprintf("Error reading %s: %v", path, err))
			return nil
		}

		lines := strings.Count(string(content), "\n") + 1
		info.TotalLines += lines
		info.LinesByType[ext] += lines

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return info, nil
}

// FormatAnalysis returns a human-readable summary of the codebase analysis
func (ci *CodebaseInfo) FormatAnalysis() string {
	var b strings.Builder

	// Repository status
	if ci.IsGitRepo {
		b.WriteString("✓ Git repository detected\n")
	} else {
		b.WriteString("⚠ Not a Git repository - version control recommended\n")
	}
	b.WriteString("\n")

	// Basic stats
	b.WriteString(fmt.Sprintf("📊 Codebase Statistics:\n"))
	b.WriteString(fmt.Sprintf("- Total source files: %d\n", ci.FileCount))
	b.WriteString(fmt.Sprintf("- Total lines of code: %d\n", ci.TotalLines))
	b.WriteString("\n")

	// Files by type
	b.WriteString("📝 Files by type:\n")
	for ext, count := range ci.FilesByType {
		lines := ci.LinesByType[ext]
		avgLines := float64(lines) / float64(count)
		b.WriteString(fmt.Sprintf("- %s: %d files (%d lines, avg %.1f lines/file)\n",
			ext, count, lines, avgLines))
	}
	b.WriteString("\n")

	// Skipped directories
	if len(ci.SkippedDirs) > 0 {
		b.WriteString("⏭️ Skipped directories:\n")
		for _, dir := range ci.SkippedDirs {
			b.WriteString(fmt.Sprintf("- %s\n", dir))
		}
		b.WriteString("\n")
	}

	// Errors
	if len(ci.Errors) > 0 {
		b.WriteString("⚠️ Analysis warnings:\n")
		for _, err := range ci.Errors {
			b.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return b.String()
}

// IsSkippableDir checks if a directory should be skipped during analysis
func IsSkippableDir(name string) bool {
	return skipDirs[name]
}

// IsSourceFile checks if a file extension is recognized as a source file
func IsSourceFile(ext string) bool {
	return defaultSourceExts[ext]
}
