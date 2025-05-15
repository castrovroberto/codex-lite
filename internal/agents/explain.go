package agents

import "github.com/castrovroberto/codex-lite/internal/ollama"

type ExplainAgent struct {
    Model string
}

func (a *ExplainAgent) Name() string {
    return "ExplainAgent"
}

func (a *ExplainAgent) Analyze(path string, code string) (Result, error) {
    prompt := "Explain the purpose of the following code:\n\n" + code
    response, err := ollama.Query(a.Model, prompt)
    return Result{
        File:   path,
        Output: response,
        Agent:  a.Name(),
    }, err
}
