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

type SecurityAgent struct{}

func NewSecurityAgent() *SecurityAgent { return &SecurityAgent{} }
func (a *SecurityAgent) Name() string { return "Security Auditor" }
func (a *SecurityAgent) Description() string {
	return "Audits code for potential security vulnerabilities and suggests fixes."
}

const securityPromptTemplate = `
Analyze the following {{ .Language }} code snippet for potential security vulnerabilities (e.g., SQL injection, XSS, buffer overflows, insecure handling of secrets).
For each vulnerability found, describe it, explain the potential impact, and suggest a mitigation.
If no vulnerabilities are found, state "No obvious security vulnerabilities found."
Format your response as a JSON object with a "security_analysis" key.
Example:
{
  "security_analysis": "Vulnerability: SQL Injection at line 10. Impact: ... Mitigation: Use prepared statements..."
}

Code:
{{ .Code }}
`

type securityTemplateData struct {
	Language string
	Code     string
}
type SecurityAnalysisResponse struct {
	SecurityAnalysis string `json:"security_analysis"`
}

func (a *SecurityAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (Result, error) {
	log := contextkeys.LoggerFromContext(ctx)
	appCfg := contextkeys.ConfigFromContext(ctx)
	lang := getFileExtension(filePath)

	log.Debug("Running SecurityAgent", "file", filePath, "model", modelName)

	// Check for context cancellation early
	select {
	case <-ctx.Done():
		log.Info("SecurityAgent analysis cancelled", "file", filePath)
		return Result{AgentName: a.Name(), File: filePath}, ctx.Err()
	default:
		// Continue
	}

	data := securityTemplateData{Language: lang, Code: fileContent}
	tmpl, err := template.New("securityPrompt").Parse(securityPromptTemplate)
	if err != nil {
		log.Error("Failed to parse security prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{AgentName: a.Name(), Message: "failed to parse security prompt template", Err: err}
	}
	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		log.Error("Failed to execute security prompt template", "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{AgentName: a.Name(), Message: "failed to execute security prompt template", Err: err}
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		log.Error("Ollama query failed for SecurityAgent", "file", filePath, "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{AgentName: a.Name(), Message: "Ollama query failed during security audit", Err: err}
	}

	log.Debug("Received Ollama response for SecurityAgent", "file", filePath, "response_length", len(response))
	var securityResp SecurityAnalysisResponse
	if err := json.Unmarshal([]byte(response), &securityResp); err != nil {
		log.Error("Failed to parse JSON response from Ollama for security audit", "response_snippet", response[:min(len(response), 200)], "error", err)
		return Result{AgentName: a.Name(), File: filePath}, &AgentError{AgentName: a.Name(), Message: "failed to parse JSON response for security audit", Err: fmt.Errorf("unmarshal error: %w, raw response: %s", err, response[:min(len(response), 500)])}
	}

	log.Debug("SecurityAgent analysis complete", "file", filePath)
	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    securityResp.SecurityAnalysis,
	}, nil
}