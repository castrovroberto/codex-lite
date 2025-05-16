package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// DefaultIgnoreDirs is a set of directory names that are commonly ignored
var DefaultIgnoreDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".svn":         true,
	".hg":          true,
	".bzr":         true,
	"target":       true,
}

// DefaultSourceExtensions is a set of file extensions that are commonly considered source code
var DefaultSourceExtensions = map[string]bool{
	".go":     true,
	".py":     true,
	".js":     true,
	".jsx":    true,
	".ts":     true,
	".tsx":    true,
	".java":   true,
	".cpp":    true,
	".c":      true,
	".h":      true,
	".hpp":    true,
	".rs":     true,
	".rb":     true,
	".php":    true,
	".cs":     true,
	".swift":  true,
	".kt":     true,
	".scala":  true,
	".sh":     true,
	".bash":   true,
	".zsh":    true,
	".fish":   true,
	".sql":    true,
	".r":      true,
	".dart":   true,
	".lua":    true,
	".pl":     true,
	".pm":     true,
	".t":      true,
	".rust":   true,
	".vue":    true,
	".svelte": true,
}

// Options configures the behavior of the scanner
type Options struct {
	// IgnoreDirs is a set of directory names to ignore
	IgnoreDirs map[string]bool
	// SourceExtensions is a set of file extensions to include
	SourceExtensions map[string]bool
	// CustomIgnorePatterns is a list of glob patterns to ignore
	CustomIgnorePatterns []string
	// MaxDepth is the maximum directory depth to scan (-1 for unlimited)
	MaxDepth int
}

// DefaultOptions returns the default scanner options
func DefaultOptions() *Options {
	return &Options{
		IgnoreDirs:       DefaultIgnoreDirs,
		SourceExtensions: DefaultSourceExtensions,
		MaxDepth:         -1,
	}
}

// Scanner provides functionality for recursively scanning a codebase
type Scanner struct {
	opts *Options
}

// NewScanner creates a new Scanner with the given options
func NewScanner(opts *Options) *Scanner {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Scanner{opts: opts}
}

// ScanResult represents a file found during scanning
type ScanResult struct {
	Path string
	Info fs.FileInfo
}

// Scan recursively scans the given root directory and returns a list of files
// that match the scanner's criteria
func (s *Scanner) Scan(root string) ([]ScanResult, error) {
	var results []ScanResult

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Check directory depth if MaxDepth is set
		if s.opts.MaxDepth >= 0 {
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth > s.opts.MaxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip ignored directories
		if d.IsDir() {
			if s.opts.IgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check custom ignore patterns
		for _, pattern := range s.opts.CustomIgnorePatterns {
			matched, err := filepath.Match(pattern, relPath)
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if !s.opts.SourceExtensions[ext] {
			return nil
		}

		// Get file info for additional metadata
		info, err := d.Info()
		if err != nil {
			return err
		}

		results = append(results, ScanResult{
			Path: path,
			Info: info,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// IsSourceFile checks if a file has a recognized source code extension
func (s *Scanner) IsSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return s.opts.SourceExtensions[ext]
}

// ShouldIgnoreDir checks if a directory should be ignored
func (s *Scanner) ShouldIgnoreDir(name string) bool {
	return s.opts.IgnoreDirs[name]
}
