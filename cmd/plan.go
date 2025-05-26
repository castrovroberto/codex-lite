package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castrovroberto/CGE/internal/context"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/templates"
	"github.com/spf13/cobra"
)

// PlanTask represents a single task in the generated plan.
type PlanTask struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	FilesToModify   []string `json:"files_to_modify,omitempty"`
	FilesToCreate   []string `json:"files_to_create,omitempty"`
	FilesToDelete   []string `json:"files_to_delete,omitempty"`
	EstimatedEffort string   `json:"estimated_effort,omitempty"` // e.g., "small", "medium", "large"
	Dependencies    []string `json:"dependencies,omitempty"`
	Rationale       string   `json:"rationale,omitempty"`
}

// Plan represents the structure of the plan.json file.
type Plan struct {
	OverallGoal            string     `json:"overall_goal"`
	Tasks                  []PlanTask `json:"tasks"`
	Summary                string     `json:"summary,omitempty"`
	EstimatedTotalEffort   string     `json:"estimated_total_effort,omitempty"`
	RisksAndConsiderations []string   `json:"risks_and_considerations,omitempty"`
}

var (
	userPromptPlan string
	outputFilePlan string
)

// planCmd represents the plan command
var planCmd = &cobra.Command{
	Use:   "plan \"<your goal or task description>\"",
	Short: "Generates a development plan based on your input and codebase context.",
	Long: `The plan command interacts with an LLM to analyze your request and
the current project context (e.g., files in the repository) to produce a
step-by-step plan in JSON format.

This plan can then be used by the 'generate' command.

Example:
  CGE plan "Refactor the user authentication module to use JWT" --output plan_auth_refactor.json`,
	Args: cobra.ExactArgs(1), // Expects the main goal as an argument
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger := contextkeys.LoggerFromContext(ctx)
		cfg := contextkeys.ConfigFromContext(ctx)

		userGoal := args[0]
		if userGoal == "" {
			return fmt.Errorf("the goal description cannot be empty")
		}

		logger.Info("Starting plan generation...", "goal", userGoal)

		// 1. Instantiate LLM Client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			llmClient = llm.NewOllamaClient() // Assuming constructor exists
			logger.Info("Using Ollama client", "host", cfg.LLM.OllamaHostURL)
		// case "openai":
		// llmClient = llm.NewOpenAIClient(cfg.LLM.OpenAIAPIKey) // Example
		// logger.Info("Using OpenAI client")
		default:
			return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
		}

		// 2. Repository Walker & Context Gathering
		logger.Info("Gathering codebase context...")

		// Get workspace root from config or current directory
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Gather real codebase context
		gatherer := context.NewGatherer(workspaceRoot)
		contextInfo, err := gatherer.GatherContext()
		if err != nil {
			logger.Error("Failed to gather codebase context", "error", err)
			return fmt.Errorf("failed to gather codebase context: %w", err)
		}
		logger.Debug("Successfully gathered codebase context")

		// 3. Plan Generation Logic using template
		logger.Info("Generating plan with template...")

		// Get prompts directory (relative to workspace root)
		promptsDir := filepath.Join(workspaceRoot, "prompts")
		templateEngine := templates.NewEngine(promptsDir)

		// Prepare template data
		templateData := templates.PlanTemplateData{
			UserGoal:        userGoal,
			CodebaseContext: contextInfo.CodebaseAnalysis,
			GitInfo:         contextInfo.GitInfo,
			FileStructure:   contextInfo.FileStructure,
			Dependencies:    contextInfo.Dependencies,
		}

		// Render the prompt template
		fullPrompt, err := templateEngine.Render("plan.tmpl", templateData)
		if err != nil {
			logger.Error("Failed to render plan template", "error", err)
			return fmt.Errorf("failed to render plan template: %w", err)
		}

		systemPrompt := "You are an expert software architect and project planner. Respond only with valid JSON."

		logger.Info("Generating plan with LLM...", "model", cfg.LLM.Model)
		llmResponse, err := llmClient.Generate(ctx, cfg.LLM.Model, fullPrompt, systemPrompt, nil)
		if err != nil {
			logger.Error("Failed to generate plan from LLM", "error", err)
			return fmt.Errorf("LLM generation failed: %w", err)
		}
		logger.Debug("LLM Raw Response:", "response", llmResponse)

		// 4. Parse LLM's JSON response
		var generatedPlan Plan
		if err := json.Unmarshal([]byte(llmResponse), &generatedPlan); err != nil {
			logger.Error("Failed to parse LLM response into Plan struct", "error", err, "response", llmResponse)
			// Attempt to save the raw response for debugging if JSON parsing fails
			rawPlanPath := "failed_plan_raw_output.txt"
			_ = os.WriteFile(rawPlanPath, []byte(llmResponse), 0644)
			logger.Info("Raw LLM response saved for debugging.", "path", rawPlanPath)
			return fmt.Errorf("failed to parse LLM JSON response: %w. Raw response saved to %s", err, rawPlanPath)
		}

		// Ensure the overall goal from user input is in the plan
		if generatedPlan.OverallGoal == "" {
			generatedPlan.OverallGoal = userGoal
		}

		// 5. Output plan.json
		planJSON, err := json.MarshalIndent(generatedPlan, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal plan to JSON", "error", err)
			return fmt.Errorf("failed to marshal plan to JSON: %w", err)
		}

		if err := os.WriteFile(outputFilePlan, planJSON, 0644); err != nil {
			logger.Error("Failed to write plan to file", "path", outputFilePlan, "error", err)
			return fmt.Errorf("failed to write plan to %s: %w", outputFilePlan, err)
		}

		logger.Info("Plan generated successfully!", "path", outputFilePlan)
		fmt.Printf("Plan generated and saved to %s\n", outputFilePlan)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVarP(&outputFilePlan, "output", "o", "plan.json", "Output file for the generated plan")
	// We are taking the prompt as a positional arg now.
	// planCmd.Flags().StringVarP(&userPromptPlan, "prompt", "p", "", "Your goal or task description (required)")
	// planCmd.MarkFlagRequired("prompt")
}
