// internal/agents/syntax_agent.go
package agents

import (
	"bytes"
	"context"
	"fmt"
	// "path/filepath" // No longer needed here if getFileExtension is in utils.go
	"strings"       // No longer needed here if getFileExtension is in utils.go
	"text/template"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SyntaxAgent struct{}

func (a *SyntaxAgent) Name() string {
	return "SyntaxAgent"
}

const syntaxPromptTemplate = `You are a syntax checking assistant.
Analyze the following {{.FileExtension}} code for syntax errors, potential issues, or suspicious lines.
Be precise and clearly indicate the line number for each issue you find.
If there are no errors, confirm that the syntax appears correct.
Format your entire output as Markdown.

Code to analyze (file type: {{.FileExtension}}):
` + "```{{.FileExtension}}\n{{.Code}}\n```"

type SyntaxPromptData struct {
	FileExtension string
	Code          string
}

func (a *SyntaxAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)

	tmpl, err := template.New("syntaxPrompt").Parse(syntaxPromptTemplate)
	if err != nil {
		return Result{}, fmt.Errorf("SyntaxAgent: failed to parse syntax prompt template: %w", err)
	}

	fileExt := getFileExtension(path) // Uses the function from utils.go
	if fileExt == "" {                 // Or whatever default you set in getFileExtension
		fileExt = "text" // Fallback if getFileExtension could return empty
	}

	data := SyntaxPromptData{
		FileExtension: fileExt,
		Code:          code,
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, fmt.Errorf("SyntaxAgent: failed to execute syntax prompt template: %w", err)
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, fmt.Errorf("SyntaxAgent: error from Ollama: %w", err)
	}

	return Result{
		File:   path,
		Output: strings.TrimSpace(response),
		Agent:  a.Name(),
	}, nil
}