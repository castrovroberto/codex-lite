# Workspace Root Path Resolution Fix

## Problem Summary

The CGE application was experiencing a critical issue where the `list_directory` tool would fail with the error:

```
Tool execution failed: access denied: directory path is outside workspace root and outside access is disabled
```

This occurred when trying to list directories like `internal/tui` that should clearly be within the workspace.

## Root Cause Analysis

The issue was caused by **inconsistent workspace root path resolution** across different parts of the application:

1. **Relative vs Absolute Paths**: The workspace root was being passed as a relative path (`.`) to tool factories, but the `ListDirTool`'s path resolution logic expected absolute paths for proper security validation.

2. **Path Comparison Logic**: The `resolvePath` method in `ListDirTool` performs security checks by comparing:
   - `cleanResolved` (the resolved target path)
   - `cleanWorkspace` (the workspace root)

3. **Inconsistent Path Formats**: When workspace root was relative (`.`) and the target path was `internal/tui`, the resolved path became an absolute path, but the workspace root remained relative, causing the security check to fail.

## Technical Details

### Before Fix
```go
// In various command files
workspaceRoot := cfg.Project.WorkspaceRoot
if workspaceRoot == "" {
    workspaceRoot = "." // Relative path
}
toolFactory := agent.NewToolFactory(workspaceRoot) // Passing relative path
```

### After Fix
```go
// In various command files
workspaceRoot := cfg.Project.WorkspaceRoot
if workspaceRoot == "" {
    workspaceRoot = "."
}

// Convert workspace root to absolute path to fix tool access issues
absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
if err != nil {
    return fmt.Errorf("failed to convert workspace root to absolute path: %w", err)
}

toolFactory := agent.NewToolFactory(absWorkspaceRoot) // Passing absolute path
```

## Files Modified

### 1. `internal/tui/chat/model.go`
- **Function**: `InitialModel`
- **Change**: Convert workspace root to absolute path before creating tool factory
- **Impact**: Fixes list_directory tool access in chat TUI

### 2. `cmd/plan_orchestrated.go`
- **Function**: Main command execution
- **Change**: Convert workspace root to absolute path before creating tool factory
- **Impact**: Fixes list_directory tool access in orchestrated planning

### 3. `cmd/review_orchestrated.go`
- **Function**: Main command execution
- **Change**: Convert workspace root to absolute path before creating tool factory
- **Impact**: Fixes list_directory tool access in orchestrated review

### 4. `cmd/generate.go`
- **Function**: Main command execution
- **Change**: Convert workspace root to absolute path and update all references
- **Impact**: Fixes list_directory tool access in code generation

### 5. `cmd/session.go`
- **Functions**: All session command functions (list, resume, info, export, analytics, cleanup)
- **Change**: Convert workspace root to absolute path before creating session managers and tool factories
- **Impact**: Fixes list_directory tool access in session management

## Security Validation Logic

The `ListDirTool.resolvePath` method performs the following security checks:

```go
// Check if within workspace - fixed to prevent false positives
cleanWorkspace := filepath.Clean(t.workspaceRoot)
cleanResolved := result.ResolvedPath

// A path is within the workspace if:
// 1. It's exactly the workspace root, OR
// 2. It starts with the workspace root followed by a path separator
if cleanResolved == cleanWorkspace {
    result.IsInWorkspace = true
} else {
    result.IsInWorkspace = strings.HasPrefix(cleanResolved, cleanWorkspace+string(filepath.Separator))
}
```

With absolute paths for both `cleanWorkspace` and `cleanResolved`, this logic now works correctly.

## Testing

### Verification Steps
1. **Build Test**: `go build -o cge .` - ✅ Successful
2. **Unit Tests**: `go test ./internal/agent -v` - ✅ All tests pass
3. **Integration**: All command functions now use consistent absolute path resolution

### Expected Behavior After Fix
- ✅ `list_directory` tool can access `internal/tui` and other workspace subdirectories
- ✅ Security validation still prevents access outside workspace (when configured)
- ✅ All existing functionality preserved
- ✅ Consistent path handling across all commands

## Configuration Compatibility

The fix is **100% backward compatible**:
- No configuration changes required
- Existing `codex.toml` files work unchanged
- Default security settings remain the same
- Optional enhanced features still available via configuration

## Benefits

### For Users
- **No more false "outside workspace" errors** for legitimate workspace directories
- **Consistent behavior** across all CGE commands
- **Improved reliability** of tool execution

### For Developers
- **Centralized path resolution** logic
- **Consistent absolute path handling** throughout the application
- **Maintained security** while fixing usability issues

### For LLM Agents
- **Reliable directory exploration** within workspace
- **Reduced tool execution failures**
- **Better user experience** in interactive sessions

## Future Considerations

1. **Path Resolution Utilities**: Consider creating a centralized workspace path resolution utility to prevent similar issues
2. **Enhanced Logging**: Add debug logging for path resolution to aid troubleshooting
3. **Configuration Validation**: Add startup validation to ensure workspace root is accessible

## Related Issues

This fix addresses the core issue described in the diagnostic:
- **Error**: "Tool execution failed: access denied: directory path is outside workspace root and outside access is disabled"
- **Symptom**: LLM agents unable to explore workspace directories
- **Root Cause**: Inconsistent relative vs absolute path handling
- **Solution**: Standardize on absolute paths for all workspace root references

The fix ensures that the `list_directory` tool and other workspace-aware tools function correctly while maintaining the security boundaries intended by the original design. 