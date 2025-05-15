package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	// "github.com/castrovroberto/codex-lite/internal/config" // No longer needed directly for appCfg
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/ollama"
	// "github.com/castrovroberto/codex-lite/internal/logger" // No longer needed directly for log
)

// ExplainAgent provides explanations for code snippets.
type ExplainAgent struct{}

// NewExplainAgent creates a new ExplainAgent.
func NewExplainAgent() *ExplainAgent {
	return &ExplainAgent{}
}

// Name returns the name of the agent.
func (a *ExplainAgent) Name() string {
	return "Explainer"
}

// Description returns a brief description of the agent.
func (a *ExplainAgent) Description() string {
	return "Explains what a piece of code does in plain language."
}

const explainPromptTemplate = `
Explain the following {{ .Language }} code snippet. Focus on its main purpose, inputs, outputs, and key logic.
Keep the explanation concise and clear.
Format your response as a JSON object with an "explanation" key.
Example:
{
  "explanation": "This code defines a function that calculates the factorial of a number using recursion."
}

Code:
{{ .Code }}
`

type explainTemplateData struct {
	Language string
	Code     string
}

// ExplanationResponse defines the expected JSON structure from Ollama.
type ExplanationResponse struct {
	Explanation string `json:"explanation"`
}

// Analyze performs the code explanation.
func (a *ExplainAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error) {
	log := contextkeys.LoggerFromContext(ctx)
	appCfg := contextkeys.ConfigFromContext(ctx)
	lang := getFileExtension(filePath) // Assumes getFileExtension is in this package (e.g. utils.go)

	log.Debug("Running ExplainAgent", "file", filePath, "model", modelName)

	// Check for context cancellation early
	select {
	case <-ctx.Done():
		log.Info("ExplainAgent analysis cancelled", "file", filePath)
		return Result{AgentName: a.Name(), File: filePath}, ctx.Err()
	default:
		// Continue
	}

	data := explainTemplateData{
		Language: lang,
		Code:     fileContent,
	}

	tmpl, err := template.New("explainPrompt").Parse(explainPromptTemplate)
	if err != nil {
		log.Error("Failed to parse explain prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse explain prompt template",
			Err:       err,
		}
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		log.Error("Failed to execute explain prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to execute explain prompt template",
			Err:       err,
		}
	}

	// Pass context to ollama.Query; appCfg.OllamaHostURL is now used inside ollama.Query via context
	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		log.Error("Ollama query failed for ExplainAgent", "file", filePath, "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "Ollama query failed during code explanation",
			Err:       err,
		}
	}

	log.Debug("Received Ollama response for ExplainAgent", "file", filePath, "response_length", len(response))
	var explanationResp ExplanationResponse
	if err := json.Unmarshal([]byte(response), &explanationResp); err != nil {
		log.Error("Failed to parse JSON response from Ollama for explanation", "response_snippet", response[:min(len(response), 200)], "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{
			AgentName: a.Name(),
			Message:   "failed to parse JSON response from Ollama for explanation",
			Err:       fmt.Errorf("unmarshal error: %w, raw response: %s", err, response[:min(len(response), 500)]),
		}
	}

	log.Debug("ExplainAgent analysis complete", "file", filePath)
	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    explanationResp.Explanation,
	}, nil
}