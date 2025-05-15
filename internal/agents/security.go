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
	appCfg := config.GetConfig()
	lang := getFileExtension(filePath) // Corrected: Use getFileExtension

	data := securityTemplateData{Language: lang, Code: fileContent}
	tmpl, err := template.New("securityPrompt").Parse(securityPromptTemplate)
	if err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to parse security prompt template", Err: err}
	}
	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to execute security prompt template", Err: err}
	}

	response, err := ollama.Query(ctx, appCfg.OllamaHostURL, modelName, promptBuf.String())
	if err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "Ollama query failed during security audit", Err: err}
	}

	var securityResp SecurityAnalysisResponse
	if err := json.Unmarshal([]byte(response), &securityResp); err != nil {
		return Result{}, &AgentError{AgentName: a.Name(), Message: "failed to parse JSON response for security audit", Err: fmt.Errorf("unmarshal error: %w, raw response: %s", err, response)}
	}
	return Result{
		AgentName: a.Name(),
		File:      filePath,
		Output:    securityResp.SecurityAnalysis,
	}, nil
}