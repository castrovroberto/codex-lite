package agents

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SyntaxAgent struct {
	// Model field removed
}

func (a *SyntaxAgent) Name() string {
	return "SyntaxAgent"
}

const syntaxPromptTemplate = `Check the following {{.FileExtension}} code for syntax errors. Format the output as Markdown.
If you find any issues or suspicious lines, please describe them and, if possible,
indicate their location (e.g., line number):

` + "```{{.FileExtension}}\n{{.Code}}\n```" + `

type SyntaxPromptData struct {
	FileExtension string
	Code          string
}

func (a *SyntaxAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)

	tmpl, err := template.New("syntax").Parse(syntaxPromptTemplate)
	if err != nil {
		return Result{}, fmt.Errorf("failed to parse syntax prompt template: %w", err)
	}
	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, SyntaxPromptData{FileExtension: getFileExtension(path), Code: code}); err != nil {
		return Result{}, fmt.Errorf("failed to execute syntax prompt template: %w", err)
	}

	response, err := ollama.Query(appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, err
	}

	return Result{
		File:   path,
		Output: response,
		Agent:  a.Name(),
	}, nil
}