package agents

import "path/filepath"

// getFileExtension extracts the file extension from a filename.
// It uses the path/filepath package for more robust handling.
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" && len(ext) > 1 { // Ensure it's not just "." and has characters after "."
		return ext[1:] // Remove the leading dot
	}
	return "" // No extension found or only a dot
}