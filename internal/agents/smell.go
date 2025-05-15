package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type SmellAgent struct{}

func NewSmellAgent() *SmellAgent { return &SmellAgent{} }
func (a *SmellAgent) Name() string { return "Code Smell Detector" }
func (a *SmellAgent) Description() string {
	return "Identifies code smells (e.g., long methods, duplicated code) and suggests improvements."
}

const smellPromptTemplate = `
Analyze the following {{ .Language }} code snippet for common code smells (e.g., long methods, large classes, duplicated code, dead code, excessive comments, tight coupling).
For each smell found, describe it and suggest a potential refactoring or improvement.
If no smells are found, state "No obvious code smells detected."
Format your response as a JSON object with a "code_smell_analysis" key.
Example:
{
  "code_smell_analysis": "Smell: Long Method at function 'processData'. Suggestion: Break down into smaller, more focused functions."
}

Code:
{{ .Code }}
`

type smellTemplateData struct {
	Language string
	Code     string
}
type SmellAnalysisResponse struct {
	CodeSmellAnalysis string `json:"code_smell_analysis"`
}

func (a *SmellAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error) {
	appCfg := config.GetConfig()
	lang := getFileExtension(filePath) // Corrected: Use getFileExtension

	data := smellTemplateData{Language: lang, Code: fileContent}
	tmpl, err := template.New("smellPrompt").Parse(smellPromptTemplate)
	if err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to parse smell prompt template", Err: err}
	}
	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to execute smell prompt template", Err: err}
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "Ollama query failed during code smell detection", Err: err}
	}

	var smellResp SmellAnalysisResponse
	if err := json.Unmarshal([]byte(response), &smellResp); err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to parse JSON response for code smell analysis", Err: fmt.Errorf("unmarshal error: %w, raw response: %s", err, response)}
	}
	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    smellResp.CodeSmellAnalysis,
	}, nil
}