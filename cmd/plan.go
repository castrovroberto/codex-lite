package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/audit"
	"github.com/castrovroberto/CGE/internal/config"
	cgecontext "github.com/castrovroberto/CGE/internal/context"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
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
	userPromptPlan  string
	outputFilePlan  string
	useOrchestrator bool
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

		logger.Info("Starting plan generation...", "goal", userGoal, "use_orchestrator", useOrchestrator)

		// 1. Instantiate LLM Client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			ollamaConfig := cfg.GetOllamaConfig()
			llmClient = llm.NewOllamaClient(ollamaConfig)
			logger.Info("Using Ollama client", "host", ollamaConfig.HostURL)
		case "openai":
			openaiConfig := cfg.GetOpenAIConfig()
			llmClient = llm.NewOpenAIClient(openaiConfig)
			logger.Info("Using OpenAI client", "base_url", openaiConfig.BaseURL)
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
		gatherer := cgecontext.NewGatherer(workspaceRoot)
		contextInfo, err := gatherer.GatherContext()
		if err != nil {
			logger.Error("Failed to gather codebase context", "error", err)
			return fmt.Errorf("failed to gather codebase context: %w", err)
		}
		logger.Debug("Successfully gathered codebase context")

		// 3. Plan Generation Logic - choose between orchestrator and template
		if useOrchestrator {
			logger.Info("Generating plan with orchestrator...")
			return generatePlanWithOrchestrator(ctx, userGoal, contextInfo, llmClient, workspaceRoot, cfg, logger)
		}

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
			_ = os.WriteFile(rawPlanPath, []byte(llmResponse), 0600)
			logger.Info("Raw LLM response saved for debugging.", "path", rawPlanPath)
			return fmt.Errorf("failed to parse LLM JSON response: %w. Raw response saved to %s", err, rawPlanPath)
		}

		// Ensure the overall goal from user input is in the plan
		if generatedPlan.OverallGoal == "" {
			generatedPlan.OverallGoal = userGoal
		}

		// Validate the generated plan
		if err := validatePlan(&generatedPlan); err != nil {
			logger.Error("Generated plan failed validation", "error", err)
			return fmt.Errorf("generated plan is invalid: %w", err)
		}

		// 5. Output plan.json
		planJSON, err := json.MarshalIndent(generatedPlan, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal plan to JSON", "error", err)
			return fmt.Errorf("failed to marshal plan to JSON: %w", err)
		}

		if err := os.WriteFile(outputFilePlan, planJSON, 0600); err != nil {
			logger.Error("Failed to write plan to file", "path", outputFilePlan, "error", err)
			return fmt.Errorf("failed to write plan to %s: %w", outputFilePlan, err)
		}

		logger.Info("Plan generated successfully!", "path", outputFilePlan)
		fmt.Printf("Plan generated and saved to %s\n", outputFilePlan)
		return nil
	},
}

// validatePlan performs sanity checks on the generated plan
func validatePlan(plan *Plan) error {
	if plan.OverallGoal == "" {
		return fmt.Errorf("plan must have an overall goal")
	}

	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan must contain at least one task")
	}

	// Check each task
	taskIDs := make(map[string]bool)
	for i, task := range plan.Tasks {
		if task.ID == "" {
			return fmt.Errorf("task %d must have a non-empty ID", i+1)
		}

		if taskIDs[task.ID] {
			return fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDs[task.ID] = true

		if task.Description == "" {
			return fmt.Errorf("task %s must have a description", task.ID)
		}

		// Validate effort levels
		if task.EstimatedEffort != "" {
			validEfforts := map[string]bool{"small": true, "medium": true, "large": true}
			if !validEfforts[task.EstimatedEffort] {
				return fmt.Errorf("task %s has invalid effort level: %s (must be small, medium, or large)", task.ID, task.EstimatedEffort)
			}
		}
	}

	// Validate dependencies exist
	for _, task := range plan.Tasks {
		for _, dep := range task.Dependencies {
			if !taskIDs[dep] {
				return fmt.Errorf("task %s depends on non-existent task: %s", task.ID, dep)
			}
		}
	}

	return nil
}

// generatePlanWithOrchestrator uses the agent orchestrator to generate a plan
func generatePlanWithOrchestrator(ctx context.Context, userGoal string, contextInfo interface{}, llmClient llm.Client, workspaceRoot string, cfg interface{}, logger interface{}) error {
	// Initialize audit logger for session tracking
	auditLogger, err := audit.NewAuditLogger(workspaceRoot, "plan")
	if err != nil {
		// Continue without audit logging if it fails
		fmt.Printf("Warning: Failed to initialize audit logger: %v\n", err)
	}
	defer func() {
		if auditLogger != nil {
			auditLogger.Close()
		}
	}()

	// TODO: Integrate session manager with planning command in future iteration

	// Initialize tool registry with planning tools
	toolFactory := agent.NewToolFactory(workspaceRoot)
	toolRegistry := toolFactory.CreatePlanningRegistry()

	// Create command integrator and execute plan
	appCfg := cfg.(*config.AppConfig) // Type assertion needed
	integratorConfig := appCfg.GetIntegratorConfig()
	integrator := orchestrator.NewCommandIntegrator(llmClient, toolRegistry, integratorConfig)

	planRequest := &orchestrator.PlanRequest{
		UserGoal:        userGoal,
		Model:           cfg.(*config.AppConfig).LLM.Model, // Type assertion needed
		CodebaseContext: contextInfo,
	}

	// Log the planning session start
	fmt.Printf("Starting planning session...\n")

	planResponse, err := integrator.ExecutePlan(ctx, planRequest)
	if err != nil {
		return fmt.Errorf("orchestrated planning failed: %w", err)
	}

	if !planResponse.Success {
		return fmt.Errorf("planning was not successful")
	}

	// Validate and save the plan
	planJSON, err := json.MarshalIndent(planResponse.Plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan to JSON: %w", err)
	}

	// Validate the plan structure
	var generatedPlan Plan
	if err := json.Unmarshal(planJSON, &generatedPlan); err != nil {
		// Save raw response for debugging
		rawPlanPath := "failed_orchestrated_plan_raw_output.txt"
		_ = os.WriteFile(rawPlanPath, planJSON, 0600)
		return fmt.Errorf("generated plan has invalid structure: %w. Raw response saved to %s", err, rawPlanPath)
	}

	// Ensure the overall goal is set
	if generatedPlan.OverallGoal == "" {
		generatedPlan.OverallGoal = userGoal
	}

	// Validate the plan
	if err := validatePlan(&generatedPlan); err != nil {
		return fmt.Errorf("generated plan is invalid: %w", err)
	}

	// Re-marshal with any corrections
	finalPlanJSON, err := json.MarshalIndent(generatedPlan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal final plan to JSON: %w", err)
	}

	// Save plan to file
	if err := os.WriteFile(outputFilePlan, finalPlanJSON, 0600); err != nil {
		return fmt.Errorf("failed to write plan to %s: %w", outputFilePlan, err)
	}

	// Save conversation history for debugging
	historyPath := filepath.Join(filepath.Dir(outputFilePlan), "plan_conversation_history.json")
	historyJSON, _ := json.MarshalIndent(planResponse.Messages, "", "  ")
	_ = os.WriteFile(historyPath, historyJSON, 0600)

	fmt.Printf("Orchestrated plan generated and saved to %s\n", outputFilePlan)
	fmt.Printf("Conversation history saved to %s\n", historyPath)

	return nil
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVarP(&outputFilePlan, "output", "o", "plan.json", "Output file for the generated plan")
	planCmd.Flags().BoolVar(&useOrchestrator, "use-orchestrator", false, "Use the agent orchestrator with function calling")
	// We are taking the prompt as a positional arg now.
	// planCmd.Flags().StringVarP(&userPromptPlan, "prompt", "p", "", "Your goal or task description (required)")
	// planCmd.MarkFlagRequired("prompt")
}
