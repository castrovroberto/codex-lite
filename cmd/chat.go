package cmd

import (
	//"context"
	"errors"
	"fmt"

	// Import slog for fallback logger
	// Needed for fallback logger

	//"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/logger"
	"github.com/castrovroberto/codex-lite/internal/tui/chat"

	//"github.com/castrovroberto/codex-lite/internal/tui/chat"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with an LLM",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config and Logger are loaded and set in context by rootCmd.PersistentPreRunE
		// We can retrieve config from context, but let's rely on the global logger
		// which is guaranteed to be initialized by PersistentPreRunE.
		appCfg := contextkeys.ConfigFromContext(cmd.Context())
		log := logger.Get() // Get the global logger, initialized by PersistentPreRunE

		// Get model name for chat (from flag or config)
		chatModelName, _ := cmd.Flags().GetString("model")
		if chatModelName == "" {
			chatModelName = appCfg.DefaultModel
		}
		if chatModelName == "" {
			log.Error("No model specified for chat and no default model configured.")
			return errors.New("chat model not specified")
		}
		log.Info("Starting chat session", "model", chatModelName)

		// Pass the context (which contains config and logger) to InitialModel
		// Note: InitialModel now takes ctx as the first argument
		chatAppModel := chat.InitialModel(cmd.Context(), &appCfg, chatModelName)

		// Create and run the Bubble Tea program
		p := tea.NewProgram(chatAppModel, tea.WithAltScreen()) // Use tea alias

		if _, err := p.Run(); err != nil {
			log.Error("Chat TUI failed", "error", err)
			return fmt.Errorf("failed to run interactive chat session: %w", err)
		}
		return nil
	},
}

func init() {
	// Define flags here. Do NOT access config from context in init().
	// The 'model' flag value will be read in RunE.
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session (overrides default model in config)")
	// chatSession and disablePrettyPrint flags were in the provided chat.go snippet,
	// but are not used in the RunE logic provided. Commenting them out for now.
	// chatCmd.Flags().StringVarP(&chatSession, "session", "s", "", "Session ID to continue a previous chat")
	// chatCmd.Flags().BoolVarP(&disablePrettyPrint, "disable-pretty-print", "d", false, "Disable pretty printing of output")

	rootCmd.AddCommand(chatCmd)
}
