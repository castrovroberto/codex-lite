package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/castrovroberto/CGE/internal/agent"
)

func main() {
	// Get current working directory
	workspaceRoot, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get current directory: %v\n", err)
		return
	}

	fmt.Printf("Testing list_directory tool in: %s\n", workspaceRoot)

	// Create the tool
	tool := agent.NewListDirTool(workspaceRoot)

	// Test listing the internal directory
	params := map[string]interface{}{
		"directory_path": "internal",
		"recursive":      false,
		"include_hidden": false,
	}

	paramsJSON, _ := json.Marshal(params)
	result, err := tool.Execute(context.Background(), paramsJSON)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("Failed: %s\n", result.Error)
		return
	}

	data := result.Data.(map[string]interface{})
	fmt.Printf("Success! Message: %s\n", data["message"])

	// Check path resolution
	if pathRes, ok := data["path_resolution"]; ok {
		if pathData, ok := pathRes.(map[string]interface{}); ok {
			fmt.Printf("Path Resolution:\n")
			fmt.Printf("  Original: %v\n", pathData["original_path"])
			fmt.Printf("  Resolved: %v\n", pathData["resolved_path"])
			fmt.Printf("  In workspace: %v\n", pathData["is_in_workspace"])
			fmt.Printf("  Allowed by: %v\n", pathData["allowed_by_rule"])
		}
	}

	// Show some files
	if files, ok := data["files"].([]interface{}); ok {
		fmt.Printf("\nFound %d items in internal/:\n", len(files))
		for i, file := range files {
			if i >= 5 { // Show first 5 items
				fmt.Printf("  ... and %d more\n", len(files)-5)
				break
			}
			if fileData, ok := file.(map[string]interface{}); ok {
				name := fileData["name"]
				isDir := fileData["is_directory"]
				if isDir.(bool) {
					fmt.Printf("  📁 %s/\n", name)
				} else {
					fmt.Printf("  📄 %s\n", name)
				}
			}
		}
	}
}
