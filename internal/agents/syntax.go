package agents

import (
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SyntaxAgent struct {
	Model string
}

func (a *SyntaxAgent) Name() string {
	return "SyntaxAgent"
}

func (a *SyntaxAgent) Analyze(path string, code string) (Result, error) {
	// In a more sophisticated implementation, we might use multiple prompts
	// and combine their results.  For now, we'll keep it simple.
	prompt := fmt.Sprintf(`Check the following %s code for syntax errors.
If you find any issues or suspicious lines, please describe them and, if possible,
indicate their location (e.g., line number):\n\n%s`, getFileExtension(path), code)

	response, err := ollama.Query(a.Model, prompt)
	if err != nil {
		return Result{}, err
	}

	return Result{
		File:   path,
		Output: response,
		Agent:  a.Name(),
	}, nil
}

// Helper function to get the file extension (for better prompting)
func getFileExtension(filename string) string {
	// This is a basic implementation.  For more robust extension handling,
	// you might want to use the "path/filepath" package.
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i+1:]
		}
	}
	return "" // No extension found
}