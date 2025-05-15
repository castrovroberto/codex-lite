package agents

import (
	"context"
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type CodeSmellAgent struct {
	// Model field removed
}

func (a *CodeSmellAgent) Name() string {
	return "CodeSmellAgent"
}

func (a *CodeSmellAgent) Analyze(ctx context.Context, modelName string, path string, code string) (Result, error) {
	appCfg := config.FromContext(ctx)
	// This is a basic prompt for detecting code smells.
	// It can be refined later for more specific types of smells.
	prompt := fmt.Sprintf(`Analyze the following %s code for potential code smells (e.g., long functions, duplicated code, complex logic, poor naming). Format the output as Markdown.
Describe any smells you find and suggest improvements. If possible, indicate the location (e.g., line number):\n\n%s`, getFileExtension(path), code)

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, prompt)
	if err != nil {
		return Result{}, err
	}

	return Result{
		File:   path,
		Output: response,
		Agent:  a.Name(),
	}, nil
}