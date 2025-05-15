/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "codex-lite",
	Short: "Codex Lite: Your AI-powered coding assistant.",
	Long: `Codex Lite is a command-line tool that leverages local LLMs (via Ollama)
to provide code explanation, analysis, and interactive chat capabilities.

Configure it via a .codex-lite.yaml file in your home or current directory,
environment variables (prefixed with CODEXLITE_), or command-line flags.
For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.LoadConfig(cfgFile); err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.codex-lite.yaml or ./.codex-lite.yaml)")

	// Bind flags for global config settings that can be overridden via root command
	rootCmd.PersistentFlags().String("ollama-host-url", "", "Ollama host URL (e.g., http://localhost:11434)")
	viper.BindPFlag("ollama_host_url", rootCmd.PersistentFlags().Lookup("ollama-host-url"))
	rootCmd.PersistentFlags().String("default-model", "", "Default LLM model name")
	viper.BindPFlag("default_model", rootCmd.PersistentFlags().Lookup("default-model"))
}
