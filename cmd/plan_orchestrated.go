package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/context"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
	"github.com/spf13/cobra"
)

var (
	userPromptPlanOrchestrated string
	outputFilePlanOrchestrated string
	useOrchestratorPlan        bool
)

// planOrchestratedCmd represents the orchestrated plan command
var planOrchestratedCmd = &cobra.Command{
	Use:   "plan-orchestrated \"<your goal or task description>\"",
	Short: "Generates a development plan using the agent orchestrator (function-calling approach)",
	Long: `The plan-orchestrated command uses the new agent orchestrator to analyze your request and
the current project context using function calls to produce a step-by-step plan in JSON format.

This version uses the agent orchestrator with function calling capabilities, allowing the AI to:
- Explore the codebase structure using tools
- Read specific files for context
- Gather comprehensive information before planning

Example:
  CGE plan-orchestrated "Refactor the user authentication module to use JWT" --output plan_auth_refactor.json`,
	Args: cobra.ExactArgs(1), // Expects the main goal as an argument
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger := contextkeys.LoggerFromContext(ctx)
		cfg := contextkeys.ConfigFromContext(ctx)

		userGoal := args[0]
		if userGoal == "" {
			return fmt.Errorf("the goal description cannot be empty")
		}

		logger.Info("Starting orchestrated plan generation...", "goal", userGoal)

		// 1. Instantiate LLM Client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			llmClient = llm.NewOllamaClient()
			logger.Info("Using Ollama client", "host", cfg.LLM.OllamaHostURL)
		default:
			return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
		}

		// 2. Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// 3. Initialize tool registry with planning tools
		toolFactory := agent.NewToolFactory(workspaceRoot)
		toolRegistry := toolFactory.CreatePlanningRegistry()

		// 4. Gather initial codebase context (lightweight)
		logger.Info("Gathering initial codebase context...")
		gatherer := context.NewGatherer(workspaceRoot)
		contextInfo, err := gatherer.GatherContext()
		if err != nil {
			logger.Error("Failed to gather codebase context", "error", err)
			return fmt.Errorf("failed to gather codebase context: %w", err)
		}
		logger.Debug("Successfully gathered initial codebase context")

		// 5. Create command integrator and execute plan
		integrator := orchestrator.NewCommandIntegrator(llmClient, toolRegistry, workspaceRoot)

		planRequest := &orchestrator.PlanRequest{
			UserGoal:        userGoal,
			Model:           cfg.LLM.Model,
			CodebaseContext: contextInfo,
		}

		logger.Info("Executing orchestrated planning...", "model", cfg.LLM.Model)
		planResponse, err := integrator.ExecutePlan(ctx, planRequest)
		if err != nil {
			logger.Error("Orchestrated planning failed", "error", err)
			return fmt.Errorf("orchestrated planning failed: %w", err)
		}

		if !planResponse.Success {
			logger.Error("Planning was not successful")
			return fmt.Errorf("planning was not successful")
		}

		// 6. Validate and save the plan
		planJSON, err := json.MarshalIndent(planResponse.Plan, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal plan to JSON", "error", err)
			return fmt.Errorf("failed to marshal plan to JSON: %w", err)
		}

		// Validate the plan structure
		var generatedPlan Plan
		if err := json.Unmarshal(planJSON, &generatedPlan); err != nil {
			logger.Error("Generated plan has invalid structure", "error", err)
			// Save raw response for debugging
			rawPlanPath := "failed_orchestrated_plan_raw_output.txt"
			_ = os.WriteFile(rawPlanPath, planJSON, 0600)
			logger.Info("Raw plan response saved for debugging.", "path", rawPlanPath)
			return fmt.Errorf("generated plan has invalid structure: %w. Raw response saved to %s", err, rawPlanPath)
		}

		// Ensure the overall goal is set
		if generatedPlan.OverallGoal == "" {
			generatedPlan.OverallGoal = userGoal
		}

		// Validate the plan
		if err := validatePlan(&generatedPlan); err != nil {
			logger.Error("Generated plan failed validation", "error", err)
			return fmt.Errorf("generated plan is invalid: %w", err)
		}

		// Re-marshal with any corrections
		finalPlanJSON, err := json.MarshalIndent(generatedPlan, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal final plan to JSON", "error", err)
			return fmt.Errorf("failed to marshal final plan to JSON: %w", err)
		}

		// 7. Save plan to file
		if err := os.WriteFile(outputFilePlanOrchestrated, finalPlanJSON, 0600); err != nil {
			logger.Error("Failed to write plan to file", "path", outputFilePlanOrchestrated, "error", err)
			return fmt.Errorf("failed to write plan to %s: %w", outputFilePlanOrchestrated, err)
		}

		// 8. Save conversation history for debugging
		historyPath := filepath.Join(filepath.Dir(outputFilePlanOrchestrated), "plan_conversation_history.json")
		historyJSON, _ := json.MarshalIndent(planResponse.Messages, "", "  ")
		_ = os.WriteFile(historyPath, historyJSON, 0600)

		logger.Info("Orchestrated plan generated successfully!", "path", outputFilePlanOrchestrated, "tool_calls", len(planResponse.Messages))
		fmt.Printf("Orchestrated plan generated and saved to %s\n", outputFilePlanOrchestrated)
		fmt.Printf("Conversation history saved to %s\n", historyPath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(planOrchestratedCmd)

	planOrchestratedCmd.Flags().StringVarP(&outputFilePlanOrchestrated, "output", "o", "plan.json", "Output file for the generated plan")
	planOrchestratedCmd.Flags().BoolVar(&useOrchestratorPlan, "use-orchestrator", true, "Use the agent orchestrator (always true for this command)")
}
