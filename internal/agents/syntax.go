package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	// "github.com/castrovroberto/codex-lite/internal/logger"
)

// SyntaxAgent focuses on identifying syntax errors or potential issues.
type SyntaxAgent struct{}

func NewSyntaxAgent() *SyntaxAgent {
	return &SyntaxAgent{}
}

func (a *SyntaxAgent) Name() string {
	return "Syntax Checker"
}

func (a *SyntaxAgent) Description() string {
	return "Identifies potential syntax errors and provides corrections or suggestions."
}

const syntaxPromptTemplate = `
Analyze the following {{ .Language }} code snippet for syntax errors and potential issues.
If you find any, provide a brief explanation of the error and a corrected version if possible.
If no errors are found, state "No syntax issues found."
Format your response as a JSON object with a "syntax_analysis" key containing your findings.
Example for an error:
{
  "syntax_analysis": "Error: Missing semicolon at line 5. Corrected: ... (corrected code) ..."
}
Example for no errors:
{
  "syntax_analysis": "No syntax issues found."
}

Code:
{{ .Code }}
`

type syntaxTemplateData struct {
	Language string
	Code     string
}

// SyntaxAnalysisResponse defines the expected JSON structure from Ollama for syntax analysis.
type SyntaxAnalysisResponse struct {
	Analysis string `json:"syntax_analysis"`
}

func (a *SyntaxAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error) {
	appCfg := config.GetConfig()
	lang := getFileExtension(filePath) // Corrected: Use getFileExtension

	data := syntaxTemplateData{
		Language: lang,
		Code:     fileContent,
	}

	tmpl, err := template.New("syntaxPrompt").Parse(syntaxPromptTemplate)
	if err != nil {
		return Result{}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse syntax prompt template",
			Err:       err,
		}
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to execute syntax prompt template",
			Err:       err,
		}
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, &AgentError{
			AgentName: a.Name(),
			Message:   "Ollama query failed during syntax analysis",
			Err:       err,
		}
	}

	var syntaxResp SyntaxAnalysisResponse
	if err := json.Unmarshal([]byte(response), &syntaxResp); err != nil {
		return Result{}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse JSON response from Ollama for syntax analysis",
			Err:       fmt.Errorf("unmarshal error: %w, raw response: %s", err, response),
		}
	}

	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    syntaxResp.Analysis,
	}, nil
}