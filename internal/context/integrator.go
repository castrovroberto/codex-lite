package context

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
)

// ContextIntegrator provides workspace context to the LLM
type ContextIntegrator struct {
	workspaceRoot string
	gatherer      *Gatherer
	toolRegistry  *agent.Registry
}

// NewContextIntegrator creates a new context integrator
func NewContextIntegrator(workspaceRoot string, toolRegistry *agent.Registry) *ContextIntegrator {
	// Ensure workspace root is absolute
	absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absWorkspaceRoot = workspaceRoot // Fallback to original if conversion fails
	}

	return &ContextIntegrator{
		workspaceRoot: absWorkspaceRoot,
		gatherer:      NewGatherer(absWorkspaceRoot),
		toolRegistry:  toolRegistry,
	}
}

// WorkspaceContext represents the complete workspace context
type WorkspaceContext struct {
	WorkspaceRoot    string                 `json:"workspace_root"`
	ProjectType      string                 `json:"project_type"`
	GitInfo          *GitContextInfo        `json:"git_info,omitempty"`
	ProjectStructure *ProjectStructureInfo  `json:"project_structure"`
	Dependencies     *DependencyInfo        `json:"dependencies,omitempty"`
	RecentActivity   *RecentActivityInfo    `json:"recent_activity,omitempty"`
	Environment      map[string]interface{} `json:"environment,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
}

// GitContextInfo represents Git repository context
type GitContextInfo struct {
	IsRepo         bool              `json:"is_repo"`
	CurrentBranch  string            `json:"current_branch,omitempty"`
	Status         string            `json:"status,omitempty"`
	RecentCommits  []CommitInfo      `json:"recent_commits,omitempty"`
	UntrackedFiles []string          `json:"untracked_files,omitempty"`
	ModifiedFiles  []string          `json:"modified_files,omitempty"`
	StagedFiles    []string          `json:"staged_files,omitempty"`
	RemoteInfo     map[string]string `json:"remote_info,omitempty"`
}

// CommitInfo represents a Git commit
type CommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Message string `json:"message"`
	Date    string `json:"date,omitempty"`
}

// ProjectStructureInfo represents project organization
type ProjectStructureInfo struct {
	RootFiles   []string           `json:"root_files"`
	Directories []DirectoryInfo    `json:"directories"`
	FileTypes   map[string]int     `json:"file_types"`
	TotalFiles  int                `json:"total_files"`
	TotalSize   int64              `json:"total_size_bytes"`
	ConfigFiles []string           `json:"config_files"`
	BuildFiles  []string           `json:"build_files"`
	Conventions ProjectConventions `json:"conventions"`
}

// DirectoryInfo represents directory structure information
type DirectoryInfo struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	FileCount int      `json:"file_count"`
	SubDirs   []string `json:"subdirs,omitempty"`
	Purpose   string   `json:"purpose,omitempty"` // Inferred purpose (src, test, docs, etc.)
}

// ProjectConventions represents detected project conventions
type ProjectConventions struct {
	NamingStyle    string   `json:"naming_style"`    // snake_case, camelCase, kebab-case
	CodeStyle      string   `json:"code_style"`      // detected from .editorconfig, etc.
	TestPatterns   []string `json:"test_patterns"`   // *_test.go, test_*.py, etc.
	ImportStyle    string   `json:"import_style"`    // relative, absolute
	DirectoryStyle string   `json:"directory_style"` // flat, nested, domain-driven
}

// DependencyInfo represents project dependencies
type DependencyInfo struct {
	PackageManager  string            `json:"package_manager"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Frameworks      []string          `json:"frameworks,omitempty"`
	Languages       []string          `json:"languages,omitempty"`
}

// RecentActivityInfo represents recent workspace activity
type RecentActivityInfo struct {
	LastModified     time.Time `json:"last_modified"`
	RecentlyModified []string  `json:"recently_modified_files"`
	ActiveBranches   []string  `json:"active_branches,omitempty"`
	LastBuildTime    time.Time `json:"last_build_time,omitempty"`
	LastTestRun      time.Time `json:"last_test_run,omitempty"`
}

// GatherWorkspaceContext collects comprehensive workspace context
func (ci *ContextIntegrator) GatherWorkspaceContext(ctx context.Context) (*WorkspaceContext, error) {
	workspaceContext := &WorkspaceContext{
		WorkspaceRoot: ci.workspaceRoot,
		Timestamp:     time.Now(),
		Environment:   make(map[string]interface{}),
	}

	// Gather Git information using the git_info tool
	if gitTool := ci.getGitTool(); gitTool != nil {
		gitInfo, err := ci.gatherGitContext(ctx, gitTool)
		if err == nil {
			workspaceContext.GitInfo = gitInfo
		}
	}

	// Gather project structure using list_directory tool
	if listDirTool := ci.getListDirectoryTool(); listDirTool != nil {
		structureInfo, err := ci.gatherProjectStructure(ctx, listDirTool)
		if err == nil {
			workspaceContext.ProjectStructure = structureInfo
		}
	}

	// Detect project type from structure and files
	workspaceContext.ProjectType = ci.detectProjectType(workspaceContext.ProjectStructure)

	// Gather dependency information
	if depInfo, err := ci.gatherDependencyInfo(); err == nil {
		workspaceContext.Dependencies = depInfo
	}

	// Gather recent activity information
	if activityInfo, err := ci.gatherRecentActivity(); err == nil {
		workspaceContext.RecentActivity = activityInfo
	}

	// Add environment information
	workspaceContext.Environment["workspace_absolute_path"] = ci.workspaceRoot
	workspaceContext.Environment["os_separator"] = string(filepath.Separator)

	return workspaceContext, nil
}

// gatherGitContext collects Git repository information
func (ci *ContextIntegrator) gatherGitContext(ctx context.Context, gitTool agent.Tool) (*GitContextInfo, error) {
	// Call git_info tool with commit history
	params := json.RawMessage(`{"include_commits": true, "commit_count": 5}`)
	result, err := gitTool.Execute(ctx, params)
	if err != nil || !result.Success {
		return &GitContextInfo{IsRepo: false}, nil
	}

	gitData, ok := result.Data.(map[string]interface{})
	if !ok {
		return &GitContextInfo{IsRepo: false}, nil
	}

	gitInfo := &GitContextInfo{
		IsRepo: true,
	}

	// Extract branch information
	if branch, ok := gitData["branch"].(string); ok {
		gitInfo.CurrentBranch = branch
	}

	// Extract status information
	if statusData, ok := gitData["status"].(map[string]interface{}); ok {
		if clean, ok := statusData["clean"].(bool); ok && clean {
			gitInfo.Status = "clean"
		} else {
			gitInfo.Status = "has_changes"
			// TODO: Extract specific file changes if needed
		}
	}

	// Extract recent commits
	if commitsData, ok := gitData["recent_commits"].([]interface{}); ok {
		for _, commitInterface := range commitsData {
			if commitMap, ok := commitInterface.(map[string]interface{}); ok {
				commit := CommitInfo{}
				if hash, ok := commitMap["hash"].(string); ok {
					commit.Hash = hash
				}
				if author, ok := commitMap["author"].(string); ok {
					commit.Author = author
				}
				if message, ok := commitMap["message"].(string); ok {
					commit.Message = message
				}
				gitInfo.RecentCommits = append(gitInfo.RecentCommits, commit)
			}
		}
	}

	return gitInfo, nil
}

// gatherProjectStructure collects project structure information
func (ci *ContextIntegrator) gatherProjectStructure(ctx context.Context, listDirTool agent.Tool) (*ProjectStructureInfo, error) {
	// Get root directory listing
	params := json.RawMessage(`{"directory_path": ".", "recursive": true, "max_depth": 3}`)
	result, err := listDirTool.Execute(ctx, params)
	if err != nil || !result.Success {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	// Parse the result to build structure info
	structureInfo := &ProjectStructureInfo{
		FileTypes:   make(map[string]int),
		Conventions: ProjectConventions{},
	}

	// TODO: Parse the actual result data structure based on list_directory tool output
	// This would need to be implemented based on the actual tool output format

	return structureInfo, nil
}

// gatherDependencyInfo analyzes project dependencies
func (ci *ContextIntegrator) gatherDependencyInfo() (*DependencyInfo, error) {
	depInfo := &DependencyInfo{}

	// Detect package manager and dependencies based on files present
	// This could be enhanced to actually parse dependency files
	if ci.fileExists("go.mod") {
		depInfo.PackageManager = "go_modules"
		depInfo.Languages = append(depInfo.Languages, "go")
	}
	if ci.fileExists("package.json") {
		depInfo.PackageManager = "npm"
		depInfo.Languages = append(depInfo.Languages, "javascript", "typescript")
	}
	if ci.fileExists("requirements.txt") || ci.fileExists("pyproject.toml") {
		depInfo.PackageManager = "pip"
		depInfo.Languages = append(depInfo.Languages, "python")
	}
	if ci.fileExists("Cargo.toml") {
		depInfo.PackageManager = "cargo"
		depInfo.Languages = append(depInfo.Languages, "rust")
	}

	return depInfo, nil
}

// gatherRecentActivity collects recent workspace activity information
func (ci *ContextIntegrator) gatherRecentActivity() (*RecentActivityInfo, error) {
	activityInfo := &RecentActivityInfo{
		LastModified: time.Now(), // Placeholder
	}

	// TODO: Implement actual file modification time analysis
	// This could examine file timestamps, build artifacts, etc.

	return activityInfo, nil
}

// detectProjectType determines the project type based on structure and files
func (ci *ContextIntegrator) detectProjectType(structure *ProjectStructureInfo) string {
	if ci.fileExists("go.mod") {
		return "go_project"
	}
	if ci.fileExists("package.json") {
		return "node_project"
	}
	if ci.fileExists("pyproject.toml") || ci.fileExists("setup.py") {
		return "python_project"
	}
	if ci.fileExists("Cargo.toml") {
		return "rust_project"
	}
	if ci.fileExists("pom.xml") || ci.fileExists("build.gradle") {
		return "java_project"
	}
	if ci.fileExists("Makefile") {
		return "make_project"
	}
	return "generic_project"
}

// FormatContextForPrompt formats the workspace context for inclusion in LLM prompts
func (ci *ContextIntegrator) FormatContextForPrompt(context *WorkspaceContext) string {
	var builder strings.Builder

	builder.WriteString("## 🔍 Workspace Context\n\n")

	// Basic info
	builder.WriteString(fmt.Sprintf("**Working Directory:** `%s`\n", context.WorkspaceRoot))
	builder.WriteString(fmt.Sprintf("**Project Type:** %s\n", context.ProjectType))

	// Git information
	if context.GitInfo != nil && context.GitInfo.IsRepo {
		builder.WriteString(fmt.Sprintf("**Git Branch:** %s\n", context.GitInfo.CurrentBranch))
		builder.WriteString(fmt.Sprintf("**Git Status:** %s\n", context.GitInfo.Status))

		if len(context.GitInfo.RecentCommits) > 0 {
			builder.WriteString("**Recent Commits:**\n")
			for i, commit := range context.GitInfo.RecentCommits {
				if i >= 3 { // Limit to 3 recent commits
					break
				}
				builder.WriteString(fmt.Sprintf("- `%s`: %s\n", commit.Hash, commit.Message))
			}
		}
	} else {
		builder.WriteString("**Git:** Not a Git repository\n")
	}

	// Project structure summary
	if context.ProjectStructure != nil {
		builder.WriteString(fmt.Sprintf("**Total Files:** %d\n", context.ProjectStructure.TotalFiles))
		if len(context.ProjectStructure.ConfigFiles) > 0 {
			builder.WriteString(fmt.Sprintf("**Config Files:** %s\n", strings.Join(context.ProjectStructure.ConfigFiles, ", ")))
		}
	}

	// Dependencies
	if context.Dependencies != nil {
		builder.WriteString(fmt.Sprintf("**Package Manager:** %s\n", context.Dependencies.PackageManager))
		if len(context.Dependencies.Languages) > 0 {
			builder.WriteString(fmt.Sprintf("**Languages:** %s\n", strings.Join(context.Dependencies.Languages, ", ")))
		}
	}

	builder.WriteString("\n---\n")
	builder.WriteString("*This context was gathered automatically. Use `git_info` and `list_directory` tools for more detailed, real-time information.*\n\n")

	return builder.String()
}

// Helper methods

// getGitTool retrieves the git_info tool from the registry
func (ci *ContextIntegrator) getGitTool() agent.Tool {
	if ci.toolRegistry == nil {
		return nil
	}
	tool, exists := ci.toolRegistry.Get("git_info")
	if !exists {
		return nil
	}
	return tool
}

// getListDirectoryTool retrieves the list_directory tool from the registry
func (ci *ContextIntegrator) getListDirectoryTool() agent.Tool {
	if ci.toolRegistry == nil {
		return nil
	}
	tool, exists := ci.toolRegistry.Get("list_directory")
	if !exists {
		return nil
	}
	return tool
}

// fileExists checks if a file exists in the workspace
func (ci *ContextIntegrator) fileExists(relativePath string) bool {
	fullPath := filepath.Join(ci.workspaceRoot, relativePath)
	_, err := filepath.Abs(fullPath)
	return err == nil
}
