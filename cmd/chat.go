package cmd

import (
   "context"
   "errors"
   "fmt"

   "github.com/castrovroberto/codex-lite/internal/config"
   "github.com/castrovroberto/codex-lite/internal/contextkeys"
   "github.com/castrovroberto/codex-lite/internal/logger"
   "github.com/castrovroberto/codex-lite/internal/tui/chat"

   tea "github.com/charmbracelet/bubbletea"
   "github.com/charmbracelet/lipgloss"
   "github.com/muesli/termenv"
   "github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with an LLM",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config and Logger are loaded by rootCmd.PersistentPreRunE.
		// Retrieve config from the global variable populated by LoadConfig.
		// Retrieve logger from the global logger initialized by InitLogger.
		appCfg := config.Cfg // Use the global config variable
		log := logger.Get()  // Get the global logger

		// Although we are using global config/logger here,
		// the context still contains them and is passed down to the TUI model
		// and subsequently to ollama.Query, which *does* retrieve them from context.
		// This ensures consistency in how downstream components access config/logger.

		/* Old context retrieval:
		appCfg := contextkeys.ConfigFromContext(cmd.Context())
		log := logger.Get() // Get the global logger, initialized by PersistentPreRunE
		*/
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

		// Create a context containing the global config and logger for downstream components
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, contextkeys.ConfigKey, appCfg)
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, log)
		chatAppModel := chat.InitialModel(ctx, &appCfg, chatModelName)

		// Attempt to force a more compatible color profile for lipgloss
		// This might help with terminals that don't fully support TrueColor OSC sequences.
		// You can try termenv.ANSI256 or termenv.ANSI
		lipgloss.SetColorProfile(termenv.ANSI256)

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
