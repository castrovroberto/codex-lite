# 🔧 Automated Apply & Commit Workflow - Implementation Plan

## 📋 Overview

This document outlines the detailed implementation plan for **Task 6: Automated Apply & Commit Workflow** from the CGE priority tasks. The goal is to enhance the existing patch application and commit workflow to be more robust, automated, and reliable.

## 🎯 Objectives

1. **Improve `apply_patch` and `applyChangesToFiles`** using robust diff/patch libraries
2. **Support auto-commits and PR creation** workflows
3. **Add rollback handling and structured logging** for error recovery
4. **Implement automated workflow orchestration** with configurable policies

## 📊 Current State Analysis

### Existing Components
- ✅ **`PatchApplyTool`** - Custom patch parser and applier
- ✅ **`GitCommitTool`** - Basic Git commit functionality  
- ✅ **`applyChangesToFiles`** - Legacy file change application
- ✅ **`rollbackChanges`** - Basic rollback mechanism

### Identified Limitations
- ❌ Custom patch parser may not handle all edge cases
- ❌ Limited rollback granularity
- ❌ No structured logging for audit trails
- ❌ No automated commit workflows
- ❌ No PR creation capabilities

## 🚀 Implementation Progress

### ✅ Phase 1: Enhanced Patch Application System

#### 1.1 Robust Diff/Patch Library Integration
- **Added dependency**: `github.com/sourcegraph/go-diff@v0.7.0`
- **Created**: `internal/patchutils/applier.go` - Robust patch application using go-diff
- **Features**:
  - Multi-file diff parsing
  - Backup creation with timestamps
  - Dry-run support
  - Whitespace handling options
  - Atomic rollback on failure

#### 1.2 Enhanced Patch Apply Tool
- **Created**: `internal/agent/patch_apply_tool_enhanced.go`
- **Features**:
  - Uses robust go-diff library
  - Configurable backup options
  - Dry-run mode
  - Detailed result reporting
  - Better error handling

### ✅ Phase 2: Structured Logging and Audit Trail

#### 2.1 Audit Logger Implementation
- **Created**: `internal/audit/logger.go`
- **Features**:
  - Structured JSON logging (JSONL format)
  - Session-based tracking
  - Multiple event types (tool execution, file operations, patches, commits, rollbacks)
  - Queryable audit trail
  - Configurable retention policies

#### 2.2 Event Types Supported
- `tool_execution` - Tool invocations with timing
- `file_operation` - File create/modify/delete operations
- `patch_apply` - Patch application events
- `git_commit` - Git commit operations
- `rollback` - Rollback operations
- `error` - Error events with context

### ✅ Phase 3: Automated Commit Workflow

#### 3.1 Enhanced Git Commit Tool
- **Created**: `internal/agent/git_commit_enhanced_tool.go`
- **Features**:
  - Auto-generated commit messages based on changes
  - Conventional Commits format support
  - Co-author support
  - Smart file staging
  - Hook skipping options
  - Comprehensive audit logging

#### 3.2 Commit Message Features
- **Auto-generation**: Heuristic-based message generation
- **Conventional Commits**: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
- **Breaking changes**: Support for `!` notation
- **Scopes**: Optional scope specification
- **Co-authors**: Multiple author attribution

### ✅ Phase 4: Configuration Extension

#### 4.1 Configuration Schema
- **Extended**: `codex.toml` with `[commands.automated_workflow]` section
- **Settings**:
  ```toml
  [commands.automated_workflow]
  auto_commit_enabled = false
  auto_commit_message_template = "chore: automated changes from CGE"
  auto_commit_on_success = false
  use_conventional_commits = true
  default_commit_type = "feat"
  create_backups = true
  backup_retention_days = 7
  auto_rollback_on_failure = true
  audit_enabled = true
  audit_retention_days = 30
  ```

### ✅ Phase 5: Workflow Orchestrator

#### 5.1 Workflow Manager
- **Created**: `internal/workflow/manager.go`
- **Features**:
  - Unified apply-and-commit workflow
  - Configurable automation policies
  - Atomic operations with rollback
  - Comprehensive error handling
  - Audit trail integration

#### 5.2 Workflow API
```go
type ApplyAndCommitRequest struct {
    Changes       []ChangeRequest
    CommitMessage string
    CommitType    string
    Scope         string
    AutoCommit    bool
    DryRun        bool
}

type ChangeRequest struct {
    FilePath     string
    Action       string // "create", "modify", "delete", "patch"
    Content      string
    PatchContent string
}
```

## 🔄 Workflow Process

### Standard Apply & Commit Flow

1. **Validation Phase**
   - Validate all change requests
   - Check file paths are within workspace
   - Verify patch formats

2. **Backup Phase** (if enabled)
   - Create timestamped backups of existing files
   - Store backup metadata for rollback

3. **Apply Phase**
   - Apply changes sequentially
   - Log each operation to audit trail
   - Stop on first failure (with rollback)

4. **Commit Phase** (if auto-commit enabled)
   - Stage modified files
   - Generate or use provided commit message
   - Apply conventional commit formatting
   - Create Git commit

5. **Cleanup Phase**
   - Clean up temporary files
   - Log workflow completion
   - Return comprehensive results

### Error Handling & Rollback

- **Automatic Rollback**: On any failure, restore from backups
- **Audit Logging**: All operations logged for debugging
- **Granular Recovery**: Individual file rollback support
- **Error Context**: Detailed error messages with context

## 🛠️ Integration Points

### Tool Registry Integration
```go
// Enhanced tools registered in workflow
registry.Register(agent.NewFileWriteTool(workspaceRoot))
registry.Register(agent.NewEnhancedPatchApplyTool(workspaceRoot))
registry.Register(agent.NewEnhancedGitCommitTool(workspaceRoot, auditLogger))
```

### Agent Orchestrator Integration
- Workflow manager can be called from agent orchestrator
- Function-calling agents can use enhanced tools
- Audit trail provides debugging for agent operations

### Command Integration
- `generate` command can use workflow manager
- `review` command can leverage enhanced commit tools
- New `workflow` command for direct workflow execution

## 📋 Next Steps & Future Enhancements

### 🔄 Immediate Next Steps

1. **Complete Rollback Implementation**
   - Expose rollback methods from patchutils
   - Implement file restoration from backups
   - Add rollback command for manual recovery

2. **Integration Testing**
   - Unit tests for all new components
   - Integration tests for full workflow
   - Error scenario testing

3. **Command Integration**
   - Update `generate` command to use workflow manager
   - Update `review` command to use enhanced tools
   - Add workflow-specific CLI commands

### 🚀 Future Enhancements

#### PR Creation Support
```go
type PRCreationTool struct {
    // GitHub/GitLab API integration
    // Automated PR creation after commits
    // Template-based PR descriptions
}
```

#### Advanced Rollback Features
- **Selective Rollback**: Rollback specific files only
- **Time-based Rollback**: Rollback to specific timestamps
- **Commit-based Rollback**: Rollback to specific commits

#### Workflow Templates
```toml
[workflow_templates.feature_development]
auto_commit = true
commit_type = "feat"
run_tests_before_commit = true
create_pr_on_success = false

[workflow_templates.bug_fix]
auto_commit = true
commit_type = "fix"
run_tests_before_commit = true
create_pr_on_success = true
```

#### Enhanced Audit Features
- **Web Dashboard**: View audit logs in web interface
- **Metrics Collection**: Track workflow success rates
- **Alert System**: Notify on repeated failures

## 📁 File Structure

```
internal/
├── audit/
│   └── logger.go                    # Structured audit logging
├── patchutils/
│   └── applier.go                   # Robust patch application
├── workflow/
│   └── manager.go                   # Workflow orchestration
└── agent/
    ├── patch_apply_tool_enhanced.go # Enhanced patch tool
    └── git_commit_enhanced_tool.go  # Enhanced commit tool

codex.toml                           # Extended configuration
```

## 🧪 Testing Strategy

### Unit Tests
- [ ] `patchutils.PatchApplier` - All patch scenarios
- [ ] `audit.AuditLogger` - Event logging and querying
- [ ] `workflow.WorkflowManager` - Workflow orchestration
- [ ] Enhanced tools - Parameter validation and execution

### Integration Tests
- [ ] End-to-end workflow execution
- [ ] Error scenarios and rollback
- [ ] Configuration loading and validation
- [ ] Audit trail verification

### Performance Tests
- [ ] Large file patch application
- [ ] Multiple file workflow performance
- [ ] Audit log performance with high volume

## 📊 Success Metrics

### Reliability Metrics
- **Patch Success Rate**: >99% for valid patches
- **Rollback Success Rate**: 100% when backups exist
- **Audit Completeness**: 100% operation coverage

### Performance Metrics
- **Workflow Latency**: <5s for typical operations
- **Backup Overhead**: <10% additional time
- **Audit Overhead**: <5% additional time

### Usability Metrics
- **Error Recovery**: Automatic rollback on 100% of failures
- **Audit Clarity**: Clear error messages and context
- **Configuration Ease**: Simple TOML configuration

## 🔒 Security Considerations

### Path Security
- All file paths validated against workspace root
- No path traversal attacks possible
- Backup files stored securely within workspace

### Audit Security
- Audit logs stored with restricted permissions
- Session IDs prevent log tampering
- Structured format prevents injection attacks

### Git Security
- Commit signing support (future enhancement)
- Hook validation (configurable)
- Branch protection awareness

## 📚 Documentation

### User Documentation
- [ ] Configuration guide for automated workflows
- [ ] Troubleshooting guide for common issues
- [ ] Best practices for workflow design

### Developer Documentation
- [ ] API documentation for workflow manager
- [ ] Tool development guide for enhanced tools
- [ ] Audit system integration guide

---

This implementation provides a solid foundation for automated apply and commit workflows while maintaining flexibility for future enhancements. The modular design allows for incremental adoption and testing of individual components. 