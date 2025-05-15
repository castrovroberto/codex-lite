// cmd/chat.go
package cmd

import (
	"context"
	"fmt"
	"os"
	// "strings" // Keep if used, remove if not (e.g. if sessionLoop changes drastically)
	// "bufio"   // Keep if used, remove if not

	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"

	// Adjust these paths if they are different
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/tui/chat" // Assuming this is your TUI model package
	// "github.com/castrovroberto/codex-lite/internal/ollama" // If ollama calls are made here directly
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive Codex Lite session",
	Run: func(cmd *cobra.Command, args []string) {
		// Get configurations (e.g., from global flags or defaults)
		// These flags might be defined on rootCmd or chatCmd itself
		// For now, let's assume they are accessible similar to analyzeCmd's flags
		// If ollamaHost and modelName are global flags on rootCmd:
		// globalOllamaHost := rootCmd.PersistentFlags().Lookup("ollama-host").Value.String()
		// globalModelName := rootCmd.PersistentFlags().Lookup("model").Value.String()

		// For simplicity, using the same defaults as analyzeCmd if flags aren't set up on chatCmd yet
		// In a real app, you'd properly get these from flags or a config file
		appCfg := config.AppConfig{
			OllamaHostURL: "http://localhost:11434", // Default or from flag
			// You might want a DefaultModel in AppConfig too
			// DefaultModel: "deepseek-coder-v2-lite", // Default or from flag
		}
		// The TUI model itself will need the model name, either from AppConfig or a flag specific to chat
		chatModelName := "deepseek-coder-v2-lite" // Or get from a flag

		// The TUI model will need its own context if it makes calls that need config
		// However, Bubbletea models don't directly take context in their constructor in the typical sense.
		// We pass the config to the model's constructor.
		// The context for Ollama calls will be created within the TUI's command execution.

		chatModel := chat.NewModel(appCfg, chatModelName) // Pass config and model name
		p := tea.NewProgram(chatModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	// Add flags for chat command if needed for Ollama host and model
	// chatCmd.Flags().String("ollama-host-tui", "http://localhost:11434", "Ollama host URL for TUI")
	// chatCmd.Flags().String("model-tui", "deepseek-coder-v2-lite", "Model name for TUI")
}

// Remove old sessionLoop and getCWD if they are no longer used by the Bubbletea TUI.
// func sessionLoop() { ... }
// func getCWD() string { ... }