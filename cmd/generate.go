package cmd

import (
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
func processTask(ctx interface{}, task PlanTask, plan *Plan, llmClient llm.Client, templateEngine *templates.Engine, workspaceRoot string, cfg interface{}, logger interface{}) error {
	// This is a simplified implementation - in a real scenario, you'd want to:
	// 1. Read the current state of files that need to be modified
	// 2. Generate appropriate prompts for code generation
	// 3. Parse LLM responses to extract code changes
	// 4. Apply changes or save diffs based on the mode

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

	// TODO: Implement actual code generation logic
	// This would involve:
	// - Creating prompts for code generation
	// - Calling the LLM to generate code
	// - Parsing the response to extract diffs
	// - Applying changes or saving to output directory

	fmt.Printf("Code generation not yet implemented for task %s\n", task.ID)
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
