// internal/agents/syntax_agent.go
package agents

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template" // Corrected from "html/template" if it's for plain text prompts

	// Assuming your project structure leads to these import paths
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SyntaxAgent struct {
	// Model field removed as modelName is passed to Analyze
}

func (a *SyntaxAgent) Name() string {
	return "SyntaxAgent"
}

// syntaxPromptTemplate defines the structure of the prompt for syntax checking.
// Using `+` for string concatenation to avoid issues with backticks inside backticks
// if the template were defined as a single raw string literal.
const syntaxPromptTemplate = `You are a syntax checking assistant.
Analyze the following {{.FileExtension}} code for syntax errors, potential issues, or suspicious lines.
Be precise and clearly indicate the line number for each issue you find.
If there are no errors, confirm that the syntax appears correct.
Format your entire output as Markdown.

Code to analyze (file type: {{.FileExtension}}):
` + "```{{.FileExtension}}\n{{.Code}}\n```"

// SyntaxPromptData holds the data to be injected into the syntax prompt template.
type SyntaxPromptData struct {
	FileExtension string
	Code          string
}

// getFileExtension extracts the file extension without the leading dot.
// e.g., "main.go" -> "go", ".bashrc" -> "bashrc"
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext != "" && len(ext) > 1 {
		return strings.TrimPrefix(ext, ".")
	}
	// Fallback for files without extension or dotfiles (e.g. "Makefile", ".bashrc")
	// For dotfiles like ".bashrc", base will be ".bashrc", then trim prefix.
	// For "Makefile", base will be "Makefile".
	// This part might need refinement based on how you want to treat extensionless files
	// or files starting with a dot but having no further extension part.
	// For simplicity, if `filepath.Ext` returns empty (e.g. "Makefile"),
	// or just a dot (which filepath.Ext doesn't do), we can return a generic placeholder or the filename itself.
	// For now, returning the "base" part if ext is empty.
	if ext == "" {
		// This might not be ideal for extensionless files, LLM might not know the language.
		// Consider passing "text" or making it mandatory for user to specify lang for such files.
		// For now, we'll just return "text" as a generic fallback.
		// A better approach for LLM would be to explicitly state if language is unknown.
		// Let's just return a generic "text" for simplicity, or let it be empty.
		// An empty FileExtension in the prompt might be better than a wrong one.
		// The prompt asks to "Check the following {{.FileExtension}} code"
		// If FileExtension is empty, it might be okay.
		// Or, we can assume it's the filename itself if no extension.
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") && len(base) > 1 { // like .gitignore
			return strings.TrimPrefix(base, ".")
		}
		// If it's like "Makefile", the extension is effectively the name itself for some contexts.
		// However, for "syntax checking {{.FileExtension}} code", this is ambiguous.
		// Defaulting to "text" or an empty string. Let's use an empty string.
		return "" // Or "text" as a generic placeholder if preferred
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

func (a *SyntaxAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx) // Retrieve app config from context

	// Ensure template is parsed only once if it's static, or cache it.
	// For simplicity here, parsing every time.
	// Consider using template.Must(template.New("syntax").Parse(syntaxPromptTemplate))
	// at package level if the template is fixed and errors are fatal.
	tmpl, err := template.New("syntaxPrompt").Parse(syntaxPromptTemplate)
	if err != nil {
		return Result{}, fmt.Errorf("SyntaxAgent: failed to parse syntax prompt template: %w", err)
	}

	fileExt := getFileExtension(path)
	if fileExt == "" {
		fileExt = "text" // Provide a default if extension is unknown/ambiguous
	}

	data := SyntaxPromptData{
		FileExtension: fileExt,
		Code:          code,
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, fmt.Errorf("SyntaxAgent: failed to execute syntax prompt template: %w", err)
	}

	// Call the updated ollama.Query with the host URL from config
	response, err := ollama.Query(appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		// It's good practice to wrap errors for context
		return Result{}, fmt.Errorf("SyntaxAgent: error from Ollama: %w", err)
	}

	return Result{
		File:   path,
		Output: strings.TrimSpace(response), // Trim whitespace from Ollama's response
		Agent:  a.Name(),
	}, nil
}