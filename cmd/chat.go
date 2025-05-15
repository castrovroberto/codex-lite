package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch an interactive Codex Lite session",
	Run: func(cmd *cobra.Command, args []string) {
		sessionLoop()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func sessionLoop() {
	fmt.Println("\n+--------------------------------------------------+")
	fmt.Println("| Codex Lite - Interactive Session (v0.1)         |")
	fmt.Println("+--------------------------------------------------+")
	fmt.Printf("| Workdir: %-40s |\n", getCWD())
	fmt.Printf("| Model:   %-40s |\n", "deepseek-coder-v2-lite")
	fmt.Println("+--------------------------------------------------+")
	fmt.Println("| Type your prompt below. Type 'exit' to quit.    |")
	fmt.Println("| Suggestions:                                    |")
	fmt.Println("|  → explain this file                            |")
	fmt.Println("|  → show smells                                  |")
	fmt.Println("|  → summarize this function                      |")
	fmt.Println("+--------------------------------------------------+")

	scanner := bufio.NewScanner(os.Stdin)
	model := "deepseek-coder-v2-lite"

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		prompt := scanner.Text()
		if strings.TrimSpace(prompt) == "exit" {
			break
		}

		response, err := ollama.Query(model, prompt)
		if err != nil {
			fmt.Println("⚠️  Error from model:", err)
			continue
		}

		fmt.Println("\n🤖 Response:\n")
		fmt.Println(strings.TrimSpace(response))
	}
}

func getCWD() string {
	dir, err := os.Getwd()
	if err != nil {
		return "(unknown)"
	}
	return dir
}
