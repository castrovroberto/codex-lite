package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/logger"
	"github.com/castrovroberto/CGE/internal/tui/chat"

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
Chat history is automatically saved in ~/.cge/chat_history/.

Examples:
  CGE chat                    # Start a new chat session
  CGE chat --model llama2     # Use a specific model
  CGE chat --session <id>     # Continue a previous session
  CGE chat --list-sessions    # List available sessions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get configuration and logger from context
		ctx := cmd.Context()
		appCfgValue := contextkeys.ConfigFromContext(ctx)
		appCfg := &appCfgValue
		log := contextkeys.LoggerFromContext(ctx)

		// Initialize TUI-safe logger to prevent logs from interfering with display
		logFile := ".cge/chat.log"
		if err := logger.InitLoggerForTUI(appCfg.Logging.Level, logFile); err != nil {
			log.Warn("Failed to initialize TUI logger, logs may interfere with display", "error", err)
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
			chatModelName = appCfg.LLM.Model
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
		ctx = context.WithValue(ctx, contextkeys.ConfigKey, appCfg)
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, log)

		// Initialize chat model with history if available
		chatAppModel := chat.InitialModel(ctx, appCfg, chatModelName)
		if history != nil {
			chatAppModel.LoadHistory(history)
		}

		// Update header with correct provider info
		if chatAppModel.Header() != nil {
			chatAppModel.Header().SetProvider(appCfg.LLM.Provider)
			chatAppModel.Header().SetModelName(chatModelName)
		}

		// Set more compatible color profile and detect terminal capabilities
		profile := termenv.ColorProfile()
		log.Debug("Detected terminal color profile", "profile", profile)

		// Log terminal environment for debugging
		log.Debug("Terminal environment details",
			"TERM", os.Getenv("TERM"),
			"COLORTERM", os.Getenv("COLORTERM"),
			"TERM_PROGRAM", os.Getenv("TERM_PROGRAM"),
			"color_profile", profile)

		// Use detected profile or fallback to safe ANSI if there are issues
		switch profile {
		case termenv.TrueColor, termenv.ANSI256:
			lipgloss.SetColorProfile(profile)
			log.Debug("Using detected color profile", "profile", profile)
		default:
			// Fallback to ANSI for better compatibility
			lipgloss.SetColorProfile(termenv.ANSI)
			log.Debug("Using ANSI color profile for compatibility")
		}

		// Create and run the Bubble Tea program with enhanced options
		programOptions := []tea.ProgramOption{
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		}

		// Add input sanitization for better terminal control sequence handling
		programOptions = append(programOptions, tea.WithInputTTY())

		p := tea.NewProgram(chatAppModel, programOptions...)

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
