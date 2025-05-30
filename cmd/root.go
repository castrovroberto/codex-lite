/*
Copyright © 2024 Roberto Castro roberto.castro@example.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/castrovroberto/CGE/internal/config" // Assuming this path is correct
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/logger" // New import
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "CGE",
	Short: "Codex Lite: Your AI-powered coding assistant.",
	Long: `Codex Lite is a command-line tool that leverages local LLMs (via Ollama)
to provide code explanation, analysis, and interactive chat capabilities.

Configure it via a .cge.yaml file in your home or current directory,
environment variables (prefixed with CODEXLITE_), or command-line flags.
For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Configuration loading should happen before anything that might depend on it.
		if err := config.LoadConfig(cfgFile); err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		logger.InitLogger(config.Cfg.Logging.Level) // Initialize logger after config is loaded

		// The context is now set by ExecuteContext before this PersistentPreRunE is called.
		// We retrieve it and add our values.
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, contextkeys.ConfigKey, &config.Cfg)
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, logger.Get())
		cmd.SetContext(ctx) // Set the enriched context back to the command

		return nil
	},
}

// ExecuteContext adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
// It uses the provided context for the command execution.
func ExecuteContext(ctx context.Context) error {
	// Set the initial context for the root command.
	// This context will be available in PersistentPreRunE and RunE functions.
	rootCmd.SetContext(ctx)

	// Execute the root command with the provided context.
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		// Cobra already prints the error to stderr when ExecuteContext fails.
		// We also os.Exit(1) in the original Execute() or let main handle it.
		// Here, we just return the error for main.go to decide.
		return err
	}
	return nil
}

// Execute is the original execute function, retained for compatibility if needed
// but new calls should ideally use ExecuteContext.
// Deprecated: Use ExecuteContext instead to support graceful shutdown.
func Execute() {
	// For simplicity in this refactor, Execute now calls ExecuteContext with a background context.
	// This means old call paths won't benefit from signal handling based cancellation.
	if err := ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cge/codex.toml, $HOME/.codex.toml or ./codex.toml)")

	// Bind flags for global config settings that can be overridden via root command
	// Example: rootCmd.PersistentFlags().String("llm-provider", "", "LLM provider (e.g., ollama, openai)")
	// viper.BindPFlag("llm.provider", rootCmd.PersistentFlags().Lookup("llm-provider"))

	// rootCmd.PersistentFlags().String("llm-model", "", "LLM model name")
	// viper.BindPFlag("llm.model", rootCmd.PersistentFlags().Lookup("llm-model"))

	// Removed default-agent-list as analyze command is deprecated
	// rootCmd.PersistentFlags().StringSlice("default-agent-list", []string{}, "Default comma-separated list of agents (overrides config if set, e.g., explain,syntax)")
	// viper.BindPFlag("default_agent_list", rootCmd.PersistentFlags().Lookup("default-agent-list"))

	// Update references to old config names in help text
	// Ensure all viper.BindPFlag calls correctly map to the new AppConfig structure if global flags are kept.
	// For example, if ollama_host_url is now under llm.ollama_host_url:
	// rootCmd.PersistentFlags().String("ollama-host-url", "", "Ollama host URL (e.g., http://localhost:11434)")
	// viper.BindPFlag("llm.ollama_host_url", rootCmd.PersistentFlags().Lookup("ollama-host-url"))

	// Default model is now under llm.model
	// rootCmd.PersistentFlags().String("default-model", "", "Default LLM model name")
	// viper.BindPFlag("llm.model", rootCmd.PersistentFlags().Lookup("default-model"))

	// It's cleaner to manage these via the codex.toml or environment variables (CGE_LLM_PROVIDER, CGE_LLM_MODEL)
	// For now, removing the direct persistent flags for ollama_host_url and default_model to simplify and encourage config file usage.
}
