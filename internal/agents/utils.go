// internal/agents/utils.go
package agents

import (
	"path/filepath"
	"strings"
)

// getFileExtension extracts the file extension without the leading dot.
// e.g., "main.go" -> "go", ".bashrc" -> "bashrc"
func getFileExtension(path string) string {
	ext := filepath.Ext(path) // Example: ".go"
	base := filepath.Base(path) // Example: "main.go" or ".bashrc"

	if ext != "" && ext != "." { // Standard extension like .go, .txt
		return strings.ToLower(strings.TrimPrefix(ext, "."))
	}

	// Handle files without extensions or "true" dotfiles (e.g. ".bashrc" but not ".travis.yml")
	// If the filename starts with a dot and has content after it,
	// and contains no other dots, treat the part after the initial dot as the "extension".
	if strings.HasPrefix(base, ".") && len(base) > 1 && !strings.Contains(base[1:], ".") {
		return strings.ToLower(strings.TrimPrefix(base, "."))
	}

	// For files like "Makefile" where there's no dot, or more complex dotfiles like "my.file.conf"
	// where filepath.Ext would return ".conf".
	// If after all this, ext is empty (e.g., "Makefile"), provide a default or the name itself.
	// Let's default to "text" for ambiguous cases where a clear extension isn't found.
	if ext == "" && base != "" && !strings.Contains(base, ".") { // e.g. "Makefile"
		return "text" // Or potentially strings.ToLower(base) if you want "makefile"
	}

	// If ext was just "." or other unhandled cases, fallback.
	if ext == "." || ext == "" {
		return "text"
	}

	// Should have been caught by the first if, but as a fallback.
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// I've added a local min function to each agent file for simplicity.
// If you have a shared utils package where getFileExtension lives,
// you could add min there too.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}