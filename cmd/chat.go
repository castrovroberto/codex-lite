// cmd/chat.go
package cmd

import (
	"fmt"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/logger"
	"github.com/castrovroberto/codex-lite/internal/tui/chat"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive codex lite session",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config is loaded globally by rootCmd (e.g., in rootCmd's PersistentPreRunE or initConfig)
		// So, config.Cfg should be populated by the time this RunE executes.

		modelToUse, _ := cmd.Flags().GetString("model")
		if modelToUse == "" {
			if config.Cfg.DefaultModel == "" { // Add a check in case DefaultModel itself is empty
				logger.Get().Warn("Default model is not set in configuration, and no model specified via flag.")
				// You might want to return an error here or use a hardcoded fallback
				// For now, let it proceed, Ollama might have its own default or error out.
			}
			modelToUse = config.Cfg.DefaultModel
		}

		// Corrected the constructor name and the config argument
		chatModel := chat.InitialModel(&config.Cfg, modelToUse)
		p := tea.NewProgram(chatModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

		if _, err := p.Run(); err != nil {
			logger.Get().Error("Chat TUI failed", "error", err)
			return fmt.Errorf("failed to run interactive chat session: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session (overrides default model in config)")
}