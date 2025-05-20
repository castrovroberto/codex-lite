package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	// "github.com/castrovroberto/codex-lite/internal/config" // No longer needed directly
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	// "github.com/castrovroberto/codex-lite/internal/logger" // No longer needed directly
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
	log := contextkeys.LoggerFromContext(ctx)
	appCfg := contextkeys.ConfigFromContext(ctx)
	lang := getFileExtension(filePath) // This line calls the function from utils.go

	log.Debug("Running SyntaxAgent", "file", filePath, "model", modelName)

	// Check for context cancellation early
	select {
	case <-ctx.Done():
		log.Info("SyntaxAgent analysis cancelled", "file", filePath)
		return Result{AgentName: a.Name(), File: filePath}, ctx.Err()
	default:
		// Continue
	}

	data := syntaxTemplateData{
		Language: lang,
		Code:     fileContent,
	}

	tmpl, err := template.New("syntaxPrompt").Parse(syntaxPromptTemplate)
	if err != nil {
		log.Error("Failed to parse syntax prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse syntax prompt template",
			Err:       err,
		}
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		log.Error("Failed to execute syntax prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to execute syntax prompt template",
			Err:       err,
		}
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		log.Error("Ollama query failed for SyntaxAgent", "file", filePath, "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "Ollama query failed during syntax analysis",
			Err:       err,
		}
	}

	log.Debug("Received Ollama response for SyntaxAgent", "file", filePath, "response_length", len(response))
	var syntaxResp SyntaxAnalysisResponse
	if err := json.Unmarshal([]byte(response), &syntaxResp); err != nil {
		log.Error("Failed to parse JSON response from Ollama for syntax analysis", "response_snippet", response[:min(len(response), 200)], "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse JSON response from Ollama for syntax analysis",
			Err:       fmt.Errorf("unmarshal error: %w, raw response: %s", err, response[:min(len(response), 500)]),
		}
	}

	log.Debug("SyntaxAgent analysis complete", "file", filePath)
	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    syntaxResp.Analysis,
	}, nil
}

// min helper function (can be in a utils package if used more broadly)
// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }