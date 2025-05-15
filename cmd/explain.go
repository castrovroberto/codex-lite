package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/logger" // Added
	"github.com/spf13/cobra"
)

// explainCmd represents the explain command
var explainCmd = &cobra.Command{
	Use:   "explain [file]",
	Short: "Explains a code file using an LLM",
	Long: `The explain command reads a specified code file, sends its content to a
local LLM via Ollama, and prints the explanation of the code.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			logger.Get().Error("Error reading file", "path", filePath, "error", err)
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		// Use the model specified by flag, or fallback to the global default model
		modelToUse, _ := cmd.Flags().GetString("model")
		if modelToUse == "" {
			modelToUse = config.Cfg.DefaultModel
		}

		// Create a basic context. Later, this context can be populated with loaded configurations.
		ctx := context.Background()
		agent := &agents.ExplainAgent{} // No model field needed here
		result, err := agent.Analyze(ctx, modelToUse, filePath, string(data))
		if err != nil {
			// Log it here for structured details, but also return it for Cobra to display
			logger.Get().Error("Analysis failed", "agent", agent.Name(), "file", filePath, "error", err)
			return fmt.Errorf("analysis by %s failed: %w", agent.Name(), err)
		}

		logger.Get().Info("Successfully explained file", "path", result.File, "model", modelToUse)
		fmt.Printf("\n📘 Explanation for %s (using %s):\n\n%s\n", result.File, modelToUse, strings.TrimSpace(result.Output)) // User output
		return nil
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
	// The default value for this flag will be empty, so Viper's default_model takes precedence unless specified.
	explainCmd.Flags().StringP("model", "m", "", "Model to use for explanation (overrides default model)")
	// No direct viper.BindPFlag here, as we handle fallback logic in Run
}