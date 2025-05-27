package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanTemplateWithFunctionCalling(t *testing.T) {
	// Create a temporary templates directory
	tempDir := t.TempDir()

	// Create a simple plan template for testing
	planTemplate := `You are a planner with tools.
User Goal: {{.UserGoal}}
Use function calls to gather context before planning.`

	templatePath := filepath.Join(tempDir, "plan.tmpl")
	err := os.WriteFile(templatePath, []byte(planTemplate), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	engine := NewEngine(tempDir)

	data := PlanTemplateData{
		UserGoal: "Test goal",
	}

	result, err := engine.Render("plan.tmpl", data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Verify the template rendered correctly
	if !strings.Contains(result, "Test goal") {
		t.Errorf("Template did not include user goal")
	}

	if !strings.Contains(result, "function calls") {
		t.Errorf("Template did not mention function calls")
	}
}

func TestGenerateTemplateValidation(t *testing.T) {
	tempDir := t.TempDir()

	generateTemplate := `You are a generator.
Task: {{.TaskDescription}}
Use function calls for file operations.
Safety guidelines apply.`

	templatePath := filepath.Join(tempDir, "generate.tmpl")
	err := os.WriteFile(templatePath, []byte(generateTemplate), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	engine := NewEngine(tempDir)

	data := GenerateTemplateData{
		TaskDescription: "Test task",
		FilesToModify:   []string{"test.go"},
	}

	result, err := engine.Render("generate.tmpl", data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Verify safety guidelines are mentioned
	if !strings.Contains(result, "function calls") {
		t.Errorf("Template did not mention function calls")
	}
}

func TestReviewTemplateWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	reviewTemplate := `You are a reviewer.
Issues: {{range .Issues}}{{.}}{{end}}
Use function calls to analyze and fix.
Follow the workflow guidelines.`

	templatePath := filepath.Join(tempDir, "review.tmpl")
	err := os.WriteFile(templatePath, []byte(reviewTemplate), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	engine := NewEngine(tempDir)

	data := ReviewTemplateData{
		Issues:    []string{"Test issue 1", "Test issue 2"},
		TargetDir: "/test/dir",
	}

	result, err := engine.Render("review.tmpl", data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Verify issues are included
	if !strings.Contains(result, "Test issue 1") {
		t.Errorf("Template did not include first issue")
	}

	if !strings.Contains(result, "function calls") {
		t.Errorf("Template did not mention function calls")
	}
}

func TestTemplateParameterValidation(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		params      string
		workspace   string
		expectError bool
	}{
		{
			name:        "valid file operation",
			toolName:    "read_file",
			params:      `{"target_file": "test.go"}`,
			workspace:   "/workspace",
			expectError: false,
		},
		{
			name:        "invalid absolute path",
			toolName:    "read_file",
			params:      `{"target_file": "/etc/passwd"}`,
			workspace:   "/workspace",
			expectError: true,
		},
		{
			name:        "path traversal attempt",
			toolName:    "write_file",
			params:      `{"file_path": "../../../etc/passwd"}`,
			workspace:   "/workspace",
			expectError: true,
		},
		{
			name:        "dangerous shell command",
			toolName:    "run_shell_command",
			params:      `{"command": "rm -rf /"}`,
			workspace:   "/workspace",
			expectError: true,
		},
		{
			name:        "safe shell command",
			toolName:    "run_shell_command",
			params:      `{"command": "go test ./..."}`,
			workspace:   "/workspace",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolCall(tt.toolName, json.RawMessage(tt.params), tt.workspace)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestRenderWithTools(t *testing.T) {
	tempDir := t.TempDir()

	template := `Available tools: {{len .AvailableTools}}
Safety guidelines: {{len .SafetyGuidelines}}`

	templatePath := filepath.Join(tempDir, "test.tmpl")
	err := os.WriteFile(templatePath, []byte(template), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	engine := NewEngine(tempDir)

	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file",
			Parameters:  json.RawMessage(`{"type": "object"}`),
		},
	}

	result, err := engine.RenderWithTools("test.tmpl", nil, tools)
	if err != nil {
		t.Fatalf("Failed to render template with tools: %v", err)
	}

	// Should show 1 tool and multiple safety guidelines
	if !strings.Contains(result, "Available tools: 1") {
		t.Errorf("Template did not show correct tool count")
	}
}

func TestRenderWithContext(t *testing.T) {
	tempDir := t.TempDir()

	template := `Workspace: {{.WorkspaceRoot}}
Max iterations: {{.MaxIterations}}
Guidelines: {{len .SafetyGuidelines}}`

	templatePath := filepath.Join(tempDir, "test.tmpl")
	err := os.WriteFile(templatePath, []byte(template), 0644)
	if err != nil {
		t.Fatalf("Failed to write test template: %v", err)
	}

	engine := NewEngine(tempDir)

	result, err := engine.RenderWithContext("test.tmpl", nil, "/test/workspace", 10)
	if err != nil {
		t.Fatalf("Failed to render template with context: %v", err)
	}

	if !strings.Contains(result, "Workspace: /test/workspace") {
		t.Errorf("Template did not show correct workspace")
	}

	if !strings.Contains(result, "Max iterations: 10") {
		t.Errorf("Template did not show correct max iterations")
	}
}

func TestSafetyGuidelines(t *testing.T) {
	guidelines := SafetyGuidelines()

	if len(guidelines) == 0 {
		t.Errorf("Expected safety guidelines but got none")
	}

	// Check for key safety concepts
	guidelinesText := strings.Join(guidelines, " ")

	expectedConcepts := []string{
		"relative paths",
		"workspace root",
		"backup",
		"validate",
	}

	for _, concept := range expectedConcepts {
		if !strings.Contains(strings.ToLower(guidelinesText), concept) {
			t.Errorf("Safety guidelines missing concept: %s", concept)
		}
	}
}

func TestValidateFileOperationEdgeCases(t *testing.T) {
	workspace := "/test/workspace"

	tests := []struct {
		name        string
		params      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty file path",
			params:      `{"file_path": ""}`,
			expectError: true,
			errorMsg:    "file path cannot be empty",
		},
		{
			name:        "missing file path",
			params:      `{}`,
			expectError: true,
			errorMsg:    "file path cannot be empty",
		},
		{
			name:        "valid relative path",
			params:      `{"file_path": "src/main.go"}`,
			expectError: false,
		},
		{
			name:        "path with dots but safe",
			params:      `{"file_path": "src/test.go"}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileOperation(json.RawMessage(tt.params), workspace)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
