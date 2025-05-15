package cmd

import (
    "fmt"
    "os"
    "strings"

    "github.com/spf13/cobra"
    "github.com/castrovroberto/codex-lite/internal/agents"
)

var analyzeCmd = &cobra.Command{
    Use:   "analyze [file]",
    Short: "Analyze a code file with the ExplainAgent",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        path := args[0]
        data, err := os.ReadFile(path)
        if err != nil {
            fmt.Println("Error reading file:", err)
            return
        }

        agent := &agents.ExplainAgent{Model: "deepseek-coder-v2-lite"}
        result, err := agent.Analyze(path, string(data))
        if err != nil {
            fmt.Println("Error analyzing:", err)
            return
        }

        fmt.Printf("\n📘 [%s]:\n%s\n", result.File, strings.TrimSpace(result.Output))
    },
}

func init() {
    rootCmd.AddCommand(analyzeCmd)
}
