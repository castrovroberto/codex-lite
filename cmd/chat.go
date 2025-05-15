// cmd/chat.go
package cmd

import (
	// "context" // Removed: imported and not used
	"fmt"
	// "os"
	// "strings" // No longer directly used here

	"github.com/castrovroberto/codex-lite/internal/config" // Added
	"github.com/castrovroberto/codex-lite/internal/logger" // Added
	"github.com/castrovroberto/codex-lite/internal/tui/chat"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	// "github.com/castrovroberto/codex-lite/internal/ollama" // Commented out in original
	// tea "github.com/charmbracelet/bubbletea" // Commented out in original
	// "github.com/castrovroberto/codex-lite/internal/tui" // Commented out in original
)

// var chatModelName string // No longer needed as a package-level var

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive codex lite session",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config is loaded globally by rootCmd
		modelToUse, _ := cmd.Flags().GetString("model")
		if modelToUse == "" {
			modelToUse = config.Cfg.DefaultModel
		}

		// Pass the global config.Cfg to the TUI model constructor
		chatModel := chat.NewModel(config.Cfg, modelToUse)
		p := tea.NewProgram(chatModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

		if _, err := p.Run(); err != nil {
			// Bubbletea errors are often not great for direct user display without context.
			// Logging it is good. Returning a simpler error might be better for the user.
			logger.Get().Error("Chat TUI failed", "error", err)
			return fmt.Errorf("failed to run interactive chat session: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session (overrides default model)")
}
// func getCWD() string { ... } // Commented out in original