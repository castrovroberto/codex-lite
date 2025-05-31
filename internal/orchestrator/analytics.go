package orchestrator

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SessionAnalytics provides analytics and insights for sessions
type SessionAnalytics struct {
	sessionManager *SessionManager
}

// NewSessionAnalytics creates a new session analytics instance
func NewSessionAnalytics(sessionManager *SessionManager) *SessionAnalytics {
	return &SessionAnalytics{
		sessionManager: sessionManager,
	}
}

// AnalyticsReport represents a comprehensive analytics report
type AnalyticsReport struct {
	GeneratedAt       time.Time        `json:"generated_at"`
	TotalSessions     int              `json:"total_sessions"`
	SessionsByCommand map[string]int   `json:"sessions_by_command"`
	SessionsByState   map[string]int   `json:"sessions_by_state"`
	ToolUsageStats    []ToolUsageStat  `json:"tool_usage_stats"`
	PerformanceStats  PerformanceStats `json:"performance_stats"`
	RecentSessions    []SessionSummary `json:"recent_sessions"`
	Insights          []string         `json:"insights"`
}

// ToolUsageStat represents statistics for a specific tool
type ToolUsageStat struct {
	ToolName        string        `json:"tool_name"`
	TotalCalls      int           `json:"total_calls"`
	SuccessfulCalls int           `json:"successful_calls"`
	FailedCalls     int           `json:"failed_calls"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
}

// PerformanceStats represents overall performance statistics
type PerformanceStats struct {
	AverageSessionDuration     time.Duration `json:"average_session_duration"`
	AverageToolCallsPerSession float64       `json:"average_tool_calls_per_session"`
	AverageMessagesPerSession  float64       `json:"average_messages_per_session"`
	TotalToolCalls             int           `json:"total_tool_calls"`
	TotalMessages              int           `json:"total_messages"`
	OverallSuccessRate         float64       `json:"overall_success_rate"`
}

// SessionSummary represents a summary of a session
type SessionSummary struct {
	SessionID string        `json:"session_id"`
	Command   string        `json:"command"`
	State     string        `json:"state"`
	Duration  time.Duration `json:"duration"`
	ToolCalls int           `json:"tool_calls"`
	Messages  int           `json:"messages"`
	StartTime time.Time     `json:"start_time"`
}

// GenerateReport generates a comprehensive analytics report
func (sa *SessionAnalytics) GenerateReport() (*AnalyticsReport, error) {
	sessions, err := sa.sessionManager.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	report := &AnalyticsReport{
		GeneratedAt:       time.Now(),
		TotalSessions:     len(sessions),
		SessionsByCommand: make(map[string]int),
		SessionsByState:   make(map[string]int),
		ToolUsageStats:    []ToolUsageStat{},
		RecentSessions:    []SessionSummary{},
		Insights:          []string{},
	}

	// Collect data from all sessions
	var allSessions []*SessionState
	toolStats := make(map[string]*ToolUsageStat)
	var totalDuration time.Duration
	var completedSessions int
	var totalToolCalls int
	var totalMessages int
	var successfulSessions int

	for _, sessionID := range sessions {
		session, err := sa.sessionManager.LoadSession(sessionID)
		if err != nil {
			continue // Skip sessions we can't load
		}

		allSessions = append(allSessions, session)

		// Count by command and state
		report.SessionsByCommand[session.Command]++
		report.SessionsByState[session.CurrentState]++

		// Calculate session duration
		var sessionDuration time.Duration
		if session.EndTime != nil {
			sessionDuration = session.EndTime.Sub(session.StartTime)
			totalDuration += sessionDuration
			completedSessions++
		}

		// Count messages and tool calls
		totalMessages += len(session.Messages)
		totalToolCalls += len(session.ToolCalls)

		// Track success
		if session.CurrentState == "completed" {
			successfulSessions++
		}

		// Process tool calls
		for _, toolCall := range session.ToolCalls {
			if toolStats[toolCall.ToolName] == nil {
				toolStats[toolCall.ToolName] = &ToolUsageStat{
					ToolName: toolCall.ToolName,
				}
			}

			stat := toolStats[toolCall.ToolName]
			stat.TotalCalls++
			stat.TotalDuration += toolCall.Duration

			if toolCall.Success {
				stat.SuccessfulCalls++
			} else {
				stat.FailedCalls++
			}
		}

		// Add to recent sessions (we'll sort and limit later)
		report.RecentSessions = append(report.RecentSessions, SessionSummary{
			SessionID: session.SessionID,
			Command:   session.Command,
			State:     session.CurrentState,
			Duration:  sessionDuration,
			ToolCalls: len(session.ToolCalls),
			Messages:  len(session.Messages),
			StartTime: session.StartTime,
		})
	}

	// Calculate tool usage statistics
	for _, stat := range toolStats {
		if stat.TotalCalls > 0 {
			stat.SuccessRate = float64(stat.SuccessfulCalls) / float64(stat.TotalCalls) * 100
			stat.AverageDuration = stat.TotalDuration / time.Duration(stat.TotalCalls)
		}
		report.ToolUsageStats = append(report.ToolUsageStats, *stat)
	}

	// Sort tool usage stats by total calls (descending)
	sort.Slice(report.ToolUsageStats, func(i, j int) bool {
		return report.ToolUsageStats[i].TotalCalls > report.ToolUsageStats[j].TotalCalls
	})

	// Sort recent sessions by start time (descending) and limit to 10
	sort.Slice(report.RecentSessions, func(i, j int) bool {
		return report.RecentSessions[i].StartTime.After(report.RecentSessions[j].StartTime)
	})
	if len(report.RecentSessions) > 10 {
		report.RecentSessions = report.RecentSessions[:10]
	}

	// Calculate performance statistics
	if len(sessions) > 0 {
		report.PerformanceStats.AverageToolCallsPerSession = float64(totalToolCalls) / float64(len(sessions))
		report.PerformanceStats.AverageMessagesPerSession = float64(totalMessages) / float64(len(sessions))
		report.PerformanceStats.TotalToolCalls = totalToolCalls
		report.PerformanceStats.TotalMessages = totalMessages
		report.PerformanceStats.OverallSuccessRate = float64(successfulSessions) / float64(len(sessions)) * 100
	}

	if completedSessions > 0 {
		report.PerformanceStats.AverageSessionDuration = totalDuration / time.Duration(completedSessions)
	}

	// Generate insights
	report.Insights = sa.generateInsights(report)

	return report, nil
}

// generateInsights generates actionable insights from the analytics data
func (sa *SessionAnalytics) generateInsights(report *AnalyticsReport) []string {
	var insights []string

	// Success rate insights
	if report.PerformanceStats.OverallSuccessRate < 70 {
		insights = append(insights, fmt.Sprintf("⚠️  Low success rate (%.1f%%). Consider reviewing failed sessions for common issues.", report.PerformanceStats.OverallSuccessRate))
	} else if report.PerformanceStats.OverallSuccessRate > 90 {
		insights = append(insights, fmt.Sprintf("✅ Excellent success rate (%.1f%%)!", report.PerformanceStats.OverallSuccessRate))
	}

	// Tool usage insights
	if len(report.ToolUsageStats) > 0 {
		mostUsedTool := report.ToolUsageStats[0]
		insights = append(insights, fmt.Sprintf("🔧 Most used tool: %s (%d calls, %.1f%% success rate)",
			mostUsedTool.ToolName, mostUsedTool.TotalCalls, mostUsedTool.SuccessRate))

		// Find tools with low success rates
		for _, tool := range report.ToolUsageStats {
			if tool.TotalCalls >= 5 && tool.SuccessRate < 60 {
				insights = append(insights, fmt.Sprintf("⚠️  Tool '%s' has low success rate (%.1f%%) - may need attention",
					tool.ToolName, tool.SuccessRate))
			}
		}
	}

	// Session duration insights
	if report.PerformanceStats.AverageSessionDuration > 10*time.Minute {
		insights = append(insights, fmt.Sprintf("⏱️  Long average session duration (%.1f minutes). Consider optimizing workflows.",
			report.PerformanceStats.AverageSessionDuration.Minutes()))
	}

	// Command usage insights
	if len(report.SessionsByCommand) > 0 {
		var mostUsedCommand string
		var maxCount int
		for command, count := range report.SessionsByCommand {
			if count > maxCount {
				maxCount = count
				mostUsedCommand = command
			}
		}
		insights = append(insights, fmt.Sprintf("📊 Most used command: %s (%d sessions)", mostUsedCommand, maxCount))
	}

	// State distribution insights
	if paused, exists := report.SessionsByState["paused"]; exists && paused > 0 {
		insights = append(insights, fmt.Sprintf("⏸️  %d paused sessions found. Consider resuming or cleaning up.", paused))
	}

	if failed, exists := report.SessionsByState["failed"]; exists && failed > len(report.SessionsByState)/4 {
		insights = append(insights, fmt.Sprintf("❌ High number of failed sessions (%d). Review error patterns.", failed))
	}

	return insights
}

// GetToolPerformanceReport generates a detailed report for a specific tool
func (sa *SessionAnalytics) GetToolPerformanceReport(toolName string) (*ToolPerformanceReport, error) {
	sessions, err := sa.sessionManager.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	report := &ToolPerformanceReport{
		ToolName:    toolName,
		GeneratedAt: time.Now(),
		CallHistory: []ToolCallDetail{},
	}

	var totalCalls int
	var successfulCalls int
	var totalDuration time.Duration
	var errorPatterns = make(map[string]int)

	for _, sessionID := range sessions {
		session, err := sa.sessionManager.LoadSession(sessionID)
		if err != nil {
			continue
		}

		for _, toolCall := range session.ToolCalls {
			if toolCall.ToolName != toolName {
				continue
			}

			totalCalls++
			totalDuration += toolCall.Duration

			if toolCall.Success {
				successfulCalls++
			} else {
				if toolCall.Error != "" {
					errorPatterns[toolCall.Error]++
				}
			}

			report.CallHistory = append(report.CallHistory, ToolCallDetail{
				SessionID: session.SessionID,
				Timestamp: toolCall.Timestamp,
				Duration:  toolCall.Duration,
				Success:   toolCall.Success,
				Error:     toolCall.Error,
				Command:   session.Command,
			})
		}
	}

	// Calculate statistics
	if totalCalls > 0 {
		report.TotalCalls = totalCalls
		report.SuccessfulCalls = successfulCalls
		report.FailedCalls = totalCalls - successfulCalls
		report.SuccessRate = float64(successfulCalls) / float64(totalCalls) * 100
		report.AverageDuration = totalDuration / time.Duration(totalCalls)
	}

	// Sort error patterns by frequency
	for errorMsg, count := range errorPatterns {
		report.CommonErrors = append(report.CommonErrors, ErrorPattern{
			Error: errorMsg,
			Count: count,
		})
	}
	sort.Slice(report.CommonErrors, func(i, j int) bool {
		return report.CommonErrors[i].Count > report.CommonErrors[j].Count
	})

	// Sort call history by timestamp (most recent first)
	sort.Slice(report.CallHistory, func(i, j int) bool {
		return report.CallHistory[i].Timestamp.After(report.CallHistory[j].Timestamp)
	})

	return report, nil
}

// ToolPerformanceReport represents a detailed performance report for a specific tool
type ToolPerformanceReport struct {
	ToolName        string           `json:"tool_name"`
	GeneratedAt     time.Time        `json:"generated_at"`
	TotalCalls      int              `json:"total_calls"`
	SuccessfulCalls int              `json:"successful_calls"`
	FailedCalls     int              `json:"failed_calls"`
	SuccessRate     float64          `json:"success_rate"`
	AverageDuration time.Duration    `json:"average_duration"`
	CommonErrors    []ErrorPattern   `json:"common_errors"`
	CallHistory     []ToolCallDetail `json:"call_history"`
}

// ErrorPattern represents a common error pattern
type ErrorPattern struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

// ToolCallDetail represents detailed information about a tool call
type ToolCallDetail struct {
	SessionID string        `json:"session_id"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Command   string        `json:"command"`
}

// ExportReportToJSON exports an analytics report to JSON format
func (sa *SessionAnalytics) ExportReportToJSON(report *AnalyticsReport, outputPath string) error {
	_, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// TODO: Implement file writing functionality
	return fmt.Errorf("export functionality not yet implemented for path: %s", outputPath)
}

// GetSessionTrends analyzes trends over time
func (sa *SessionAnalytics) GetSessionTrends(days int) (*TrendAnalysis, error) {
	sessions, err := sa.sessionManager.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	trends := &TrendAnalysis{
		PeriodDays:  days,
		GeneratedAt: time.Now(),
		DailyStats:  make(map[string]DailyStats),
	}

	for _, sessionID := range sessions {
		session, err := sa.sessionManager.LoadSession(sessionID)
		if err != nil {
			continue
		}

		if session.StartTime.Before(cutoff) {
			continue
		}

		dayKey := session.StartTime.Format("2006-01-02")
		if trends.DailyStats[dayKey] == (DailyStats{}) {
			trends.DailyStats[dayKey] = DailyStats{
				Date: session.StartTime.Format("2006-01-02"),
			}
		}

		stats := trends.DailyStats[dayKey]
		stats.TotalSessions++
		stats.TotalToolCalls += len(session.ToolCalls)
		stats.TotalMessages += len(session.Messages)

		if session.CurrentState == "completed" {
			stats.SuccessfulSessions++
		}

		trends.DailyStats[dayKey] = stats
	}

	return trends, nil
}

// TrendAnalysis represents trend analysis over time
type TrendAnalysis struct {
	PeriodDays  int                   `json:"period_days"`
	GeneratedAt time.Time             `json:"generated_at"`
	DailyStats  map[string]DailyStats `json:"daily_stats"`
}

// DailyStats represents statistics for a single day
type DailyStats struct {
	Date               string `json:"date"`
	TotalSessions      int    `json:"total_sessions"`
	SuccessfulSessions int    `json:"successful_sessions"`
	TotalToolCalls     int    `json:"total_tool_calls"`
	TotalMessages      int    `json:"total_messages"`
}
