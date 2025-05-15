package agents

import (
	"bytes"
	"context"
	"fmt"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	"text/template"
)

type ExplainAgent struct {
	// Model field removed
}

func (a *ExplainAgent) Name() string {
	return "ExplainAgent"
}

func (a *ExplainAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)
	prompt := fmt.Sprintf("Explain the purpose of the following %s code. Format the output as Markdown:\n\n```%s\n%s\n```", getFileExtension(path), getFileExtension(path), code)
	response, err := ollama.Query(appCfg.OllamaHostURL, modelName, prompt)
	return Result{File: path, Output: response, Agent: a.Name()}, err
}
