package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/castrovroberto/CGE/internal/audit"
	"github.com/google/uuid"
)

// SessionState represents the complete state of an agent session
type SessionState struct {
	SessionID     string                 `json:"session_id"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	SystemPrompt  string                 `json:"system_prompt"`
	Model         string                 `json:"model"`
	Config        *RunConfig             `json:"config"`
	Messages      []Message              `json:"messages"`
	ToolCalls     []ToolCallRecord       `json:"tool_calls"`
	CurrentState  string                 `json:"current_state"` // "running", "completed", "failed", "paused"
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	WorkspaceRoot string                 `json:"workspace_root"`
	Command       string                 `json:"command"` // "plan", "generate", "review", "chat"
}

// ToolCallRecord represents a detailed record of a tool call
type ToolCallRecord struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	Parameters json.RawMessage        `json:"parameters"`
	Result     *ToolCallResult        `json:"result,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Iteration  int                    `json:"iteration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SessionManager handles persistence and management of agent sessions
type SessionManager struct {
	workspaceRoot string
	sessionDir    string
	auditLogger   *audit.AuditLogger
}

// NewSessionManager creates a new session manager
func NewSessionManager(workspaceRoot string, auditLogger *audit.AuditLogger) (*SessionManager, error) {
	sessionDir := filepath.Join(workspaceRoot, ".cge", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	return &SessionManager{
		workspaceRoot: workspaceRoot,
		sessionDir:    sessionDir,
		auditLogger:   auditLogger,
	}, nil
}

// CreateSession creates a new session state
func (sm *SessionManager) CreateSession(systemPrompt, model, command string, config *RunConfig) *SessionState {
	sessionID := uuid.New().String()

	return &SessionState{
		SessionID:     sessionID,
		StartTime:     time.Now(),
		SystemPrompt:  systemPrompt,
		Model:         model,
		Config:        config,
		Messages:      []Message{},
		ToolCalls:     []ToolCallRecord{},
		CurrentState:  "running",
		Metadata:      make(map[string]interface{}),
		WorkspaceRoot: sm.workspaceRoot,
		Command:       command,
	}
}

// SaveSession saves a session state to disk
func (sm *SessionManager) SaveSession(session *SessionState) error {
	filename := fmt.Sprintf("session_%s.json", session.SessionID)
	filepath := filepath.Join(sm.sessionDir, filename)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	// Log session save event
	if sm.auditLogger != nil {
		sm.auditLogger.LogToolExecution("session_manager", true, 0, nil, map[string]interface{}{
			"operation":  "save_session",
			"session_id": session.SessionID,
			"tool_calls": len(session.ToolCalls),
			"messages":   len(session.Messages),
			"state":      session.CurrentState,
		})
	}

	return nil
}

// LoadSession loads a session state from disk
func (sm *SessionManager) LoadSession(sessionID string) (*SessionState, error) {
	filename := fmt.Sprintf("session_%s.json", sessionID)
	filepath := filepath.Join(sm.sessionDir, filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
	}

	// Log session load event
	if sm.auditLogger != nil {
		sm.auditLogger.LogToolExecution("session_manager", true, 0, nil, map[string]interface{}{
			"operation":  "load_session",
			"session_id": sessionID,
			"tool_calls": len(session.ToolCalls),
			"messages":   len(session.Messages),
			"state":      session.CurrentState,
		})
	}

	return &session, nil
}

// ListSessions returns a list of available session IDs
func (sm *SessionManager) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(sm.sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			// Extract session ID from filename
			sessionID := entry.Name()
			if len(sessionID) > 13 && sessionID[:8] == "session_" {
				sessionID = sessionID[8 : len(sessionID)-5] // Remove "session_" prefix and ".json" suffix
				sessions = append(sessions, sessionID)
			}
		}
	}

	return sessions, nil
}

// GetSessionInfo returns basic information about a session without loading the full state
func (sm *SessionManager) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	session, err := sm.LoadSession(sessionID)
	if err != nil {
		return nil, err
	}

	return &SessionInfo{
		SessionID:    session.SessionID,
		StartTime:    session.StartTime,
		EndTime:      session.EndTime,
		Model:        session.Model,
		Command:      session.Command,
		CurrentState: session.CurrentState,
		ToolCalls:    len(session.ToolCalls),
		Messages:     len(session.Messages),
	}, nil
}

// SessionInfo represents basic session information
type SessionInfo struct {
	SessionID    string     `json:"session_id"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Model        string     `json:"model"`
	Command      string     `json:"command"`
	CurrentState string     `json:"current_state"`
	ToolCalls    int        `json:"tool_calls"`
	Messages     int        `json:"messages"`
}

// DeleteSession removes a session from disk
func (sm *SessionManager) DeleteSession(sessionID string) error {
	filename := fmt.Sprintf("session_%s.json", sessionID)
	filepath := filepath.Join(sm.sessionDir, filename)

	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Log session deletion
	if sm.auditLogger != nil {
		sm.auditLogger.LogToolExecution("session_manager", true, 0, nil, map[string]interface{}{
			"operation":  "delete_session",
			"session_id": sessionID,
		})
	}

	return nil
}

// CleanupOldSessions removes sessions older than the specified duration
func (sm *SessionManager) CleanupOldSessions(maxAge time.Duration) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	deletedCount := 0

	for _, sessionID := range sessions {
		info, err := sm.GetSessionInfo(sessionID)
		if err != nil {
			continue // Skip sessions we can't read
		}

		if info.StartTime.Before(cutoff) {
			if err := sm.DeleteSession(sessionID); err == nil {
				deletedCount++
			}
		}
	}

	// Log cleanup operation
	if sm.auditLogger != nil {
		sm.auditLogger.LogToolExecution("session_manager", true, 0, nil, map[string]interface{}{
			"operation":     "cleanup_sessions",
			"deleted_count": deletedCount,
			"max_age_hours": maxAge.Hours(),
		})
	}

	return nil
}

// AddToolCall adds a tool call record to a session
func (sm *SessionManager) AddToolCall(session *SessionState, toolCall ToolCallRecord) {
	session.ToolCalls = append(session.ToolCalls, toolCall)
}

// UpdateSessionState updates the current state of a session
func (sm *SessionManager) UpdateSessionState(session *SessionState, state string) {
	session.CurrentState = state
	if state == "completed" || state == "failed" {
		now := time.Now()
		session.EndTime = &now
	}
}

// GetToolCallHistory returns the tool call history for a session
func (sm *SessionManager) GetToolCallHistory(sessionID string) ([]ToolCallRecord, error) {
	session, err := sm.LoadSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.ToolCalls, nil
}

// ExportSessionToJSONL exports a session's tool calls to JSONL format for analysis
func (sm *SessionManager) ExportSessionToJSONL(sessionID string, outputPath string) error {
	session, err := sm.LoadSession(sessionID)
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, toolCall := range session.ToolCalls {
		if err := encoder.Encode(toolCall); err != nil {
			return fmt.Errorf("failed to encode tool call: %w", err)
		}
	}

	return nil
}
