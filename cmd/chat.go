package cmd

import (
	"fmt"
	"os"

	"github.com/castrovroberto/codex-lite/internal/tui/chat" // New import
	tea "github.com/charmbracelet/bubbletea"                 // New import
	"github.com/spf13/cobra"
)

var chatModelName string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive Codex Lite session",
	Run: func(cmd *cobra.Command, args []string) {
		cwd := getCWD()
		// chatModelName is already available from the flag.

		initialModel := chat.NewInitialModel(cwd, chatModelName)
		p := tea.NewProgram(initialModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().StringVarP(&chatModelName, "model", "m", "deepseek-coder-v2:lite", "Model to use for the chat session")
}

func getCWD() string {
	dir, err := os.Getwd()
	if err != nil {
		return "(unknown)"
	}
	return dir
}
