// cmd/chat.go
package cmd

import (
	"bufio"
	// "context" // Removed: imported and not used
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	// "github.com/castrovroberto/codex-lite/internal/ollama" // Commented out in original
	// tea "github.com/charmbracelet/bubbletea" // Commented out in original
	// "github.com/castrovroberto/codex-lite/internal/tui" // Commented out in original
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive codex lite session",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Codex Lite interactive session. Type 'exit' to quit.")
		// ollama.Hello() // Commented out in original
		sessionLoop()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func sessionLoop() {
	reader := bufio.NewReader(os.Stdin)
	// var conversationHistory []string // Commented out in original

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		// conversationHistory = append(conversationHistory, "User: "+input) // Commented out in original

		if input == "exit" {
			fmt.Println("Exiting...")
			break
		}

		// Placeholder for processing the input and getting a response
		// response := ollama.Send(input, conversationHistory) // Commented out in original
		// conversationHistory = append(conversationHistory, "AI: "+response) // Commented out in original
		// fmt.Println(response) // Commented out in original
		fmt.Println("AI: Placeholder response to '" + input + "'") // Current placeholder
	}
}

// func getCWD() string { ... } // Commented out in original