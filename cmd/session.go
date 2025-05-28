package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/audit"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
	"github.com/spf13/cobra"
)

var (
	sessionListAll     bool
	sessionCleanupDays int
	sessionExportPath  string
	sessionCommand     string
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage agent sessions",
	Long: `Manage agent sessions including listing, resuming, pausing, and cleaning up old sessions.

Sessions store the complete state of agent interactions including:
- Message history
- Tool call records
- Agent configuration
- Execution state

Examples:
  CGE session list                    # List recent sessions
  CGE session list --all              # List all sessions
  CGE session resume <session-id>     # Resume a specific session
  CGE session info <session-id>       # Show session information
  CGE session export <session-id>     # Export session to JSONL
  CGE session cleanup --days 30       # Clean up sessions older than 30 days`,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available sessions",
	Long:  `List available agent sessions with basic information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-list")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		// List sessions
		sessions, err := sessionManager.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		fmt.Printf("Found %d session(s):\n\n", len(sessions))

		// Display session information
		for _, sessionID := range sessions {
			info, err := sessionManager.GetSessionInfo(sessionID)
			if err != nil {
				fmt.Printf("❌ %s (error loading: %v)\n", sessionID, err)
				continue
			}

			status := getStatusIcon(info.CurrentState)
			duration := ""
			if info.EndTime != nil {
				duration = fmt.Sprintf(" (%.1fs)", info.EndTime.Sub(info.StartTime).Seconds())
			} else {
				duration = fmt.Sprintf(" (%.1fs+)", time.Since(info.StartTime).Seconds())
			}

			fmt.Printf("%s %s\n", status, sessionID)
			fmt.Printf("   Command: %s | Model: %s | State: %s%s\n",
				info.Command, info.Model, info.CurrentState, duration)
			fmt.Printf("   Started: %s | Messages: %d | Tool Calls: %d\n",
				info.StartTime.Format("2006-01-02 15:04:05"), info.Messages, info.ToolCalls)
			fmt.Println()
		}

		return nil
	},
}

var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume a paused session",
	Long:  `Resume a paused agent session and continue execution.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		sessionID := args[0]

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-resume")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		// Load session
		session, err := sessionManager.LoadSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}

		fmt.Printf("Resuming session: %s\n", sessionID)
		fmt.Printf("Command: %s | Model: %s | State: %s\n",
			session.Command, session.Model, session.CurrentState)
		fmt.Printf("Messages: %d | Tool Calls: %d\n",
			len(session.Messages), len(session.ToolCalls))

		// Initialize LLM client
		var llmClient llm.Client
		switch cfg.LLM.Provider {
		case "ollama":
			ollamaConfig := cfg.GetOllamaConfig()
			llmClient = llm.NewOllamaClient(ollamaConfig)
		case "openai":
			openaiConfig := cfg.GetOpenAIConfig()
			llmClient = llm.NewOpenAIClient(openaiConfig)
		default:
			return fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
		}

		// Initialize tool registry based on session command
		toolFactory := agent.NewToolFactory(workspaceRoot)
		var toolRegistry *agent.Registry
		switch session.Command {
		case "plan":
			toolRegistry = toolFactory.CreatePlanningRegistry()
		case "generate":
			toolRegistry = toolFactory.CreateGenerationRegistry()
		case "review":
			toolRegistry = toolFactory.CreateReviewRegistry()
		default:
			toolRegistry = toolFactory.CreateGenerationRegistry() // Default
		}

		// Create agent runner with session
		runner := orchestrator.NewAgentRunnerWithSession(
			llmClient, toolRegistry, session.SystemPrompt, session.Model, sessionManager)

		// Resume the session
		if err := runner.ResumeSession(sessionID); err != nil {
			return fmt.Errorf("failed to resume session: %w", err)
		}

		// Continue execution with a continuation prompt
		continuationPrompt := "Please continue from where we left off."
		if sessionCommand != "" {
			continuationPrompt = sessionCommand
		}

		fmt.Printf("\nContinuing session with prompt: %s\n\n", continuationPrompt)

		result, err := runner.RunWithCommand(ctx, continuationPrompt, session.Command)
		if err != nil {
			return fmt.Errorf("session execution failed: %w", err)
		}

		// Display results
		fmt.Printf("\n📊 Session Resume Results:\n")
		fmt.Printf("Success: %t\n", result.Success)
		fmt.Printf("Total Iterations: %d\n", result.Iterations)
		fmt.Printf("Total Tool Calls: %d\n", result.ToolCalls)

		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}

		fmt.Printf("\n💬 Final Response:\n%s\n", result.FinalResponse)

		return nil
	},
}

var sessionInfoCmd = &cobra.Command{
	Use:   "info <session-id>",
	Short: "Show detailed session information",
	Long:  `Show detailed information about a specific session including tool call history.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		sessionID := args[0]

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-info")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		// Load session
		session, err := sessionManager.LoadSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}

		// Display session information
		fmt.Printf("Session Information: %s\n", sessionID)
		fmt.Printf("═══════════════════════════════════════\n\n")

		fmt.Printf("📋 Basic Info:\n")
		fmt.Printf("  Command: %s\n", session.Command)
		fmt.Printf("  Model: %s\n", session.Model)
		fmt.Printf("  State: %s\n", session.CurrentState)
		fmt.Printf("  Workspace: %s\n", session.WorkspaceRoot)
		fmt.Printf("\n")

		fmt.Printf("⏰ Timing:\n")
		fmt.Printf("  Started: %s\n", session.StartTime.Format("2006-01-02 15:04:05"))
		if session.EndTime != nil {
			fmt.Printf("  Ended: %s\n", session.EndTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Duration: %.1fs\n", session.EndTime.Sub(session.StartTime).Seconds())
		} else {
			fmt.Printf("  Duration: %.1fs (ongoing)\n", time.Since(session.StartTime).Seconds())
		}
		fmt.Printf("\n")

		fmt.Printf("📊 Statistics:\n")
		fmt.Printf("  Messages: %d\n", len(session.Messages))
		fmt.Printf("  Tool Calls: %d\n", len(session.ToolCalls))

		// Tool call statistics
		toolStats := make(map[string]int)
		successCount := 0
		totalDuration := time.Duration(0)

		for _, toolCall := range session.ToolCalls {
			toolStats[toolCall.ToolName]++
			if toolCall.Success {
				successCount++
			}
			totalDuration += toolCall.Duration
		}

		if len(session.ToolCalls) > 0 {
			fmt.Printf("  Tool Success Rate: %.1f%%\n", float64(successCount)/float64(len(session.ToolCalls))*100)
			fmt.Printf("  Average Tool Duration: %.2fs\n", totalDuration.Seconds()/float64(len(session.ToolCalls)))
		}
		fmt.Printf("\n")

		if len(toolStats) > 0 {
			fmt.Printf("🔧 Tool Usage:\n")
			for toolName, count := range toolStats {
				fmt.Printf("  %s: %d calls\n", toolName, count)
			}
			fmt.Printf("\n")
		}

		if len(session.ToolCalls) > 0 {
			fmt.Printf("📝 Recent Tool Calls (last 5):\n")
			start := len(session.ToolCalls) - 5
			if start < 0 {
				start = 0
			}

			for i := start; i < len(session.ToolCalls); i++ {
				toolCall := session.ToolCalls[i]
				status := "✅"
				if !toolCall.Success {
					status = "❌"
				}

				fmt.Printf("  %s %s (%s) - %.2fs\n",
					status, toolCall.ToolName,
					toolCall.Timestamp.Format("15:04:05"),
					toolCall.Duration.Seconds())
			}
		}

		return nil
	},
}

var sessionExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export session to JSONL format",
	Long:  `Export a session's tool call history to JSONL format for analysis.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		sessionID := args[0]

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-export")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		// Determine output path
		outputPath := sessionExportPath
		if outputPath == "" {
			outputPath = fmt.Sprintf("session_%s_export.jsonl", sessionID[:8])
		}

		// Make path absolute
		if !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(workspaceRoot, outputPath)
		}

		// Export session
		if err := sessionManager.ExportSessionToJSONL(sessionID, outputPath); err != nil {
			return fmt.Errorf("failed to export session: %w", err)
		}

		fmt.Printf("Session exported to: %s\n", outputPath)
		return nil
	},
}

var sessionAnalyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Generate analytics report for sessions",
	Long:  `Generate a comprehensive analytics report showing session statistics, tool usage patterns, and performance insights.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-analytics")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		// Create analytics instance
		analytics := orchestrator.NewSessionAnalytics(sessionManager)

		// Generate report
		fmt.Println("Generating analytics report...")
		report, err := analytics.GenerateReport()
		if err != nil {
			return fmt.Errorf("failed to generate analytics report: %w", err)
		}

		// Display report
		fmt.Printf("\n📊 Session Analytics Report\n")
		fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("═══════════════════════════════════════\n\n")

		fmt.Printf("📈 Overview:\n")
		fmt.Printf("  Total Sessions: %d\n", report.TotalSessions)
		fmt.Printf("  Overall Success Rate: %.1f%%\n", report.PerformanceStats.OverallSuccessRate)
		fmt.Printf("  Average Session Duration: %.1f minutes\n", report.PerformanceStats.AverageSessionDuration.Minutes())
		fmt.Printf("  Total Tool Calls: %d\n", report.PerformanceStats.TotalToolCalls)
		fmt.Printf("  Average Tool Calls per Session: %.1f\n", report.PerformanceStats.AverageToolCallsPerSession)
		fmt.Printf("\n")

		if len(report.SessionsByCommand) > 0 {
			fmt.Printf("📋 Sessions by Command:\n")
			for command, count := range report.SessionsByCommand {
				fmt.Printf("  %s: %d\n", command, count)
			}
			fmt.Printf("\n")
		}

		if len(report.SessionsByState) > 0 {
			fmt.Printf("🔄 Sessions by State:\n")
			for state, count := range report.SessionsByState {
				fmt.Printf("  %s: %d\n", state, count)
			}
			fmt.Printf("\n")
		}

		if len(report.ToolUsageStats) > 0 {
			fmt.Printf("🔧 Top Tool Usage:\n")
			for i, tool := range report.ToolUsageStats {
				if i >= 5 { // Show top 5 tools
					break
				}
				fmt.Printf("  %s: %d calls (%.1f%% success, avg %.2fs)\n",
					tool.ToolName, tool.TotalCalls, tool.SuccessRate, tool.AverageDuration.Seconds())
			}
			fmt.Printf("\n")
		}

		if len(report.Insights) > 0 {
			fmt.Printf("💡 Insights:\n")
			for _, insight := range report.Insights {
				fmt.Printf("  %s\n", insight)
			}
			fmt.Printf("\n")
		}

		if len(report.RecentSessions) > 0 {
			fmt.Printf("📝 Recent Sessions:\n")
			for _, session := range report.RecentSessions {
				status := getStatusIcon(session.State)
				fmt.Printf("  %s %s (%s) - %s\n",
					status, session.SessionID[:8], session.Command,
					session.StartTime.Format("2006-01-02 15:04"))
			}
		}

		return nil
	},
}

var sessionCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old sessions",
	Long:  `Remove sessions older than the specified number of days.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := contextkeys.ConfigFromContext(ctx)
		logger := contextkeys.LoggerFromContext(ctx)

		// Get workspace root
		workspaceRoot := cfg.Project.WorkspaceRoot
		if workspaceRoot == "" {
			var err error
			workspaceRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Initialize audit logger
		auditLogger, err := audit.NewAuditLogger(workspaceRoot, "session-cleanup")
		if err != nil {
			logger.Warn("Failed to initialize audit logger", "error", err)
		}
		defer func() {
			if auditLogger != nil {
				auditLogger.Close()
			}
		}()

		// Initialize session manager
		sessionManager, err := orchestrator.NewSessionManager(workspaceRoot, auditLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		maxAge := time.Duration(sessionCleanupDays) * 24 * time.Hour
		fmt.Printf("Cleaning up sessions older than %d days...\n", sessionCleanupDays)

		if err := sessionManager.CleanupOldSessions(maxAge); err != nil {
			return fmt.Errorf("failed to cleanup sessions: %w", err)
		}

		fmt.Println("Session cleanup completed.")
		return nil
	},
}

func getStatusIcon(state string) string {
	switch state {
	case "running":
		return "🔄"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "paused":
		return "⏸️"
	default:
		return "❓"
	}
}

func init() {
	// Add subcommands
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionResumeCmd)
	sessionCmd.AddCommand(sessionInfoCmd)
	sessionCmd.AddCommand(sessionExportCmd)
	sessionCmd.AddCommand(sessionAnalyticsCmd)
	sessionCmd.AddCommand(sessionCleanupCmd)

	// Flags for list command
	sessionListCmd.Flags().BoolVar(&sessionListAll, "all", false, "List all sessions (including completed)")

	// Flags for resume command
	sessionResumeCmd.Flags().StringVar(&sessionCommand, "command", "", "Custom command to continue with")

	// Flags for export command
	sessionExportCmd.Flags().StringVar(&sessionExportPath, "output", "", "Output file path (default: session_<id>_export.jsonl)")

	// Flags for cleanup command
	sessionCleanupCmd.Flags().IntVar(&sessionCleanupDays, "days", 30, "Remove sessions older than this many days")

	// Add to root command
	rootCmd.AddCommand(sessionCmd)
}
