package context

import (
	"context"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
)

func TestNewContextIntegrator(t *testing.T) {
	workspaceRoot := "/tmp/test-workspace"
	toolRegistry := agent.NewRegistry()

	integrator := NewContextIntegrator(workspaceRoot, toolRegistry)

	if integrator == nil {
		t.Fatal("Expected non-nil context integrator")
	}

	if integrator.workspaceRoot == "" {
		t.Error("Expected workspace root to be set")
	}

	if integrator.toolRegistry == nil {
		t.Error("Expected tool registry to be set")
	}
}

func TestGatherWorkspaceContext_BasicStructure(t *testing.T) {
	workspaceRoot := "."
	toolRegistry := agent.NewRegistry()

	integrator := NewContextIntegrator(workspaceRoot, toolRegistry)
	ctx := context.Background()

	workspaceContext, err := integrator.GatherWorkspaceContext(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if workspaceContext == nil {
		t.Fatal("Expected non-nil workspace context")
	}

	// Verify basic structure
	if workspaceContext.WorkspaceRoot == "" {
		t.Error("Expected workspace root to be set")
	}

	if workspaceContext.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	if workspaceContext.Environment == nil {
		t.Error("Expected environment map to be initialized")
	}
}

func TestFormatContextForPrompt(t *testing.T) {
	integrator := NewContextIntegrator(".", nil)

	// Create a sample workspace context
	workspaceContext := &WorkspaceContext{
		WorkspaceRoot: "/test/workspace",
		ProjectType:   "go_project",
		GitInfo: &GitContextInfo{
			IsRepo:        true,
			CurrentBranch: "main",
			Status:        "clean",
			RecentCommits: []CommitInfo{
				{Hash: "abc123", Author: "test", Message: "Initial commit"},
			},
		},
		Dependencies: &DependencyInfo{
			PackageManager: "go_modules",
			Languages:      []string{"go"},
		},
		Timestamp: time.Now(),
	}

	formatted := integrator.FormatContextForPrompt(workspaceContext)

	if formatted == "" {
		t.Error("Expected non-empty formatted context")
	}

	// Check that key information is included
	expectedStrings := []string{
		"Workspace Context",
		"/test/workspace",
		"go_project",
		"main",
		"clean",
		"go_modules",
	}

	for _, expected := range expectedStrings {
		if !contains(formatted, expected) {
			t.Errorf("Expected formatted context to contain '%s'", expected)
		}
	}
}

func TestDetectProjectType(t *testing.T) {
	integrator := NewContextIntegrator(".", nil)

	// Test with current directory (should detect go_project due to go.mod)
	projectType := integrator.detectProjectType(nil)

	// Since we're in a Go project with go.mod, it should detect as go_project
	if projectType != "go_project" {
		t.Errorf("Expected 'go_project', got '%s'", projectType)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
