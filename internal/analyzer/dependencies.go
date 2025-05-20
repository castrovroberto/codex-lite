package analyzer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DependencyInfo holds information about project dependencies
type DependencyInfo struct {
	Type         string                 // Type of dependency file (package.json, go.mod, etc.)
	Path         string                 // Path to the dependency file
	Dependencies map[string]interface{} // Parsed dependencies
}

// Common dependency file patterns
var dependencyFiles = map[string]struct {
	pattern  string
	parser   func([]byte) (map[string]interface{}, error)
	depTypes []string // Which fields to look for dependencies
}{
	"npm": {
		pattern: "package.json",
		parser:  parseJSON,
		depTypes: []string{
			"dependencies",
			"devDependencies",
			"peerDependencies",
			"optionalDependencies",
		},
	},
	"go": {
		pattern: "go.mod",
		parser:  parseGoMod,
		depTypes: []string{
			"require",
			"replace",
		},
	},
	"python": {
		pattern: "requirements.txt",
		parser:  parsePythonReqs,
		depTypes: []string{
			"requirements",
		},
	},
	"ruby": {
		pattern: "Gemfile",
		parser:  parseGemfile,
		depTypes: []string{
			"gems",
		},
	},
	"composer": {
		pattern: "composer.json",
		parser:  parseJSON,
		depTypes: []string{
			"require",
			"require-dev",
		},
	},
	"cargo": {
		pattern: "Cargo.toml",
		parser:  parseYAML,
		depTypes: []string{
			"dependencies",
			"dev-dependencies",
		},
	},
}

// AnalyzeDependencies scans the codebase for dependency files
func AnalyzeDependencies(rootPath string) ([]*DependencyInfo, error) {
	var results []*DependencyInfo

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip common vendor/dependency directories
			if IsSkippableDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check each dependency file pattern
		for depType, config := range dependencyFiles {
			if strings.HasSuffix(path, config.pattern) {
				content, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", path, err)
				}

				deps, err := config.parser(content)
				if err != nil {
					return fmt.Errorf("failed to parse %s: %w", path, err)
				}

				// Filter to only include dependency fields
				filteredDeps := make(map[string]interface{})
				for _, field := range config.depTypes {
					if val, ok := deps[field]; ok {
						filteredDeps[field] = val
					}
				}

				results = append(results, &DependencyInfo{
					Type:         depType,
					Path:         path,
					Dependencies: filteredDeps,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	return results, nil
}

// Parser functions for different dependency file formats

func parseJSON(content []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseYAML(content []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseGoMod(content []byte) (map[string]interface{}, error) {
	// Simple parser for go.mod files
	lines := strings.Split(string(content), "\n")
	result := make(map[string]interface{})
	var requires []string
	var replaces []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require ") {
			req := strings.TrimPrefix(line, "require ")
			requires = append(requires, req)
		} else if strings.HasPrefix(line, "replace ") {
			rep := strings.TrimPrefix(line, "replace ")
			replaces = append(replaces, rep)
		}
	}

	if len(requires) > 0 {
		result["require"] = requires
	}
	if len(replaces) > 0 {
		result["replace"] = replaces
	}

	return result, nil
}

func parsePythonReqs(content []byte) (map[string]interface{}, error) {
	// Simple parser for requirements.txt
	lines := strings.Split(string(content), "\n")
	var reqs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			reqs = append(reqs, line)
		}
	}

	return map[string]interface{}{
		"requirements": reqs,
	}, nil
}

func parseGemfile(content []byte) (map[string]interface{}, error) {
	// Simple parser for Gemfile
	lines := strings.Split(string(content), "\n")
	var gems []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gem ") {
			gems = append(gems, strings.TrimPrefix(line, "gem "))
		}
	}

	return map[string]interface{}{
		"gems": gems,
	}, nil
}

// FormatDependencyAnalysis returns a human-readable summary of dependencies
func FormatDependencyAnalysis(deps []*DependencyInfo) string {
	var b strings.Builder

	if len(deps) == 0 {
		b.WriteString("📦 No dependency files found\n")
		return b.String()
	}

	b.WriteString("📦 Dependency Analysis:\n\n")

	for _, dep := range deps {
		// Write dependency file info
		relPath, err := filepath.Rel(".", dep.Path)
		if err != nil {
			relPath = dep.Path
		}
		b.WriteString(fmt.Sprintf("📄 %s (%s):\n", relPath, dep.Type))

		// Write dependencies by type
		for depType, deps := range dep.Dependencies {
			b.WriteString(fmt.Sprintf("  %s:\n", depType))
			switch v := deps.(type) {
			case []string:
				for _, d := range v {
					b.WriteString(fmt.Sprintf("    - %s\n", d))
				}
			case map[string]interface{}:
				for name, version := range v {
					b.WriteString(fmt.Sprintf("    - %s: %v\n", name, version))
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
