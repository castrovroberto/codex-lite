package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/tui/chat"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with an LLM",
	Long: `Start an interactive chat session with an LLM.
You can continue a previous session using the --session flag.
Chat history is automatically saved in ~/.codex-lite/chat_history/.

Examples:
  codex-lite chat                    # Start a new chat session
  codex-lite chat --model llama2     # Use a specific model
  codex-lite chat --session <id>     # Continue a previous session
  codex-lite chat --list-sessions    # List available sessions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCfg := contextkeys.ConfigFromContext(cmd.Context())
		log := contextkeys.LoggerFromContext(cmd.Context())

		if log == nil {
			fmt.Fprintln(os.Stderr, "Error: Logger not found in context. Using a temporary basic logger.")
			log = slog.New(slog.NewTextHandler(os.Stderr, nil))
		}

		// Handle --list-sessions flag
		listSessions, _ := cmd.Flags().GetBool("list-sessions")
		if listSessions {
			sessions, err := chat.ListChatSessions()
			if err != nil {
				log.Error("Failed to list chat sessions", "error", err)
				return fmt.Errorf("failed to list chat sessions: %w", err)
			}
			if len(sessions) == 0 {
				fmt.Println("No chat sessions found.")
				return nil
			}
			fmt.Println("Available chat sessions:")
			for _, session := range sessions {
				fmt.Printf("  %s\n", session)
			}
			return nil
		}

		// Get model name for chat (from flag or config)
		chatModelName, _ := cmd.Flags().GetString("model")
		if chatModelName == "" {
			chatModelName = appCfg.DefaultModel
		}
		if chatModelName == "" {
			log.Error("No model specified for chat and no default model configured.")
			return errors.New("chat model not specified")
		}

		// Check for session ID
		sessionID, _ := cmd.Flags().GetString("session")
		var history *chat.ChatHistory
		var err error

		if sessionID != "" {
			// Load specific session
			history, err = chat.LoadHistory(sessionID)
			if err != nil {
				log.Error("Failed to load chat session", "session", sessionID, "error", err)
				return fmt.Errorf("failed to load chat session: %w", err)
			}
			log.Info("Loaded chat session", "session", sessionID)
		}

		log.Info("Starting chat session", "model", chatModelName)

		// Create a context containing the global config and logger for downstream components
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, contextkeys.ConfigKey, appCfg)
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, log)

		// Initialize chat model with history if available
		chatAppModel := chat.InitialModel(ctx, &appCfg, chatModelName)
		if history != nil {
			chatAppModel.LoadHistory(history)
		}

		// Attempt to force a more compatible color profile for lipgloss
		lipgloss.SetColorProfile(termenv.ANSI256)

		// Create and run the Bubble Tea program with mouse support
		p := tea.NewProgram(
			chatAppModel,
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		)

		if _, err := p.Run(); err != nil {
			log.Error("Chat TUI failed", "error", err)
			return fmt.Errorf("failed to run interactive chat session: %w", err)
		}

		return nil
	},
}

func init() {
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session (overrides default model in config)")
	chatCmd.Flags().StringP("session", "s", "", "Session ID to continue a previous chat")
	chatCmd.Flags().Bool("list-sessions", false, "List available chat sessions")
	rootCmd.AddCommand(chatCmd)
}
