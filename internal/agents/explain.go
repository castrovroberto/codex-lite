// internal/agents/explain_agent.go
package agents

import (
	"bytes"
	"context"
	"fmt"
	"strings"       // For strings.TrimSpace
	"text/template"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type ExplainAgent struct{}

func (a *ExplainAgent) Name() string {
	return "ExplainAgent"
}

const explainPromptTemplate = `You are a code explanation assistant.
Explain the purpose, functionality, and key components of the following {{.FileExtension}} code.
If there are any complex parts, try to simplify them.
Format your entire output as Markdown.

Code to analyze (file type: {{.FileExtension}}):
` + "```{{.FileExtension}}\n{{.Code}}\n```"

type ExplainPromptData struct {
	FileExtension string
	Code          string
}

func (a *ExplainAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)

	tmpl, err := template.New("explainPrompt").Parse(explainPromptTemplate)
	if err != nil {
		return Result{}, fmt.Errorf("ExplainAgent: failed to parse prompt template: %w", err)
	}

	fileExt := getFileExtension(path) // Uses the function from utils.go
	if fileExt == "" {
		fileExt = "text" // Fallback
	}

	data := ExplainPromptData{
		FileExtension: fileExt,
		Code:          code,
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, fmt.Errorf("ExplainAgent: failed to execute prompt template: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, appCfg.OllamaRequestTimeout)
	defer cancel() // Ensure resources are released even if the function returns early.
	response, err := ollama.Query(ctxWithTimeout, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, fmt.Errorf("ExplainAgent: error from Ollama: %w", err)
	}

	return Result{
		File:   path,
		Output: strings.TrimSpace(response),
		Agent:  a.Name(),
	}, nil
}