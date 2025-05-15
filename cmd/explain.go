package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/spf13/cobra"
)

var explainModelName string

// explainCmd represents the explain command
var explainCmd = &cobra.Command{
	Use:   "explain [file]",
	Short: "Explains a code file using an LLM",
	Long: `The explain command reads a specified code file, sends its content to a
local LLM via Ollama, and prints the explanation of the code.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
			os.Exit(1)
		}

		agent := &agents.ExplainAgent{Model: explainModelName}
		result, err := agent.Analyze(filePath, string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error analyzing file with ExplainAgent: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n📘 Explanation for %s (using %s):\n\n%s\n", result.File, explainModelName, strings.TrimSpace(result.Output))
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
	explainCmd.Flags().StringVarP(&explainModelName, "model", "m", "deepseek-coder-v2-lite", "Model to use for explanation")
}