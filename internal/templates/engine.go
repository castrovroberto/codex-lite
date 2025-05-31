package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/castrovroberto/CGE/internal/security"
)

// Engine handles template rendering
type Engine struct {
	templatesDir string
	safeOps      *security.SafeFileOps
}

// NewEngine creates a new template engine
func NewEngine(templatesDir string) *Engine {
	// Create safe file operations with templates directory as allowed root
	safeOps := security.NewSafeFileOps(templatesDir)

	return &Engine{
		templatesDir: templatesDir,
		safeOps:      safeOps,
	}
}

// Render renders a template with the given data
func (e *Engine) Render(templateName string, data interface{}) (string, error) {
	templatePath := filepath.Join(e.templatesDir, templateName)

	// Read template file using secure file operations
	content, err := e.safeOps.SafeReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Parse template
	tmpl, err := template.New(templateName).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// RenderWithTools renders a template with tool definitions included
func (e *Engine) RenderWithTools(templateName string, data interface{}, tools []ToolDefinition) (string, error) {
	// Create enhanced data structure that includes tools
	enhancedData := FunctionCallingTemplateData{
		BaseData:         data,
		AvailableTools:   tools,
		SafetyGuidelines: SafetyGuidelines(),
	}

	return e.Render(templateName, enhancedData)
}

// RenderWithContext renders a template with additional context for function calling
func (e *Engine) RenderWithContext(templateName string, data interface{}, workspaceRoot string, maxIterations int) (string, error) {
	enhancedData := FunctionCallingTemplateData{
		BaseData:         data,
		WorkspaceRoot:    workspaceRoot,
		MaxIterations:    maxIterations,
		SafetyGuidelines: SafetyGuidelines(),
	}

	return e.Render(templateName, enhancedData)
}

// ToolDefinition represents a tool available for function calling
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// FunctionCallingTemplateData holds enhanced data for function-calling templates
type FunctionCallingTemplateData struct {
	BaseData         interface{}      `json:"base_data"`
	AvailableTools   []ToolDefinition `json:"available_tools,omitempty"`
	MaxIterations    int              `json:"max_iterations,omitempty"`
	WorkspaceRoot    string           `json:"workspace_root,omitempty"`
	SafetyGuidelines []string         `json:"safety_guidelines,omitempty"`
}

// PlanTemplateData holds data for the plan template
type PlanTemplateData struct {
	UserGoal        string
	CodebaseContext string
	GitInfo         string
	FileStructure   string
	Dependencies    string
}

// GenerateTemplateData holds data for the generate template
type GenerateTemplateData struct {
	TaskID              string
	TaskDescription     string
	EstimatedEffort     string
	Rationale           string
	OverallGoal         string
	FilesToModify       []string
	FilesToCreate       []string
	FilesToDelete       []string
	CurrentFileContents string
	ProjectContext      string
}

// ReviewTemplateData holds data for the review template
type ReviewTemplateData struct {
	TestOutput     string
	LintOutput     string
	Issues         []string
	TargetDir      string
	FileContents   map[string]string
	ProjectContext string
}
