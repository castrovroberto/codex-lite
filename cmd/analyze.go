// cmd/analyze.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castrovroberto/codex-lite/internal/agents"    // Adjust path if needed
	"github.com/castrovroberto/codex-lite/internal/config"    // Adjust path if needed
	"github.com/castrovroberto/codex-lite/internal/logger" // Added
)

var selectedAgentsStr string // Comma-separated list of agent names

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
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			logger.Get().Error("Error reading file", "path", filePath, "error", err)
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		// Config is already loaded globally by rootCmd's PersistentPreRunE
		// ctx := config.NewContext(context.Background(), appConfiguration) // No longer needed
		ctx := context.Background() // Use a standard background context

		// Determine model to use: flag > global config
		modelToUse, _ := cmd.Flags().GetString("model")
		if modelToUse == "" {
			modelToUse = config.Cfg.DefaultModel
		}

		// Determine which agents to run
		var agentsToRun []agents.Agent
		var agentNames []string
		if selectedAgentsStr != "" { // User provided --agents flag
			agentNames = strings.Split(selectedAgentsStr, ",")
		} else { // --agents flag not provided or was explicitly empty, use the effective default agent list from config
			// config.Cfg.DefaultAgentList would have been populated by Viper from flags, env, config file, or its own SetDefault.
			agentNames = config.Cfg.DefaultAgentList
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
				logger.Get().Warn("Unknown agent specified, skipping", "agent_name", trimmedName)
			}
		}

		if len(agentsToRun) == 0 {
			return fmt.Errorf("no valid agents selected to run. Available: explain, syntax. Check config for 'default_agent_list' or use --agents flag")
		}

		// User-facing informational output can still use fmt.Printf or be logged at INFO level
		// depending on whether it's primary output or diagnostic.
		logger.Get().Info("Starting analysis", "file", filePath, "model", modelToUse, "ollama_host", config.Cfg.OllamaHostURL)
		fmt.Printf("Analyzing file: %s with model %s (Ollama: %s)\n", filePath, modelToUse, config.Cfg.OllamaHostURL)
		fmt.Println("---") // User output separator

		// Run selected agents
		for _, agent := range agentsToRun {
			logger.Get().Info("Running agent", "agent_name", agent.Name())
			fmt.Printf("🤖 Running %s...\n", agent.Name()) // User output
			result, err := agent.Analyze(ctx, modelToUse, filePath, string(fileData))
			if err != nil {
				logger.Get().Error("Error during agent analysis", "agent_name", agent.Name(), "error", err)
				// Optionally print a user-friendly error message too
				fmt.Printf("⚠️ Error with %s: %v\n", agent.Name(), err)
				fmt.Println("---") // User output separator
				continue // Continue to the next agent even if one fails
			}
			fmt.Printf("\n📘 [%s] - Result from %s:\n%s\n", result.File, result.Agent, result.Output)
			fmt.Println("---")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	// Flags for analyze command. These will override global settings if provided.
	// No need to bind ollama-host-url here if it's a persistent flag on root.
	// If you want analyze to have its own ollama-host distinct from global:
	// analyzeCmd.Flags().String("ollama-host-url", "", "Ollama host URL for this analysis")
	// viper.BindPFlag("ollama_host_url_analyze", analyzeCmd.Flags().Lookup("ollama-host-url")) // Use a distinct key
	analyzeCmd.Flags().StringP("model", "m", "", "Model name to use for analysis (overrides default model).")
	analyzeCmd.Flags().StringVarP(&selectedAgentsStr, "agents", "a", "", "Comma-separated list of agents to run (e.g., explain,syntax). Overrides default agent list from config.")
}