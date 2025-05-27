package agent

import (
	"fmt"
)

// ToolFactory creates and configures tool registries
type ToolFactory struct {
	workspaceRoot string
}

// NewToolFactory creates a new tool factory
func NewToolFactory(workspaceRoot string) *ToolFactory {
	return &ToolFactory{
		workspaceRoot: workspaceRoot,
	}
}

// CreateRegistry creates a new registry with all available tools
func (tf *ToolFactory) CreateRegistry() *Registry {
	registry := NewRegistry()

	// Register all available tools
	tf.registerCoreTool(registry)

	return registry
}

// CreatePlanningRegistry creates a registry with tools suitable for planning
func (tf *ToolFactory) CreatePlanningRegistry() *Registry {
	registry := NewRegistry()

	// Planning tools - read-only operations
	registry.Register(NewFileReadTool(tf.workspaceRoot))
	registry.Register(NewCodeSearchTool(tf.workspaceRoot))
	registry.Register(NewListDirTool(tf.workspaceRoot))
	registry.Register(NewGitTool(tf.workspaceRoot))

	return registry
}

// CreateGenerationRegistry creates a registry with tools suitable for code generation
func (tf *ToolFactory) CreateGenerationRegistry() *Registry {
	registry := NewRegistry()

	// Generation tools - read and write operations
	registry.Register(NewFileReadTool(tf.workspaceRoot))
	registry.Register(NewFileWriteTool(tf.workspaceRoot))
	registry.Register(NewCodeSearchTool(tf.workspaceRoot))
	registry.Register(NewListDirTool(tf.workspaceRoot))
	registry.Register(NewPatchApplyTool(tf.workspaceRoot))
	registry.Register(NewGitTool(tf.workspaceRoot))

	return registry
}

// CreateReviewRegistry creates a registry with tools suitable for code review
func (tf *ToolFactory) CreateReviewRegistry() *Registry {
	registry := NewRegistry()

	// Review tools - read, write, and test operations
	registry.Register(NewFileReadTool(tf.workspaceRoot))
	registry.Register(NewFileWriteTool(tf.workspaceRoot))
	registry.Register(NewCodeSearchTool(tf.workspaceRoot))
	registry.Register(NewListDirTool(tf.workspaceRoot))
	registry.Register(NewPatchApplyTool(tf.workspaceRoot))
	registry.Register(NewShellRunTool(tf.workspaceRoot))
	registry.Register(NewGitTool(tf.workspaceRoot))
	registry.Register(NewGitCommitTool(tf.workspaceRoot))
	registry.Register(NewTestRunnerTool(tf.workspaceRoot))
	registry.Register(NewLintRunnerTool(tf.workspaceRoot))

	return registry
}

// CreateFullRegistry creates a registry with all available tools
func (tf *ToolFactory) CreateFullRegistry() *Registry {
	registry := NewRegistry()
	tf.registerCoreTool(registry)
	return registry
}

// registerCoreTool registers all core tools
func (tf *ToolFactory) registerCoreTool(registry *Registry) {
	tools := []Tool{
		NewFileReadTool(tf.workspaceRoot),
		NewFileWriteTool(tf.workspaceRoot),
		NewCodeSearchTool(tf.workspaceRoot),
		NewListDirTool(tf.workspaceRoot),
		NewPatchApplyTool(tf.workspaceRoot),
		NewShellRunTool(tf.workspaceRoot),
		NewGitTool(tf.workspaceRoot),
		NewGitCommitTool(tf.workspaceRoot),
		NewTestRunnerTool(tf.workspaceRoot),
		NewLintRunnerTool(tf.workspaceRoot),
	}

	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			// Log error but continue - this shouldn't happen with our tools
			fmt.Printf("Warning: failed to register tool %s: %v\n", tool.Name(), err)
		}
	}
}

// GetAvailableToolNames returns the names of all available tools
func (tf *ToolFactory) GetAvailableToolNames() []string {
	return []string{
		"read_file",
		"write_file",
		"codebase_search",
		"list_directory",
		"apply_patch_to_file",
		"run_shell_command",
		"git_info",
		"git_commit",
		"run_tests",
		"run_linter",
	}
}
