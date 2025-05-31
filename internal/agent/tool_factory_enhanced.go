package agent

import (
	"context"
	"io"
	"os"
)

// FileSystemService abstracts file system operations for tools
type FileSystemService interface {
	WriteFile(path string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	ListDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	Exists(path string) bool
	IsDir(path string) bool
}

// CommandExecutor abstracts command execution for tools
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args ...string) (output []byte, err error)
	ExecuteWithWorkDir(ctx context.Context, workDir, command string, args ...string) (output []byte, err error)
	ExecuteStream(ctx context.Context, command string, args ...string) (stdout, stderr io.Reader, err error)
}

// EnhancedToolFactory creates tools with dependency injection
type EnhancedToolFactory struct {
	workspaceRoot string
	config        *ToolFactoryConfig
	fileSystem    FileSystemService
	cmdExecutor   CommandExecutor
}

// NewEnhancedToolFactory creates a new enhanced tool factory with dependency injection
func NewEnhancedToolFactory(workspaceRoot string, fs FileSystemService, exec CommandExecutor) *EnhancedToolFactory {
	return &EnhancedToolFactory{
		workspaceRoot: workspaceRoot,
		config:        &ToolFactoryConfig{},
		fileSystem:    fs,
		cmdExecutor:   exec,
	}
}

// NewEnhancedToolFactoryWithConfig creates a new enhanced tool factory with custom configuration
func NewEnhancedToolFactoryWithConfig(workspaceRoot string, config ToolFactoryConfig, fs FileSystemService, exec CommandExecutor) *EnhancedToolFactory {
	return &EnhancedToolFactory{
		workspaceRoot: workspaceRoot,
		config:        &config,
		fileSystem:    fs,
		cmdExecutor:   exec,
	}
}

// CreateGenerationRegistry creates a registry with tools suitable for code generation
func (etf *EnhancedToolFactory) CreateGenerationRegistry() *Registry {
	registry := NewRegistry()

	// Create tools with dependency injection
	registry.Register(NewFileReadToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewFileWriteToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewCodeSearchToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(etf.createListDirTool())
	registry.Register(NewPatchApplyToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewGitToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewClarificationTool(etf.workspaceRoot))

	return registry
}

// CreateReviewRegistry creates a registry with tools suitable for code review
func (etf *EnhancedToolFactory) CreateReviewRegistry() *Registry {
	registry := NewRegistry()

	// Create tools with dependency injection
	registry.Register(NewFileReadToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewFileWriteToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewCodeSearchToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(etf.createListDirTool())
	registry.Register(NewPatchApplyToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewShellRunToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewGitToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewGitCommitToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewTestRunnerToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewLintRunnerToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewParseTestResultsToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewParseLintResultsToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewClarificationTool(etf.workspaceRoot))

	return registry
}

// CreatePlanningRegistry creates a registry with tools suitable for planning
func (etf *EnhancedToolFactory) CreatePlanningRegistry() *Registry {
	registry := NewRegistry()

	// Planning tools - read-only operations
	registry.Register(NewFileReadToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(NewCodeSearchToolWithFS(etf.workspaceRoot, etf.fileSystem))
	registry.Register(etf.createListDirTool())
	registry.Register(NewGitToolWithExecutor(etf.workspaceRoot, etf.cmdExecutor))
	registry.Register(NewClarificationTool(etf.workspaceRoot))

	return registry
}

// createListDirTool creates the appropriate list directory tool based on configuration
func (etf *EnhancedToolFactory) createListDirTool() Tool {
	if etf.config != nil && etf.config.ListDirectory != nil {
		return NewListDirToolWithConfigAndFS(etf.workspaceRoot, *etf.config.ListDirectory, etf.fileSystem)
	}
	return NewListDirToolWithFS(etf.workspaceRoot, etf.fileSystem)
}

// Placeholder constructors for enhanced tools (these would need to be implemented)
// For now, we'll fallback to regular constructors and gradually enhance each tool

func NewFileReadToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced file read tool with FS injection
	return NewFileReadTool(workspaceRoot)
}

func NewFileWriteToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// Use the enhanced file write tool with FS injection
	return NewFileWriteToolEnhanced(workspaceRoot, fs)
}

func NewCodeSearchToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced code search tool with FS injection
	return NewCodeSearchTool(workspaceRoot)
}

func NewListDirToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced list dir tool with FS injection
	return NewListDirTool(workspaceRoot)
}

func NewListDirToolWithConfigAndFS(workspaceRoot string, config ListDirToolConfig, fs FileSystemService) Tool {
	// TODO: Implement enhanced list dir tool with config and FS injection
	return NewListDirToolWithConfig(workspaceRoot, config)
}

func NewPatchApplyToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced patch apply tool with FS injection
	return NewPatchApplyTool(workspaceRoot)
}

func NewShellRunToolWithExecutor(workspaceRoot string, exec CommandExecutor) Tool {
	// TODO: Implement enhanced shell run tool with executor injection
	return NewShellRunTool(workspaceRoot)
}

func NewGitToolWithExecutor(workspaceRoot string, exec CommandExecutor) Tool {
	// TODO: Implement enhanced git tool with executor injection
	return NewGitTool(workspaceRoot)
}

func NewGitCommitToolWithExecutor(workspaceRoot string, exec CommandExecutor) Tool {
	// TODO: Implement enhanced git commit tool with executor injection
	return NewGitCommitTool(workspaceRoot)
}

func NewTestRunnerToolWithExecutor(workspaceRoot string, exec CommandExecutor) Tool {
	// TODO: Implement enhanced test runner tool with executor injection
	return NewTestRunnerTool(workspaceRoot)
}

func NewLintRunnerToolWithExecutor(workspaceRoot string, exec CommandExecutor) Tool {
	// TODO: Implement enhanced lint runner tool with executor injection
	return NewLintRunnerTool(workspaceRoot)
}

func NewParseTestResultsToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced parse test results tool with FS injection
	return NewParseTestResultsTool(workspaceRoot)
}

func NewParseLintResultsToolWithFS(workspaceRoot string, fs FileSystemService) Tool {
	// TODO: Implement enhanced parse lint results tool with FS injection
	return NewParseLintResultsTool(workspaceRoot)
}
