package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
	"github.com/spf13/cobra"
)

var (
	orchestratedReviewTargetDir string
	orchestratedTestCommand     string
	orchestratedLintCommand     string
	orchestratedMaxCycles       int
	orchestratedAutoFix         bool
	orchestratedDryRun          bool
)

// reviewOrchestratedCmd represents the orchestrated review command
var reviewOrchestratedCmd = &cobra.Command{
	Use:   "review-orchestrated [target_directory]",
	Short: "Review and validate generated code using function-calling agent orchestrator",
	Long: `The review-orchestrated command validates generated code by running tests and linters,
then uses an LLM with function-calling capabilities to iteratively improve the code.

This is the next-generation review command that uses the agent orchestrator infrastructure
for more precise and reliable code fixes.

It can:
- Run tests and linters using dedicated tools
- Parse test and lint results with structured output
- Apply targeted fixes using patch tools
- Iterate until all issues are resolved or max cycles reached

Example:
  CGE review-orchestrated ./src --auto-fix --max-cycles 5
  CGE review-orchestrated --test-cmd "go test ./..." --lint-cmd "golangci-lint run"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger := contextkeys.LoggerFromContext(ctx)
		cfg := contextkeys.ConfigFromContext(ctx)

		// Determine target directory
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}
		if orchestratedReviewTargetDir != "" {
			targetDir = orchestratedReviewTargetDir
		}

		// Get absolute path
		absTargetDir, err := filepath.Abs(targetDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		logger.Info("Starting orchestrated code review", "target_dir", absTargetDir)

		// Use commands from config if not specified
		if orchestratedTestCommand == "" {
			orchestratedTestCommand = cfg.Commands.Review.TestCommand
		}
		if orchestratedLintCommand == "" {
			orchestratedLintCommand = cfg.Commands.Review.LintCommand
		}
		if orchestratedMaxCycles == 0 {
			orchestratedMaxCycles = cfg.Commands.Review.MaxCycles
		}

		// Initialize LLM client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			llmClient = llm.NewOllamaClient()
			logger.Info("Using Ollama client for orchestrated review", "host", cfg.LLM.OllamaHostURL)
		default:
			return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
		}

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize tool registry with review tools
		toolFactory := agent.NewToolFactory(workspaceRoot)
		toolRegistry := toolFactory.CreateReviewRegistry()

		// Create command integrator and execute review
		integrator := orchestrator.NewCommandIntegrator(llmClient, toolRegistry, workspaceRoot)

		// Run initial tests and linting to get baseline
		logger.Info("Running initial tests and linting...")
		initialTestOutput := ""
		initialLintOutput := ""

		if orchestratedTestCommand != "" {
			fmt.Printf("🧪 Running tests: %s\n", orchestratedTestCommand)
			testOutput, testErr := runCommand(ctx, orchestratedTestCommand, absTargetDir)
			initialTestOutput = testOutput
			if testErr != nil {
				fmt.Printf("❌ Tests failed\n")
			} else {
				fmt.Printf("✅ Tests passed\n")
			}
		}

		if orchestratedLintCommand != "" {
			fmt.Printf("🔍 Running linter: %s\n", orchestratedLintCommand)
			lintOutput, lintErr := runCommand(ctx, orchestratedLintCommand, absTargetDir)
			initialLintOutput = lintOutput
			if lintErr != nil {
				fmt.Printf("❌ Linting failed\n")
			} else {
				fmt.Printf("✅ Linting passed\n")
			}
		}

		// If no issues found, we're done
		if initialTestOutput == "" && initialLintOutput == "" {
			fmt.Printf("✅ No issues found. Code review complete!\n")
			return nil
		}

		// If auto-fix is disabled, just show the results
		if !orchestratedAutoFix {
			fmt.Printf("\n📋 Review Results:\n")
			if initialTestOutput != "" {
				fmt.Printf("Test Output:\n%s\n\n", initialTestOutput)
			}
			if initialLintOutput != "" {
				fmt.Printf("Lint Output:\n%s\n\n", initialLintOutput)
			}
			fmt.Printf("Use --auto-fix to automatically attempt fixes.\n")
			return nil
		}

		// Execute orchestrated review with function calling
		reviewRequest := &orchestrator.ReviewRequest{
			TargetDir:  absTargetDir,
			TestOutput: initialTestOutput,
			LintOutput: initialLintOutput,
			Model:      cfg.LLM.Model,
			MaxCycles:  orchestratedMaxCycles,
		}

		logger.Info("Executing orchestrated review with function calling...", "model", cfg.LLM.Model)
		reviewResponse, err := integrator.ExecuteReview(ctx, reviewRequest)
		if err != nil {
			logger.Error("Orchestrated review failed", "error", err)
			return fmt.Errorf("orchestrated review failed: %w", err)
		}

		// Print results
		fmt.Printf("\n🎯 Orchestrated Review Complete!\n")
		if reviewResponse.Success {
			fmt.Printf("✅ Review completed successfully\n")
		} else {
			fmt.Printf("⚠️ Review completed with some issues remaining\n")
		}

		if len(reviewResponse.FixesApplied) > 0 {
			fmt.Printf("\n🔧 Fixes Applied:\n")
			for _, fix := range reviewResponse.FixesApplied {
				fmt.Printf("  - %s\n", fix)
			}
		}

		// Show conversation summary
		fmt.Printf("\n📊 Review Statistics:\n")
		fmt.Printf("  - Total Messages: %d\n", len(reviewResponse.Messages))

		toolCalls := 0
		for _, msg := range reviewResponse.Messages {
			if msg.Role == "tool" {
				toolCalls++
			}
		}
		fmt.Printf("  - Tool Calls: %d\n", toolCalls)

		if orchestratedDryRun {
			fmt.Printf("\n🔍 Dry Run Mode: No actual changes were made\n")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(reviewOrchestratedCmd)

	reviewOrchestratedCmd.Flags().StringVarP(&orchestratedReviewTargetDir, "target", "t", "", "Target directory to review (default: current directory)")
	reviewOrchestratedCmd.Flags().StringVar(&orchestratedTestCommand, "test-cmd", "", "Command to run tests (overrides config)")
	reviewOrchestratedCmd.Flags().StringVar(&orchestratedLintCommand, "lint-cmd", "", "Command to run linter (overrides config)")
	reviewOrchestratedCmd.Flags().IntVar(&orchestratedMaxCycles, "max-cycles", 0, "Maximum number of review cycles (overrides config)")
	reviewOrchestratedCmd.Flags().BoolVar(&orchestratedAutoFix, "auto-fix", false, "Automatically attempt to fix issues using function-calling agent")
	reviewOrchestratedCmd.Flags().BoolVar(&orchestratedDryRun, "dry-run", false, "Show what would be done without making actual changes")
}
