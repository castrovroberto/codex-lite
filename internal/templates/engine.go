package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Engine handles template rendering
type Engine struct {
	templatesDir string
}

// NewEngine creates a new template engine
func NewEngine(templatesDir string) *Engine {
	return &Engine{
		templatesDir: templatesDir,
	}
}

// Render renders a template with the given data
func (e *Engine) Render(templateName string, data interface{}) (string, error) {
	templatePath := filepath.Join(e.templatesDir, templateName)

	// Read template file
	content, err := os.ReadFile(templatePath)
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
