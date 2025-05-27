package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeFileOps provides secure file operations that prevent path traversal attacks
type SafeFileOps struct {
	allowedRoots []string
}

// NewSafeFileOps creates a new SafeFileOps instance with allowed root directories
func NewSafeFileOps(allowedRoots ...string) *SafeFileOps {
	// Clean and make absolute all allowed roots
	cleanRoots := make([]string, len(allowedRoots))
	for i, root := range allowedRoots {
		cleanRoot, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			// If we can't make it absolute, use the cleaned version
			cleanRoot = filepath.Clean(root)
		}
		cleanRoots[i] = cleanRoot
	}

	return &SafeFileOps{
		allowedRoots: cleanRoots,
	}
}

// ValidatePath checks if a path is safe and within allowed roots
func (sfo *SafeFileOps) ValidatePath(path string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Make it absolute if it's not already
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if the path is within any of the allowed roots
	for _, root := range sfo.allowedRoots {
		// Check if the absolute path is within this root
		relPath, err := filepath.Rel(root, absPath)
		if err != nil {
			continue // Try next root
		}

		// If the relative path doesn't start with "..", it's within the root
		if !strings.HasPrefix(relPath, "..") && !strings.HasPrefix(relPath, "/") {
			return nil // Path is safe
		}
	}

	return fmt.Errorf("path %q is outside allowed directories", path)
}

// SafeReadFile reads a file after validating the path is safe
func (sfo *SafeFileOps) SafeReadFile(path string) ([]byte, error) {
	if err := sfo.ValidatePath(path); err != nil {
		return nil, fmt.Errorf("unsafe path: %w", err)
	}

	// #nosec G304 - Path has been validated above
	return os.ReadFile(path)
}

// SafeWriteFile writes a file after validating the path is safe
func (sfo *SafeFileOps) SafeWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := sfo.ValidatePath(path); err != nil {
		return fmt.Errorf("unsafe path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// #nosec G304 - Path has been validated above
	return os.WriteFile(path, data, perm)
}

// SafeCreate creates a file after validating the path is safe
func (sfo *SafeFileOps) SafeCreate(path string) (*os.File, error) {
	if err := sfo.ValidatePath(path); err != nil {
		return nil, fmt.Errorf("unsafe path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// #nosec G304 - Path has been validated above
	return os.Create(path)
}

// SafeRemove removes a file after validating the path is safe
func (sfo *SafeFileOps) SafeRemove(path string) error {
	if err := sfo.ValidatePath(path); err != nil {
		return fmt.Errorf("unsafe path: %w", err)
	}

	return os.Remove(path)
}

// SafeStat gets file info after validating the path is safe
func (sfo *SafeFileOps) SafeStat(path string) (os.FileInfo, error) {
	if err := sfo.ValidatePath(path); err != nil {
		return nil, fmt.Errorf("unsafe path: %w", err)
	}

	return os.Stat(path)
}

// JoinPath safely joins path components and validates the result
func (sfo *SafeFileOps) JoinPath(base string, elem ...string) (string, error) {
	path := filepath.Join(base, filepath.Join(elem...))
	if err := sfo.ValidatePath(path); err != nil {
		return "", err
	}
	return path, nil
}
