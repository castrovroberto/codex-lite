package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of audit event
type EventType string

const (
	EventToolExecution EventType = "tool_execution"
	EventFileOperation EventType = "file_operation"
	EventPatchApply    EventType = "patch_apply"
	EventGitCommit     EventType = "git_commit"
	EventRollback      EventType = "rollback"
	EventError         EventType = "error"
)

// OperationType represents the type of operation performed
type OperationType string

const (
	OpCreate   OperationType = "create"
	OpModify   OperationType = "modify"
	OpDelete   OperationType = "delete"
	OpRead     OperationType = "read"
	OpPatch    OperationType = "patch"
	OpCommit   OperationType = "commit"
	OpRollback OperationType = "rollback"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	ID             string                 `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	SessionID      string                 `json:"session_id"`
	EventType      EventType              `json:"event_type"`
	Operation      OperationType          `json:"operation"`
	ToolName       string                 `json:"tool_name,omitempty"`
	FilePath       string                 `json:"file_path,omitempty"`
	Success        bool                   `json:"success"`
	Error          string                 `json:"error,omitempty"`
	Duration       time.Duration          `json:"duration"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	BackupPath     string                 `json:"backup_path,omitempty"`
	ChangesSummary string                 `json:"changes_summary,omitempty"`
}

// AuditLogger handles structured logging of all operations
type AuditLogger struct {
	sessionID string
	logDir    string
	logFile   *os.File
	enabled   bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(workspaceRoot string, sessionID string) (*AuditLogger, error) {
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	logDir := filepath.Join(workspaceRoot, ".cge", "audit")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	logFileName := fmt.Sprintf("audit_%s_%s.jsonl",
		time.Now().Format("2006-01-02"), sessionID[:8])
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &AuditLogger{
		sessionID: sessionID,
		logDir:    logDir,
		logFile:   logFile,
		enabled:   true,
	}, nil
}

// LogToolExecution logs a tool execution event
func (al *AuditLogger) LogToolExecution(toolName string, success bool, duration time.Duration, err error, metadata map[string]interface{}) {
	if !al.enabled {
		return
	}

	event := AuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		SessionID: al.sessionID,
		EventType: EventToolExecution,
		Operation: OpRead, // Default, can be overridden in metadata
		ToolName:  toolName,
		Success:   success,
		Duration:  duration,
		Metadata:  metadata,
	}

	if err != nil {
		event.Error = err.Error()
	}

	// Extract operation type from metadata if available
	if op, exists := metadata["operation"]; exists {
		if opStr, ok := op.(string); ok {
			event.Operation = OperationType(opStr)
		}
	}

	al.writeEvent(event)
}

// LogFileOperation logs a file operation event
func (al *AuditLogger) LogFileOperation(operation OperationType, filePath string, success bool, err error, metadata map[string]interface{}) {
	if !al.enabled {
		return
	}

	event := AuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		SessionID: al.sessionID,
		EventType: EventFileOperation,
		Operation: operation,
		FilePath:  filePath,
		Success:   success,
		Metadata:  metadata,
	}

	if err != nil {
		event.Error = err.Error()
	}

	// Extract backup path if available
	if backupPath, exists := metadata["backup_path"]; exists {
		if backupStr, ok := backupPath.(string); ok {
			event.BackupPath = backupStr
		}
	}

	al.writeEvent(event)
}

// LogPatchApply logs a patch application event
func (al *AuditLogger) LogPatchApply(filePath string, success bool, hunksApplied int, linesChanged int, backupPath string, err error) {
	if !al.enabled {
		return
	}

	metadata := map[string]interface{}{
		"hunks_applied": hunksApplied,
		"lines_changed": linesChanged,
	}

	event := AuditEvent{
		ID:             uuid.New().String(),
		Timestamp:      time.Now(),
		SessionID:      al.sessionID,
		EventType:      EventPatchApply,
		Operation:      OpPatch,
		FilePath:       filePath,
		Success:        success,
		Metadata:       metadata,
		BackupPath:     backupPath,
		ChangesSummary: fmt.Sprintf("%d hunks applied, %d lines changed", hunksApplied, linesChanged),
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.writeEvent(event)
}

// LogGitCommit logs a git commit event
func (al *AuditLogger) LogGitCommit(commitHash string, commitMessage string, filesStaged []string, success bool, err error) {
	if !al.enabled {
		return
	}

	metadata := map[string]interface{}{
		"commit_hash":    commitHash,
		"commit_message": commitMessage,
		"files_staged":   filesStaged,
		"files_count":    len(filesStaged),
	}

	event := AuditEvent{
		ID:             uuid.New().String(),
		Timestamp:      time.Now(),
		SessionID:      al.sessionID,
		EventType:      EventGitCommit,
		Operation:      OpCommit,
		Success:        success,
		Metadata:       metadata,
		ChangesSummary: fmt.Sprintf("Committed %d files: %s", len(filesStaged), commitMessage),
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.writeEvent(event)
}

// LogRollback logs a rollback event
func (al *AuditLogger) LogRollback(filePaths []string, success bool, err error) {
	if !al.enabled {
		return
	}

	metadata := map[string]interface{}{
		"files_rolled_back": filePaths,
		"files_count":       len(filePaths),
	}

	event := AuditEvent{
		ID:             uuid.New().String(),
		Timestamp:      time.Now(),
		SessionID:      al.sessionID,
		EventType:      EventRollback,
		Operation:      OpRollback,
		Success:        success,
		Metadata:       metadata,
		ChangesSummary: fmt.Sprintf("Rolled back %d files", len(filePaths)),
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.writeEvent(event)
}

// LogError logs an error event
func (al *AuditLogger) LogError(operation OperationType, context string, err error, metadata map[string]interface{}) {
	if !al.enabled {
		return
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["context"] = context

	event := AuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		SessionID: al.sessionID,
		EventType: EventError,
		Operation: operation,
		Success:   false,
		Error:     err.Error(),
		Metadata:  metadata,
	}

	al.writeEvent(event)
}

// writeEvent writes an event to the log file
func (al *AuditLogger) writeEvent(event AuditEvent) {
	if al.logFile == nil {
		return
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		// Can't log the logging error, just return
		return
	}

	al.logFile.Write(eventJSON)
	al.logFile.Write([]byte("\n"))
	al.logFile.Sync() // Ensure immediate write
}

// Close closes the audit logger
func (al *AuditLogger) Close() error {
	if al.logFile != nil {
		return al.logFile.Close()
	}
	return nil
}

// GetSessionID returns the current session ID
func (al *AuditLogger) GetSessionID() string {
	return al.sessionID
}

// SetEnabled enables or disables audit logging
func (al *AuditLogger) SetEnabled(enabled bool) {
	al.enabled = enabled
}

// GetLogFilePath returns the path to the current log file
func (al *AuditLogger) GetLogFilePath() string {
	if al.logFile == nil {
		return ""
	}
	return al.logFile.Name()
}

// QueryEvents reads and filters events from the audit log
func (al *AuditLogger) QueryEvents(filter func(AuditEvent) bool) ([]AuditEvent, error) {
	// This is a simple implementation - in production, you might want to use a database
	logFiles, err := filepath.Glob(filepath.Join(al.logDir, "audit_*.jsonl"))
	if err != nil {
		return nil, err
	}

	var events []AuditEvent
	for _, logFile := range logFiles {
		fileEvents, err := al.readEventsFromFile(logFile, filter)
		if err != nil {
			continue // Skip files we can't read
		}
		events = append(events, fileEvents...)
	}

	return events, nil
}

// readEventsFromFile reads events from a specific log file
func (al *AuditLogger) readEventsFromFile(filePath string, filter func(AuditEvent) bool) ([]AuditEvent, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var events []AuditEvent

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed lines
		}

		if filter == nil || filter(event) {
			events = append(events, event)
		}
	}

	return events, nil
}
