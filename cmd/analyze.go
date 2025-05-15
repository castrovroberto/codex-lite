// cmd/analyze.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castrovroberto/codex-lite/internal/agents"    // Adjust path if needed
	"github.com/castrovroberto/codex-lite/internal/config" // Adjust path if needed
)

var (
	// Flags for the analyze command
	ollamaHost       string
	modelName        string
	selectedAgentsStr string // Comma-separated list of agent names
)

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze [file]",
	Short: "Analyze a code file with selected agent(s)",
	Long: `Analyzes a specified code file using one or more available agents.
You can specify which agents to run using the --agents flag.
Available agents include: explain, syntax.

Example:
  codex-lite analyze main.go --agents explain,syntax --model deepseek-coder-v2-lite
  codex-lite analyze utils.py --agents syntax --ollama-host http://custom-ollama:11434`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument: the file path
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file '%s': %v\n", filePath, err)
			os.Exit(1)
		}

		// Setup AppConfig from flags
		appConfiguration := config.AppConfig{
			OllamaHostURL: ollamaHost,
			// Add other global configs here if needed, e.g., appConfiguration.DefaultModel = modelName
		}
		ctx := config.NewContext(context.Background(), appConfiguration)

		// Determine which agents to run
		var agentsToRun []agents.Agent
		agentNames := strings.Split(selectedAgentsStr, ",")
		if selectedAgentsStr == "" { // Default agents if none specified
			agentNames = []string{"explain", "syntax"}
		}

		// Available agents registry (can be expanded)
		availableAgents := map[string]agents.Agent{
			"explain": &agents.ExplainAgent{},
			"syntax":  &agents.SyntaxAgent{},
			// Add other agents here:
			// "smell": &agents.SmellDetectorAgent{},
			// "security": &agents.SecurityAgent{},
		}

		for _, name := range agentNames {
			trimmedName := strings.TrimSpace(name)
			if agent, ok := availableAgents[trimmedName]; ok {
				agentsToRun = append(agentsToRun, agent)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Unknown agent '%s' specified, skipping.\n", trimmedName)
			}
		}

		if len(agentsToRun) == 0 {
			fmt.Fprintln(os.Stderr, "No valid agents selected to run. Exiting.")
			fmt.Fprintln(os.Stderr, "Available agents: explain, syntax") // List available ones
			os.Exit(1)
		}

		fmt.Printf("Analyzing file: %s\n", filePath)
		fmt.Printf("Using model: %s\n", modelName)
		fmt.Printf("Ollama host: %s\n", appConfiguration.OllamaHostURL)
		fmt.Println("---")

		// Run selected agents
		for _, agent := range agentsToRun {
			fmt.Printf("🤖 Running %s...\n", agent.Name())
			result, err := agent.Analyze(ctx, modelName, filePath, string(fileData))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error during %s analysis: %v\n", agent.Name(), err)
				fmt.Println("---")
				continue // Continue to the next agent even if one fails
			}
			fmt.Printf("\n📘 [%s] - Result from %s:\n%s\n", result.File, result.Agent, result.Output)
			fmt.Println("---")
		}
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	// Define flags for the analyze command
	analyzeCmd.Flags().StringVar(&ollamaHost, "ollama-host", "http://localhost:11434", "Ollama host URL")
	analyzeCmd.Flags().StringVarP(&modelName, "model", "m", "deepseek-coder-v2-lite", "Model name to use for analysis")
	analyzeCmd.Flags().StringVarP(&selectedAgentsStr, "agents", "a", "explain,syntax", "Comma-separated list of agents to run (e.g., explain,syntax)")
}