// internal/agents/utils.go
package agents

import (
	"path/filepath"
	"strings"
)

// getFileExtension extracts the file extension without the leading dot.
// e.g., "main.go" -> "go", ".bashrc" -> "bashrc"
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	base := filepath.Base(path)

	if ext != "" {
		// Standard extension like .go, .txt
		return strings.ToLower(strings.TrimPrefix(ext, "."))
	}

	// Handle files without extensions or dotfiles (e.g. "Makefile", ".bashrc")
	// If the filename starts with a dot and has content after it (like .gitignore),
	// treat the part after the dot as the "extension" for our purposes.
	if strings.HasPrefix(base, ".") && len(base) > 1 {
		return strings.ToLower(strings.TrimPrefix(base, "."))
	}

	// For files like "Makefile" where there's no dot, or if it's just "." (unlikely)
	// We might return the filename itself, "text", or an empty string.
	// For LLM prompts, providing something like "text" or the filename can be useful.
	// Let's return "text" if no other specific type can be determined.
	// Or, if you prefer the LLM to infer, return an empty string or the base itself.
	// For now, returning "text" for ambiguous cases.
	if base != "" && !strings.Contains(base, ".") { // e.g. "Makefile"
	    // return strings.ToLower(base) // Option: return "makefile"
	    return "text" // Option: return "text"
	}

	return "text" // Default fallback
}