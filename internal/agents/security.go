package agents

import (
	"context"
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SecurityAgent struct {
	// Model field removed
}

func (a *SecurityAgent) Name() string {
	return "SecurityAgent"
}

func (a *SecurityAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)
	// This is a basic prompt for identifying potential security vulnerabilities.
	// It can be refined later for specific types of vulnerabilities or languages.
	prompt := fmt.Sprintf(`Review the following %s code for potential security vulnerabilities (e.g., injection flaws, insecure handling of sensitive data, broken access control, weak cryptography). Format the output as Markdown.
Describe any vulnerabilities you find and suggest how to fix them. If possible, indicate the location (e.g., line number):\n\n%s`, getFileExtension(path), code)

	response, err := ollama.Query(appCfg.OllamaHostURL, modelName, prompt)
	if err != nil {
		return Result{}, err
	}

	return Result{
		File:   path,
		Output: response,
		Agent:  a.Name(),
	}, nil
}