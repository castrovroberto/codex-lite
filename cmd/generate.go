package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/templates"
	"github.com/spf13/cobra"
)

var (
	planFilePath string
	dryRun       bool
	applyChanges bool
	outputDir    string
	taskFilter   string
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code based on a development plan",
	Long: `The generate command reads a plan.json file (created by the 'plan' command)
and uses an LLM to generate code changes for each task in the plan.

You can run in different modes:
- Dry run: Preview what changes would be made without applying them
- Apply: Directly apply changes to the codebase
- Output: Save generated diffs to a specified directory

Example:
  CGE generate --plan plan.json --dry-run
  CGE generate --plan plan.json --apply
  CGE generate --plan plan.json --output-dir ./generated_changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger := contextkeys.LoggerFromContext(ctx)
		cfg := contextkeys.ConfigFromContext(ctx)

		logger.Info("Starting code generation...", "plan_file", planFilePath)

		// 1. Read and parse plan.json
		plan, err := readPlan(planFilePath)
		if err != nil {
			logger.Error("Failed to read plan file", "error", err)
			return fmt.Errorf("failed to read plan file: %w", err)
		}

		logger.Info("Loaded plan", "tasks", len(plan.Tasks), "goal", plan.OverallGoal)

		// 2. Initialize LLM client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			llmClient = llm.NewOllamaClient()
			logger.Info("Using Ollama client", "host", cfg.LLM.OllamaHostURL)
		default:
			return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
		}

		// 3. Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// 4. Initialize template engine
		promptsDir := filepath.Join(workspaceRoot, "prompts")
		templateEngine := templates.NewEngine(promptsDir)

		// 5. Process each task
		var processedTasks []string
		for _, task := range plan.Tasks {
			// Skip if task filter is specified and doesn't match
			if taskFilter != "" && !strings.Contains(task.ID, taskFilter) {
				continue
			}

			logger.Info("Processing task", "id", task.ID, "description", task.Description)

			// Check dependencies
			if !areTaskDependenciesMet(task.Dependencies, processedTasks) {
				logger.Warn("Skipping task due to unmet dependencies", "id", task.ID, "dependencies", task.Dependencies)
				continue
			}

			// Generate code for this task
			err := processTask(ctx, task, plan, llmClient, templateEngine, workspaceRoot, cfg, logger)
			if err != nil {
				logger.Error("Failed to process task", "id", task.ID, "error", err)
				if !dryRun {
					return fmt.Errorf("failed to process task %s: %w", task.ID, err)
				}
				// In dry-run mode, continue with other tasks
				continue
			}

			processedTasks = append(processedTasks, task.ID)
			logger.Info("Successfully processed task", "id", task.ID)
		}

		logger.Info("Code generation completed", "processed_tasks", len(processedTasks))
		return nil
	},
}

// readPlan reads and parses a plan.json file
func readPlan(filePath string) (*Plan, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return &plan, nil
}

// areTaskDependenciesMet checks if all dependencies for a task have been processed
func areTaskDependenciesMet(dependencies, processedTasks []string) bool {
	for _, dep := range dependencies {
		found := false
		for _, processed := range processedTasks {
			if processed == dep {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// processTask generates code for a single task
func processTask(ctx context.Context, task PlanTask, plan *Plan, llmClient llm.Client, templateEngine *templates.Engine, workspaceRoot string, cfg interface{}, logger interface{}) error {
	fmt.Printf("\n=== Processing Task: %s ===\n", task.ID)
	fmt.Printf("Description: %s\n", task.Description)
	fmt.Printf("Files to modify: %v\n", task.FilesToModify)
	fmt.Printf("Files to create: %v\n", task.FilesToCreate)
	fmt.Printf("Files to delete: %v\n", task.FilesToDelete)
	fmt.Printf("Estimated effort: %s\n", task.EstimatedEffort)

	if dryRun {
		fmt.Printf("DRY RUN: Would generate code for this task\n")
		return nil
	}

	// 1. Read current file contents for files to modify
	currentFileContents := ""
	for _, filePath := range task.FilesToModify {
		fullPath := filepath.Join(workspaceRoot, filePath)
		if content, err := os.ReadFile(fullPath); err == nil {
			currentFileContents += fmt.Sprintf("=== %s ===\n%s\n\n", filePath, string(content))
		} else {
			currentFileContents += fmt.Sprintf("=== %s ===\n(File not found or unreadable)\n\n", filePath)
		}
	}

	// 2. Gather project context
	projectContext := fmt.Sprintf("Workspace: %s\nOverall Goal: %s", workspaceRoot, plan.OverallGoal)

	// 3. Prepare template data
	templateData := templates.GenerateTemplateData{
		TaskID:              task.ID,
		TaskDescription:     task.Description,
		EstimatedEffort:     task.EstimatedEffort,
		Rationale:           task.Rationale,
		OverallGoal:         plan.OverallGoal,
		FilesToModify:       task.FilesToModify,
		FilesToCreate:       task.FilesToCreate,
		FilesToDelete:       task.FilesToDelete,
		CurrentFileContents: currentFileContents,
		ProjectContext:      projectContext,
	}

	// 4. Render the prompt template
	fullPrompt, err := templateEngine.Render("generate.tmpl", templateData)
	if err != nil {
		return fmt.Errorf("failed to render generate template: %w", err)
	}

	// 5. Call LLM to generate code
	systemPrompt := "You are an expert software engineer. Generate precise code changes in the specified JSON format."

	// Type assertion to get the config - we'll use a more flexible approach
	// Since we can't easily type assert the complex config structure,
	// we'll use reflection or a simpler approach
	model := "llama3.2" // Default model, should come from config

	// Try to extract model from config if possible
	if cfgMap, ok := cfg.(map[string]interface{}); ok {
		if llmCfg, exists := cfgMap["LLM"]; exists {
			if llmMap, ok := llmCfg.(map[string]interface{}); ok {
				if modelVal, exists := llmMap["Model"]; exists {
					if modelStr, ok := modelVal.(string); ok {
						model = modelStr
					}
				}
			}
		}
	}

	llmResponse, err := llmClient.Generate(ctx, model, fullPrompt, systemPrompt, nil)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	// 6. Parse LLM response
	var response struct {
		Changes []struct {
			Action   string `json:"action"`
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
			Diff     string `json:"diff"`
			Reason   string `json:"reason"`
		} `json:"changes"`
		Summary      string   `json:"summary"`
		Notes        []string `json:"notes"`
		TestsNeeded  []string `json:"tests_needed"`
		Dependencies []string `json:"dependencies"`
	}

	if err := json.Unmarshal([]byte(llmResponse), &response); err != nil {
		// Save raw response for debugging
		rawPath := fmt.Sprintf("failed_generate_task_%s_raw.txt", task.ID)
		_ = os.WriteFile(rawPath, []byte(llmResponse), 0600)
		return fmt.Errorf("failed to parse LLM JSON response: %w. Raw response saved to %s", err, rawPath)
	}

	// 7. Apply changes based on mode
	if applyChanges {
		return applyChangesToFiles(response.Changes, workspaceRoot)
	} else if outputDir != "" {
		return saveChangesToOutputDir(response.Changes, outputDir, task.ID)
	}

	// Default: just print what would be done
	fmt.Printf("Generated %d changes for task %s\n", len(response.Changes), task.ID)
	fmt.Printf("Summary: %s\n", response.Summary)

	// Print additional information
	if len(response.Notes) > 0 {
		fmt.Printf("\n📝 Notes:\n")
		for _, note := range response.Notes {
			fmt.Printf("  - %s\n", note)
		}
	}

	if len(response.TestsNeeded) > 0 {
		fmt.Printf("\n🧪 Tests needed:\n")
		for _, test := range response.TestsNeeded {
			fmt.Printf("  - %s\n", test)
		}
	}

	if len(response.Dependencies) > 0 {
		fmt.Printf("\n📦 Dependencies to add:\n")
		for _, dep := range response.Dependencies {
			fmt.Printf("  - %s\n", dep)
		}
	}

	return nil
}

// applyChangesToFiles applies the generated changes directly to the filesystem
func applyChangesToFiles(changes []struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Diff     string `json:"diff"`
	Reason   string `json:"reason"`
}, workspaceRoot string) error {
	// Create backups for rollback capability
	backups := make(map[string][]byte)

	// First pass: create backups
	for _, change := range changes {
		if change.Action == "modify" || change.Action == "delete" {
			fullPath := filepath.Join(workspaceRoot, change.FilePath)
			if content, err := os.ReadFile(fullPath); err == nil {
				backups[change.FilePath] = content
			}
		}
	}

	// Second pass: apply changes
	appliedChanges := []string{}
	for _, change := range changes {
		fullPath := filepath.Join(workspaceRoot, change.FilePath)

		switch change.Action {
		case "create", "modify":
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
				// Rollback on error
				rollbackChanges(backups, appliedChanges, workspaceRoot)
				return fmt.Errorf("failed to create directory for %s: %w", change.FilePath, err)
			}

			// Write the file content
			if err := os.WriteFile(fullPath, []byte(change.Content), 0600); err != nil {
				// Rollback on error
				rollbackChanges(backups, appliedChanges, workspaceRoot)
				return fmt.Errorf("failed to write file %s: %w", change.FilePath, err)
			}

			appliedChanges = append(appliedChanges, change.FilePath)
			fmt.Printf("✅ %s: %s (%s)\n", strings.ToUpper(change.Action), change.FilePath, change.Reason)

		case "delete":
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				// Rollback on error
				rollbackChanges(backups, appliedChanges, workspaceRoot)
				return fmt.Errorf("failed to delete file %s: %w", change.FilePath, err)
			}

			appliedChanges = append(appliedChanges, change.FilePath)
			fmt.Printf("🗑️  DELETED: %s (%s)\n", change.FilePath, change.Reason)

		default:
			fmt.Printf("⚠️  Unknown action '%s' for file %s\n", change.Action, change.FilePath)
		}
	}

	fmt.Printf("\n✅ Successfully applied %d changes\n", len(appliedChanges))
	return nil
}

// rollbackChanges restores files from backups in case of errors
func rollbackChanges(backups map[string][]byte, appliedChanges []string, workspaceRoot string) {
	fmt.Printf("\n⚠️  Error occurred, rolling back changes...\n")

	for _, filePath := range appliedChanges {
		fullPath := filepath.Join(workspaceRoot, filePath)

		if backup, exists := backups[filePath]; exists {
			// Restore from backup
			if err := os.WriteFile(fullPath, backup, 0600); err != nil {
				fmt.Printf("❌ Failed to restore %s: %v\n", filePath, err)
			} else {
				fmt.Printf("🔄 Restored %s\n", filePath)
			}
		} else {
			// File was created, so delete it
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("❌ Failed to remove created file %s: %v\n", filePath, err)
			} else {
				fmt.Printf("🗑️  Removed created file %s\n", filePath)
			}
		}
	}
}

// saveChangesToOutputDir saves the generated changes to a specified output directory
func saveChangesToOutputDir(changes []struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Diff     string `json:"diff"`
	Reason   string `json:"reason"`
}, outputDir, taskID string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save changes as JSON
	changesJSON, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal changes: %w", err)
	}

	changesFile := filepath.Join(outputDir, fmt.Sprintf("task_%s_changes.json", taskID))
	if err := os.WriteFile(changesFile, changesJSON, 0600); err != nil {
		return fmt.Errorf("failed to write changes file: %w", err)
	}

	// Save individual files
	for _, change := range changes {
		if change.Action == "create" || change.Action == "modify" {
			// Create a safe filename
			safeFileName := strings.ReplaceAll(change.FilePath, "/", "_")
			outputFile := filepath.Join(outputDir, fmt.Sprintf("task_%s_%s_%s", taskID, change.Action, safeFileName))

			if err := os.WriteFile(outputFile, []byte(change.Content), 0600); err != nil {
				return fmt.Errorf("failed to write output file %s: %w", outputFile, err)
			}
		}
	}

	fmt.Printf("💾 Changes saved to %s\n", outputDir)
	return nil
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&planFilePath, "plan", "p", "plan.json", "Path to the plan.json file")
	generateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	generateCmd.Flags().BoolVar(&applyChanges, "apply", false, "Apply changes directly to the codebase")
	generateCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Directory to save generated changes (if not applying directly)")
	generateCmd.Flags().StringVar(&taskFilter, "task", "", "Filter to process only tasks containing this string")

	// Make the flags mutually exclusive
	generateCmd.MarkFlagsMutuallyExclusive("dry-run", "apply")
}
