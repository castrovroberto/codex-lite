package cmd

import (
	// "context" // Will be used now
	"fmt"
	// "log/slog" // May become unused if log variable is not explicitly typed here
	// "github.com/castrovroberto/codex-lite/internal/config" // May become unused

	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/tui/chat"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive codex lite session",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context() // Get the context
		log := contextkeys.LoggerFromContext(ctx)
		appCfg := contextkeys.ConfigFromContext(ctx) // appCfg is config.AppConfig

		modelToUse, _ := cmd.Flags().GetString("model")
		if modelToUse == "" {
			if appCfg.DefaultModel == "" {
				log.Warn("Default model is not set in configuration, and no model specified via flag.")
			}
			modelToUse = appCfg.DefaultModel
		}

		if modelToUse == "" {
			log.Error("No model available for chat session. Please set a default model or use the --model flag.")
			return fmt.Errorf("no model available for chat session")
		}

		log.Info("Starting chat session", "model", modelToUse, "ollama_host", appCfg.OllamaHostURL)

		// Call chat.InitialModel with the context as the first argument
		// The error message "want (context.Context, *config.AppConfig, string)" implies this is the expected signature
		chatModel := chat.InitialModel(ctx, &appCfg, modelToUse) // Pass ctx, and address of appCfg

		p := tea.NewProgram(chatModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

		if _, err := p.Run(); err != nil {
			log.Error("Chat TUI failed", "error", err)
			return fmt.Errorf("failed to run interactive chat session: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session (overrides default model in config)")
}