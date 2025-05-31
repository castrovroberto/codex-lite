package context

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/castrovroberto/CGE/internal/analyzer"
	"github.com/castrovroberto/CGE/internal/scanner"
)

// Gatherer collects codebase context information
type Gatherer struct {
	workspaceRoot string
	scanner       *scanner.Scanner
}

// NewGatherer creates a new context gatherer
func NewGatherer(workspaceRoot string) *Gatherer {
	return &Gatherer{
		workspaceRoot: workspaceRoot,
		scanner:       scanner.NewScanner(scanner.DefaultOptions()),
	}
}

// GatherContext collects comprehensive codebase context
func (g *Gatherer) GatherContext() (*ContextInfo, error) {
	info := &ContextInfo{}

	// Gather codebase analysis
	codebaseInfo, err := analyzer.AnalyzeCodebase(g.workspaceRoot, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze codebase: %w", err)
	}
	info.CodebaseAnalysis = codebaseInfo.FormatAnalysis()

	// Gather file structure
	fileStructure, err := g.gatherFileStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to gather file structure: %w", err)
	}
	info.FileStructure = fileStructure

	// Gather Git information
	gitInfo, err := g.gatherGitInfo()
	if err != nil {
		// Git info is optional, don't fail if not available
		info.GitInfo = "Not a Git repository or Git not available"
	} else {
		info.GitInfo = gitInfo
	}

	// Gather dependencies
	dependencies, err := g.gatherDependencies()
	if err != nil {
		// Dependencies are optional
		info.Dependencies = "No dependency files found or analysis failed"
	} else {
		info.Dependencies = dependencies
	}

	return info, nil
}

// ContextInfo holds all gathered context information
type ContextInfo struct {
	CodebaseAnalysis string
	FileStructure    string
	GitInfo          string
	Dependencies     string
}

// gatherFileStructure creates a summary of the project structure
func (g *Gatherer) gatherFileStructure() (string, error) {
	results, err := g.scanner.Scan(g.workspaceRoot)
	if err != nil {
		return "", err
	}

	var structure strings.Builder
	structure.WriteString("📁 Project Structure:\n")

	// Group files by directory
	dirFiles := make(map[string][]string)
	for _, result := range results {
		relPath, err := filepath.Rel(g.workspaceRoot, result.Path)
		if err != nil {
			relPath = result.Path
		}

		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = "root"
		}

		fileName := filepath.Base(relPath)
		dirFiles[dir] = append(dirFiles[dir], fileName)
	}

	// Format structure
	for dir, files := range dirFiles {
		structure.WriteString(fmt.Sprintf("\n📂 %s/\n", dir))
		for _, file := range files {
			structure.WriteString(fmt.Sprintf("  📄 %s\n", file))
		}
	}

	return structure.String(), nil
}

// gatherGitInfo collects Git repository information
func (g *Gatherer) gatherGitInfo() (string, error) {
	var info strings.Builder

	// Check if it's a Git repository
	if !g.isGitRepo() {
		return "Not a Git repository", nil
	}

	info.WriteString("🔄 Git Repository Information:\n")

	// Get current branch
	if branch, err := g.getCurrentBranch(); err == nil {
		info.WriteString(fmt.Sprintf("- Current branch: %s\n", branch))
	}

	// Get repository status
	if status, err := g.getGitStatus(); err == nil {
		info.WriteString(fmt.Sprintf("- Repository status: %s\n", status))
	}

	// Get recent commits
	if commits, err := g.getRecentCommits(3); err == nil {
		info.WriteString("- Recent commits:\n")
		for _, commit := range commits {
			info.WriteString(fmt.Sprintf("  • %s: %s\n", commit["hash"], commit["message"]))
		}
	}

	return info.String(), nil
}

// gatherDependencies analyzes project dependencies
func (g *Gatherer) gatherDependencies() (string, error) {
	deps, err := analyzer.AnalyzeDependencies(g.workspaceRoot)
	if err != nil {
		return "", err
	}

	if len(deps) == 0 {
		return "No dependency files found", nil
	}

	return analyzer.FormatDependencyAnalysis(deps), nil
}

// Helper methods for Git operations
func (g *Gatherer) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = g.workspaceRoot
	return cmd.Run() == nil
}

func (g *Gatherer) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = g.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *Gatherer) getGitStatus() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	if len(out) == 0 {
		return "clean", nil
	}
	return "has uncommitted changes", nil
}

func (g *Gatherer) getRecentCommits(count int) ([]map[string]string, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%h|%s", fmt.Sprintf("-%d", count))
	cmd.Dir = g.workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			commits = append(commits, map[string]string{
				"hash":    parts[0],
				"message": parts[1],
			})
		}
	}

	return commits, nil
}
