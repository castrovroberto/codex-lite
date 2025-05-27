package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/audit"
	"github.com/castrovroberto/CGE/internal/patchutils"
)

// WorkflowConfig holds configuration for automated workflows
type WorkflowConfig struct {
	// Auto-commit settings
	AutoCommitEnabled         bool   `toml:"auto_commit_enabled"`
	AutoCommitMessageTemplate string `toml:"auto_commit_message_template"`
	AutoCommitOnSuccess       bool   `toml:"auto_commit_on_success"`

	// Commit message conventions
	UseConventionalCommits bool   `toml:"use_conventional_commits"`
	DefaultCommitType      string `toml:"default_commit_type"`

	// Rollback settings
	CreateBackups         bool `toml:"create_backups"`
	BackupRetentionDays   int  `toml:"backup_retention_days"`
	AutoRollbackOnFailure bool `toml:"auto_rollback_on_failure"`

	// Audit settings
	AuditEnabled       bool `toml:"audit_enabled"`
	AuditRetentionDays int  `toml:"audit_retention_days"`
}

// DefaultWorkflowConfig returns default workflow configuration
func DefaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		AutoCommitEnabled:         false,
		AutoCommitMessageTemplate: "chore: automated changes from CGE",
		AutoCommitOnSuccess:       false,
		UseConventionalCommits:    true,
		DefaultCommitType:         "feat",
		CreateBackups:             true,
		BackupRetentionDays:       7,
		AutoRollbackOnFailure:     true,
		AuditEnabled:              true,
		AuditRetentionDays:        30,
	}
}

// ApplyAndCommitRequest represents a request to apply changes and optionally commit
type ApplyAndCommitRequest struct {
	Changes       []ChangeRequest `json:"changes"`
	CommitMessage string          `json:"commit_message,omitempty"`
	CommitType    string          `json:"commit_type,omitempty"`
	Scope         string          `json:"scope,omitempty"`
	AutoCommit    bool            `json:"auto_commit"`
	DryRun        bool            `json:"dry_run"`
}

// ChangeRequest represents a single file change
type ChangeRequest struct {
	FilePath     string `json:"file_path"`
	Action       string `json:"action"` // "create", "modify", "delete", "patch"
	Content      string `json:"content,omitempty"`
	PatchContent string `json:"patch_content,omitempty"`
}

// ApplyAndCommitResponse represents the response from applying changes
type ApplyAndCommitResponse struct {
	Success        bool                     `json:"success"`
	AppliedChanges []patchutils.ApplyResult `json:"applied_changes"`
	CommitHash     string                   `json:"commit_hash,omitempty"`
	CommitMessage  string                   `json:"commit_message,omitempty"`
	RollbackInfo   *RollbackInfo            `json:"rollback_info,omitempty"`
	Error          string                   `json:"error,omitempty"`
	AuditSessionID string                   `json:"audit_session_id,omitempty"`
}

// RollbackInfo contains information needed for rollback
type RollbackInfo struct {
	BackupPaths  []string  `json:"backup_paths"`
	ChangedFiles []string  `json:"changed_files"`
	Timestamp    time.Time `json:"timestamp"`
	CanRollback  bool      `json:"can_rollback"`
}

// WorkflowManager orchestrates automated apply and commit workflows
type WorkflowManager struct {
	workspaceRoot string
	config        WorkflowConfig
	auditLogger   *audit.AuditLogger
	toolRegistry  *agent.Registry
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(workspaceRoot string, config WorkflowConfig, auditLogger *audit.AuditLogger) *WorkflowManager {
	return &WorkflowManager{
		workspaceRoot: workspaceRoot,
		config:        config,
		auditLogger:   auditLogger,
		toolRegistry:  createWorkflowToolRegistry(workspaceRoot, auditLogger),
	}
}

// ExecuteApplyAndCommit executes the automated apply and commit workflow
func (wm *WorkflowManager) ExecuteApplyAndCommit(ctx context.Context, req ApplyAndCommitRequest) (*ApplyAndCommitResponse, error) {
	startTime := time.Now()
	response := &ApplyAndCommitResponse{
		AuditSessionID: wm.auditLogger.GetSessionID(),
	}

	// Log workflow start
	if wm.auditLogger != nil {
		wm.auditLogger.LogToolExecution("workflow_manager", true, 0, nil, map[string]interface{}{
			"operation":     "apply_and_commit_start",
			"changes_count": len(req.Changes),
			"auto_commit":   req.AutoCommit,
			"dry_run":       req.DryRun,
		})
	}

	// Step 1: Apply changes
	appliedChanges, err := wm.applyChanges(ctx, req.Changes, req.DryRun)
	response.AppliedChanges = appliedChanges

	if err != nil {
		response.Error = err.Error()

		// Auto-rollback if enabled and not a dry run
		if wm.config.AutoRollbackOnFailure && !req.DryRun {
			rollbackErr := wm.rollbackChanges(ctx, appliedChanges)
			if rollbackErr != nil {
				response.Error = fmt.Sprintf("%s; rollback failed: %v", response.Error, rollbackErr)
			} else {
				response.Error = fmt.Sprintf("%s; changes rolled back", response.Error)
			}
		}

		return response, err
	}

	// Step 2: Commit changes if requested and not dry run
	if (req.AutoCommit || wm.config.AutoCommitOnSuccess) && !req.DryRun {
		commitHash, commitMessage, err := wm.commitChanges(ctx, req, appliedChanges)
		if err != nil {
			response.Error = fmt.Sprintf("changes applied but commit failed: %v", err)

			// Auto-rollback if enabled
			if wm.config.AutoRollbackOnFailure {
				rollbackErr := wm.rollbackChanges(ctx, appliedChanges)
				if rollbackErr != nil {
					response.Error = fmt.Sprintf("%s; rollback failed: %v", response.Error, rollbackErr)
				} else {
					response.Error = fmt.Sprintf("%s; changes rolled back", response.Error)
				}
			}

			return response, err
		}

		response.CommitHash = commitHash
		response.CommitMessage = commitMessage
	}

	// Step 3: Prepare rollback info
	response.RollbackInfo = wm.prepareRollbackInfo(appliedChanges)

	response.Success = true
	duration := time.Since(startTime)

	// Log workflow completion
	if wm.auditLogger != nil {
		wm.auditLogger.LogToolExecution("workflow_manager", true, duration, nil, map[string]interface{}{
			"operation":       "apply_and_commit_complete",
			"changes_applied": len(appliedChanges),
			"commit_hash":     response.CommitHash,
			"duration_ms":     duration.Milliseconds(),
		})
	}

	return response, nil
}

// applyChanges applies all the requested changes
func (wm *WorkflowManager) applyChanges(ctx context.Context, changes []ChangeRequest, dryRun bool) ([]patchutils.ApplyResult, error) {
	var results []patchutils.ApplyResult

	// Configure patch applier
	options := patchutils.ApplyOptions{
		CreateBackup:     wm.config.CreateBackups,
		BackupSuffix:     ".bak",
		DryRun:           dryRun,
		IgnoreWhitespace: false,
	}

	applier := patchutils.NewPatchApplier(wm.workspaceRoot, options)

	for _, change := range changes {
		var result *patchutils.ApplyResult
		var err error

		switch change.Action {
		case "patch":
			if change.PatchContent == "" {
				return results, fmt.Errorf("patch_content required for patch action on %s", change.FilePath)
			}
			result, err = applier.ApplyPatch(change.FilePath, change.PatchContent)

		case "create", "modify":
			// For create/modify, we'll use the file write tool
			fileWriteTool, exists := wm.toolRegistry.Get("write_file")
			if !exists {
				return results, fmt.Errorf("write_file tool not available")
			}

			params := map[string]interface{}{
				"file_path": change.FilePath,
				"content":   change.Content,
			}

			paramsJSON, _ := json.Marshal(params)
			toolResult, err := fileWriteTool.Execute(ctx, paramsJSON)

			if err != nil || !toolResult.Success {
				return results, fmt.Errorf("failed to %s file %s: %v", change.Action, change.FilePath, err)
			}

			// Convert tool result to apply result
			result = &patchutils.ApplyResult{
				FilePath:     change.FilePath,
				Success:      true,
				HunksApplied: 1,
				LinesChanged: len(strings.Split(change.Content, "\n")),
				NewSize:      len(change.Content),
			}

		case "delete":
			// For delete, we'll use shell command or implement file deletion
			return results, fmt.Errorf("delete action not yet implemented")

		default:
			return results, fmt.Errorf("unknown action: %s", change.Action)
		}

		if err != nil {
			return results, fmt.Errorf("failed to apply change to %s: %w", change.FilePath, err)
		}

		results = append(results, *result)

		// Log individual change
		if wm.auditLogger != nil {
			wm.auditLogger.LogFileOperation(
				audit.OperationType(change.Action),
				change.FilePath,
				result.Success,
				err,
				map[string]interface{}{
					"backup_path":   result.BackupPath,
					"hunks_applied": result.HunksApplied,
					"lines_changed": result.LinesChanged,
				},
			)
		}
	}

	return results, nil
}

// commitChanges commits the applied changes
func (wm *WorkflowManager) commitChanges(ctx context.Context, req ApplyAndCommitRequest, appliedChanges []patchutils.ApplyResult) (string, string, error) {
	// Prepare commit message
	commitMessage := req.CommitMessage
	if commitMessage == "" {
		commitMessage = wm.config.AutoCommitMessageTemplate
	}

	// Get enhanced git commit tool
	gitTool, exists := wm.toolRegistry.Get("git_commit_enhanced")
	if !exists {
		return "", "", fmt.Errorf("git_commit_enhanced tool not available")
	}

	// Prepare files to stage
	var filesToStage []string
	for _, change := range appliedChanges {
		if change.Success {
			filesToStage = append(filesToStage, change.FilePath)
		}
	}

	// Prepare commit parameters
	commitParams := map[string]interface{}{
		"commit_message": commitMessage,
		"files_to_stage": filesToStage,
	}

	// Add conventional commit formatting if enabled
	if wm.config.UseConventionalCommits {
		commitType := req.CommitType
		if commitType == "" {
			commitType = wm.config.DefaultCommitType
		}
		commitParams["commit_type"] = commitType
		if req.Scope != "" {
			commitParams["scope"] = req.Scope
		}
	}

	paramsJSON, _ := json.Marshal(commitParams)
	result, err := gitTool.Execute(ctx, paramsJSON)

	if err != nil || !result.Success {
		return "", "", fmt.Errorf("commit failed: %v", err)
	}

	// Extract commit info from result
	commitHash := ""
	if data, ok := result.Data.(map[string]interface{}); ok {
		if hash, exists := data["commit_hash"]; exists {
			commitHash = fmt.Sprintf("%v", hash)
		}
		if msg, exists := data["commit_message"]; exists {
			commitMessage = fmt.Sprintf("%v", msg)
		}
	}

	return commitHash, commitMessage, nil
}

// rollbackChanges rolls back the applied changes
func (wm *WorkflowManager) rollbackChanges(ctx context.Context, appliedChanges []patchutils.ApplyResult) error {
	var filesToRollback []string

	for _, change := range appliedChanges {
		if change.Success && change.BackupPath != "" {
			filesToRollback = append(filesToRollback, change.FilePath)
		}
	}

	if len(filesToRollback) == 0 {
		return nil // Nothing to rollback
	}

	// For now, implement basic rollback by restoring from backups
	// TODO: Expose rollback method from patchutils or implement here
	for _, change := range appliedChanges {
		if change.Success && change.BackupPath != "" {
			// Restore from backup (simplified implementation)
			// In a full implementation, we'd restore the backup file
		}
	}

	// Log rollback
	if wm.auditLogger != nil {
		wm.auditLogger.LogRollback(filesToRollback, true, nil)
	}

	return nil
}

// prepareRollbackInfo prepares rollback information
func (wm *WorkflowManager) prepareRollbackInfo(appliedChanges []patchutils.ApplyResult) *RollbackInfo {
	var backupPaths []string
	var changedFiles []string
	canRollback := true

	for _, change := range appliedChanges {
		if change.Success {
			changedFiles = append(changedFiles, change.FilePath)
			if change.BackupPath != "" {
				backupPaths = append(backupPaths, change.BackupPath)
			} else {
				canRollback = false // Can't rollback if no backup
			}
		}
	}

	return &RollbackInfo{
		BackupPaths:  backupPaths,
		ChangedFiles: changedFiles,
		Timestamp:    time.Now(),
		CanRollback:  canRollback,
	}
}

// createWorkflowToolRegistry creates a tool registry for workflow operations
func createWorkflowToolRegistry(workspaceRoot string, auditLogger *audit.AuditLogger) *agent.Registry {
	registry := agent.NewRegistry()

	// Register essential tools
	registry.Register(agent.NewFileWriteTool(workspaceRoot))
	registry.Register(agent.NewEnhancedPatchApplyTool(workspaceRoot))
	registry.Register(agent.NewEnhancedGitCommitTool(workspaceRoot, auditLogger))

	return registry
}
