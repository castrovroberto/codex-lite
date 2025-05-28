package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/castrovroberto/CGE/internal/security"
)

// ChatHistory represents the persistent chat history
type ChatHistory struct {
	SessionID    string                 `json:"session_id"`
	ModelName    string                 `json:"model_name"`
	Messages     []chatMessage          `json:"messages"`
	ToolCalls    []ToolCallRecord       `json:"tool_calls,omitempty"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Command      string                 `json:"command,omitempty"`
	SystemPrompt string                 `json:"system_prompt,omitempty"`
}

// ToolCallRecord represents a tool call in chat history
type ToolCallRecord struct {
	ID           string          `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	ToolName     string          `json:"tool_name"`
	Parameters   json.RawMessage `json:"parameters"`
	Result       *ToolCallResult `json:"result,omitempty"`
	Duration     time.Duration   `json:"duration"`
	Success      bool            `json:"success"`
	Error        string          `json:"error,omitempty"`
	MessageIndex int             `json:"message_index"` // Index of related message
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SaveHistory saves the current chat history to a file
func (m *Model) SaveHistory() error {
	now := time.Now() // Get current time once
	history := ChatHistory{
		SessionID: m.header.GetSessionID(),
		ModelName: m.header.GetModelName(),
		Messages:  m.messageList.GetMessages(),
		ToolCalls: []ToolCallRecord{}, // Initialize empty, will be populated if available
		StartTime: m.chatStartTime,    // Use the actual chat start time from the model
		EndTime:   &now,               // Set the end time to when history is saved
		Metadata:  make(map[string]interface{}),
		Command:   "chat",
	}

	// Create history directory if it doesn't exist
	historyDir := filepath.Join(os.Getenv("HOME"), ".cge", "chat_history")
	if err := os.MkdirAll(historyDir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Create history file with timestamp
	filename := fmt.Sprintf("chat_%s.json", m.header.GetSessionID())
	filepath := filepath.Join(historyDir, filename)

	// Marshal history to JSON
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chat history: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write chat history: %w", err)
	}

	return nil
}

// LoadHistory loads chat history from a file
func LoadHistory(sessionID string) (*ChatHistory, error) {
	historyDir := filepath.Join(os.Getenv("HOME"), ".cge", "chat_history")
	filepath := filepath.Join(historyDir, fmt.Sprintf("chat_%s.json", sessionID))

	// Create safe file operations with history directory as allowed root
	safeOps := security.NewSafeFileOps(historyDir)

	data, err := safeOps.SafeReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chat history: %w", err)
	}

	var history ChatHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat history: %w", err)
	}

	return &history, nil
}

// ListChatSessions returns a list of available chat session IDs
func ListChatSessions() ([]string, error) {
	historyDir := filepath.Join(os.Getenv("HOME"), ".cge", "chat_history")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(historyDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			// Extract session ID from filename
			sessionID := entry.Name()
			sessionID = sessionID[5:]                // Remove "chat_" prefix
			sessionID = sessionID[:len(sessionID)-5] // Remove ".json" suffix
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

// LoadLatestHistory loads the most recent chat history
func LoadLatestHistory() (*ChatHistory, error) {
	sessions, err := ListChatSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no chat history found")
	}

	// Find the most recent session by parsing timestamps
	var latestSession string
	var latestTime time.Time

	for _, session := range sessions {
		sessionTime, err := time.Parse("2006-01-02 15:04:05", session)
		if err != nil {
			continue
		}

		if sessionTime.After(latestTime) {
			latestTime = sessionTime
			latestSession = session
		}
	}

	if latestSession == "" {
		return nil, fmt.Errorf("no valid chat history found")
	}

	return LoadHistory(latestSession)
}
