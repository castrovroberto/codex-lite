package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/templates"
	"github.com/spf13/cobra"
)

var (
	reviewTargetDir string
	testCommand     string
	lintCommand     string
	maxCycles       int
	autoFix         bool
)

// ReviewResult represents the result of a review cycle
type ReviewResult struct {
	TestsPassed  bool     `json:"tests_passed"`
	LintPassed   bool     `json:"lint_passed"`
	TestOutput   string   `json:"test_output"`
	LintOutput   string   `json:"lint_output"`
	Issues       []string `json:"issues"`
	Suggestions  []string `json:"suggestions"`
	FixesApplied []string `json:"fixes_applied"`
}

// reviewCmd represents the review command
var reviewCmd = &cobra.Command{
	Use:   "review [target_directory]",
	Short: "Review and validate generated code using tests and linters",
	Long: `The review command validates generated code by running tests and linters,
then uses an LLM to iteratively improve the code based on the results.

It can:
- Run tests and linters on the codebase
- Analyze failures and suggest fixes
- Automatically apply fixes using LLM assistance
- Iterate until all issues are resolved or max cycles reached

Example:
  CGE review ./src --test-cmd "go test ./..." --lint-cmd "golangci-lint run"
  CGE review --auto-fix --max-cycles 3`,
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
		if reviewTargetDir != "" {
			targetDir = reviewTargetDir
		}

		// Get absolute path
		absTargetDir, err := filepath.Abs(targetDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		logger.Info("Starting code review", "target_dir", absTargetDir)

		// Use commands from config if not specified
		if testCommand == "" {
			testCommand = cfg.Commands.Review.TestCommand
		}
		if lintCommand == "" {
			lintCommand = cfg.Commands.Review.LintCommand
		}
		if maxCycles == 0 {
			maxCycles = cfg.Commands.Review.MaxCycles
		}

		// Initialize LLM client if auto-fix is enabled
		var llmClient llm.Client
		if autoFix {
			switch cfg.LLM.Provider {
			case "ollama":
				llmClient = llm.NewOllamaClient()
				logger.Info("Using Ollama client for auto-fix", "host", cfg.LLM.OllamaHostURL)
			default:
				return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
			}
		}

		// Get workspace root for templates
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize template engine
		promptsDir := filepath.Join(workspaceRoot, "prompts")
		templateEngine := templates.NewEngine(promptsDir)

		// Run review cycles
		for cycle := 1; cycle <= maxCycles; cycle++ {
			logger.Info("Starting review cycle", "cycle", cycle, "max_cycles", maxCycles)

			result, err := runReviewCycle(ctx, absTargetDir, testCommand, lintCommand, logger)
			if err != nil {
				logger.Error("Review cycle failed", "cycle", cycle, "error", err)
				return fmt.Errorf("review cycle %d failed: %w", cycle, err)
			}

			// Print results
			printReviewResults(result, cycle)

			// Check if we're done
			if result.TestsPassed && result.LintPassed {
				logger.Info("All checks passed!", "cycle", cycle)
				fmt.Printf("✅ All checks passed after %d cycle(s)!\n", cycle)
				return nil
			}

			// If auto-fix is disabled or this is the last cycle, stop here
			if !autoFix || cycle == maxCycles {
				if cycle == maxCycles {
					logger.Warn("Maximum cycles reached", "max_cycles", maxCycles)
					fmt.Printf("⚠️  Maximum cycles (%d) reached. Some issues remain unresolved.\n", maxCycles)
				}
				break
			}

			// Apply fixes using LLM
			logger.Info("Attempting to fix issues with LLM", "cycle", cycle)
			err = applyLLMFixes(ctx, result, llmClient, templateEngine, absTargetDir, cfg, logger)
			if err != nil {
				logger.Error("Failed to apply LLM fixes", "cycle", cycle, "error", err)
				// Continue to next cycle even if fixes fail
			}

			// Wait a bit before next cycle
			time.Sleep(2 * time.Second)
		}

		return nil
	},
}

// runReviewCycle executes tests and linters and returns the results
func runReviewCycle(ctx context.Context, targetDir, testCmd, lintCmd string, logger interface{}) (*ReviewResult, error) {
	result := &ReviewResult{}

	// Run tests if command is specified
	if testCmd != "" {
		fmt.Printf("🧪 Running tests: %s\n", testCmd)
		testOutput, testErr := runCommand(ctx, testCmd, targetDir)
		result.TestOutput = testOutput
		result.TestsPassed = testErr == nil

		if testErr != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Tests failed: %v", testErr))
		}
	} else {
		result.TestsPassed = true // No tests to run
	}

	// Run linter if command is specified
	if lintCmd != "" {
		fmt.Printf("🔍 Running linter: %s\n", lintCmd)
		lintOutput, lintErr := runCommand(ctx, lintCmd, targetDir)
		result.LintOutput = lintOutput
		result.LintPassed = lintErr == nil

		if lintErr != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Linting failed: %v", lintErr))
		}
	} else {
		result.LintPassed = true // No linting to run
	}

	return result, nil
}

// runCommand executes a shell command and returns its output
func runCommand(ctx context.Context, command, workingDir string) (string, error) {
	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// printReviewResults displays the results of a review cycle
func printReviewResults(result *ReviewResult, cycle int) {
	fmt.Printf("\n=== Review Cycle %d Results ===\n", cycle)

	// Test results
	if result.TestOutput != "" {
		if result.TestsPassed {
			fmt.Printf("✅ Tests: PASSED\n")
		} else {
			fmt.Printf("❌ Tests: FAILED\n")
			if result.TestOutput != "" {
				fmt.Printf("Test output:\n%s\n", result.TestOutput)
			}
		}
	}

	// Lint results
	if result.LintOutput != "" {
		if result.LintPassed {
			fmt.Printf("✅ Linting: PASSED\n")
		} else {
			fmt.Printf("❌ Linting: FAILED\n")
			if result.LintOutput != "" {
				fmt.Printf("Lint output:\n%s\n", result.LintOutput)
			}
		}
	}

	// Issues
	if len(result.Issues) > 0 {
		fmt.Printf("\n🔍 Issues found:\n")
		for _, issue := range result.Issues {
			fmt.Printf("  - %s\n", issue)
		}
	}

	// Fixes applied
	if len(result.FixesApplied) > 0 {
		fmt.Printf("\n🔧 Fixes applied:\n")
		for _, fix := range result.FixesApplied {
			fmt.Printf("  - %s\n", fix)
		}
	}
}

// applyLLMFixes uses the LLM to suggest and apply fixes for the identified issues
func applyLLMFixes(ctx context.Context, result *ReviewResult, llmClient llm.Client, templateEngine *templates.Engine, targetDir string, cfg interface{}, logger interface{}) error {
	// This is a placeholder for LLM-based fix generation
	// In a real implementation, you would:
	// 1. Create a prompt with the test/lint failures
	// 2. Ask the LLM to suggest fixes
	// 3. Parse the LLM response to extract code changes
	// 4. Apply the changes to the files
	// 5. Update result.FixesApplied

	fmt.Printf("🤖 LLM-based auto-fix not yet implemented\n")
	fmt.Printf("   Would analyze: %d issues\n", len(result.Issues))

	// For now, just add a placeholder fix
	result.FixesApplied = append(result.FixesApplied, "Placeholder: LLM analysis completed")

	return nil
}

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().StringVarP(&reviewTargetDir, "target", "t", "", "Target directory to review (default: current directory)")
	reviewCmd.Flags().StringVar(&testCommand, "test-cmd", "", "Command to run tests (overrides config)")
	reviewCmd.Flags().StringVar(&lintCommand, "lint-cmd", "", "Command to run linter (overrides config)")
	reviewCmd.Flags().IntVar(&maxCycles, "max-cycles", 0, "Maximum number of review cycles (overrides config)")
	reviewCmd.Flags().BoolVar(&autoFix, "auto-fix", false, "Automatically attempt to fix issues using LLM")
}
