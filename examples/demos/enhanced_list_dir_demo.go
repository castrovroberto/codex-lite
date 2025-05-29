package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
)

func main() {
	// Load configuration
	if err := config.LoadConfig(""); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.GetConfig()

	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	fmt.Printf("🚀 Enhanced List Directory Tool Demo\n")
	fmt.Printf("Workspace: %s\n\n", workspaceRoot)

	// Demo 1: Basic directory listing with default configuration
	fmt.Println("📁 Demo 1: Basic directory listing (workspace root)")
	basicDemo(workspaceRoot)

	// Demo 2: Enhanced configuration with outside workspace access
	fmt.Println("\n📁 Demo 2: Enhanced configuration with custom settings")
	enhancedDemo(workspaceRoot, cfg.GetListDirectoryConfig())

	// Demo 3: Smart path resolution
	fmt.Println("\n📁 Demo 3: Smart path resolution")
	smartResolutionDemo(workspaceRoot)

	// Demo 4: Pattern filtering and advanced sorting
	fmt.Println("\n📁 Demo 4: Pattern filtering and sorting")
	patternDemo(workspaceRoot)

	// Demo 5: Configuration showcase
	fmt.Println("\n📁 Demo 5: Custom configuration showcase")
	configDemo(workspaceRoot)
}

func basicDemo(workspaceRoot string) {
	tool := agent.NewListDirTool(workspaceRoot)

	params := map[string]interface{}{
		"directory_path": ".",
		"recursive":      false,
		"include_hidden": false,
	}

	executeAndPrint(tool, params, "Basic listing of workspace root")
}

func enhancedDemo(workspaceRoot string, config agent.ListDirToolConfig) {
	// Enable outside workspace access for this demo
	config.AllowOutsideWorkspace = true
	config.AllowedRoots = []string{"/tmp", os.Getenv("HOME")}

	tool := agent.NewListDirToolWithConfig(workspaceRoot, config)

	// Try to list home directory
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		params := map[string]interface{}{
			"directory_path": homeDir,
			"recursive":      false,
			"include_hidden": false,
			"smart_resolve":  true,
		}

		executeAndPrint(tool, params, fmt.Sprintf("Listing home directory: %s", homeDir))
	}
}

func smartResolutionDemo(workspaceRoot string) {
	tool := agent.NewListDirTool(workspaceRoot)

	// Try smart resolution for common directory names
	commonDirs := []string{"docs", "src", "test", "config", "scripts", "internal"}

	for _, dir := range commonDirs {
		params := map[string]interface{}{
			"directory_path": dir,
			"recursive":      false,
			"smart_resolve":  true,
		}

		executeAndPrint(tool, params, fmt.Sprintf("Smart resolution for '%s'", dir))
	}
}

func patternDemo(workspaceRoot string) {
	tool := agent.NewListDirTool(workspaceRoot)

	demos := []struct {
		pattern string
		sortBy  string
		desc    string
	}{
		{"*.go", "size", "Go files sorted by size"},
		{"*.md", "modified", "Markdown files sorted by modification time"},
		{"*.toml", "name", "TOML files sorted by name"},
		{"*test*", "type_name", "Test-related files/dirs sorted by type then name"},
	}

	for _, demo := range demos {
		params := map[string]interface{}{
			"directory_path": ".",
			"recursive":      true,
			"pattern":        demo.pattern,
			"sort_by":        demo.sortBy,
			"max_depth":      2,
		}

		executeAndPrint(tool, params, demo.desc)
	}
}

func configDemo(workspaceRoot string) {
	// Create a custom configuration showcasing all features
	customConfig := agent.ListDirToolConfig{
		AllowOutsideWorkspace: false, // Keep secure for this demo
		AllowedRoots:          []string{},
		MaxDepthLimit:         5,
		MaxFilesLimit:         50, // Limit for demo purposes
		AutoResolveSymlinks:   true,
		SmartPathResolution:   true,
	}

	tool := agent.NewListDirToolWithConfig(workspaceRoot, customConfig)

	params := map[string]interface{}{
		"directory_path": ".",
		"recursive":      true,
		"include_hidden": true,
		"max_depth":      3,
		"pattern":        "*.go",
		"sort_by":        "modified",
		"smart_resolve":  true,
	}

	executeAndPrint(tool, params, "Custom configuration with all features enabled")
}

func executeAndPrint(tool agent.Tool, params map[string]interface{}, description string) {
	fmt.Printf("  %s:\n", description)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		fmt.Printf("    ❌ Error marshaling parameters: %v\n", err)
		return
	}

	result, err := tool.Execute(context.Background(), paramsJSON)
	if err != nil {
		fmt.Printf("    ❌ Error executing tool: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("    ❌ Tool execution failed: %s\n", result.Error)
		return
	}

	// Extract key information from the result
	data := result.Data.(map[string]interface{})

	fmt.Printf("    ✅ %s\n", data["message"])

	if pathRes, ok := data["path_resolution"]; ok {
		if pathData, ok := pathRes.(*agent.PathResolutionResult); ok {
			fmt.Printf("    🔍 Resolved path: %s\n", pathData.ResolvedPath)
			if pathData.AllowedByRule != "" {
				fmt.Printf("    🔐 Access rule: %s\n", pathData.AllowedByRule)
			}
		}
	}

	if truncated, ok := data["truncated"]; ok && truncated.(bool) {
		fmt.Printf("    ⚠️  Results truncated due to file limit\n")
	}

	if pattern, ok := data["pattern"]; ok && pattern != "" {
		fmt.Printf("    🔎 Pattern filter: %s\n", pattern)
	}

	fmt.Println()
}
