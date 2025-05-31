# Lint Error Analysis & Action Plan

## Overview
Analysis of 100+ lint errors found in the CGE codebase, grouped by type with actionable solutions.

## Error Categories Summary

| Category | Count | Severity | Effort | Priority |
|----------|-------|----------|--------|----------|
| **Security (gosec)** | 45 | High | Medium | 🔴 High |
| **Formatting** | 8 | Low | Easy | 🟡 Medium |
| **Unchecked Errors** | 20 | Medium | Easy | 🟡 Medium |
| **Unused Code** | 15 | Low | Easy | 🟢 Low |
| **Code Quality** | 12 | Medium | Medium | 🟡 Medium |
| **Deprecated APIs** | 5 | Medium | Easy | 🟡 Medium |

---

## 🔴 HIGH PRIORITY: Security Issues (gosec)

### File Permissions (G301, G302, G306)
**Count**: 15 issues
**Problem**: Files/directories created with overly permissive permissions

**Examples**:
```go
// Current (insecure)
os.MkdirAll(logDir, 0755)        // G301: Should be 0750 or less
os.WriteFile(filepath, data, 0644) // G306: Should be 0600 or less
os.OpenFile(path, flags, 0644)    // G302: Should be 0600 or less
```

**Solution**:
```go
// Fixed (secure)
os.MkdirAll(logDir, 0750)        // Directory: owner+group read/write/execute
os.WriteFile(filepath, data, 0600) // File: owner read/write only
os.OpenFile(path, flags, 0600)    // File: owner read/write only
```

**Files affected**:
- `internal/audit/logger.go` (3 issues)
- `internal/orchestrator/session_manager.go` (2 issues)
- `internal/tui/chat/history.go` (3 issues)
- `cmd/generate.go` (6 issues)
- `cmd/plan.go` (1 issue)

### File Inclusion via Variable (G304)
**Count**: 12 issues
**Problem**: Potential path traversal vulnerabilities

**Examples**:
```go
// Current (potentially unsafe)
content, err := os.ReadFile(filepath)  // filepath from user input
```

**Solution**:
```go
// Fixed (safe)
import "path/filepath"

// Validate and clean the path
cleanPath := filepath.Clean(filepath)
if !strings.HasPrefix(cleanPath, allowedDir) {
    return fmt.Errorf("invalid path: %s", filepath)
}
content, err := os.ReadFile(cleanPath)
```

**Files affected**:
- `internal/analyzer/` (4 files)
- `internal/audit/logger.go`
- `internal/config/config.go`
- `internal/orchestrator/session_manager.go` (2 issues)
- `internal/templates/engine.go`
- `internal/textutils/chunker.go`
- `internal/patchutils/applier.go`

### Command Injection (G204)
**Count**: 8 issues
**Problem**: Subprocess execution with potentially tainted input

**Examples**:
```go
// Current (potentially unsafe)
cmd := exec.Command("git", args...)  // args from user input
```

**Solution**:
```go
// Fixed (safe)
// Validate arguments
for _, arg := range args {
    if !isValidArg(arg) {
        return fmt.Errorf("invalid argument: %s", arg)
    }
}
cmd := exec.Command("git", args...)
```

**Files affected**:
- `internal/agent/git_*.go` (4 files)
- `internal/agent/lint_runner_tool.go`
- `internal/agent/shell_run_tool.go`
- `internal/context/gatherer.go`
- `cmd/review.go`

### Unhandled Errors (G104)
**Count**: 10 issues
**Problem**: Error return values ignored

**Examples**:
```go
// Current (unsafe)
os.Remove(backupPath)  // Error ignored
auditLogger.Close()    // Error ignored
```

**Solution**:
```go
// Fixed (safe)
if err := os.Remove(backupPath); err != nil {
    log.Warn("Failed to remove backup", "path", backupPath, "error", err)
}
if err := auditLogger.Close(); err != nil {
    log.Error("Failed to close audit logger", "error", err)
}
```

---

## 🟡 MEDIUM PRIORITY: Code Quality Issues

### Unchecked Error Returns (errcheck)
**Count**: 20 issues
**Problem**: Function calls that return errors but aren't checked

**Examples**:
```go
// Current
json.Unmarshal(params, &toolParams)  // Error not checked
fmt.Sscanf(s, "%d", &result)        // Error not checked
registry.Register(tool)              // Error not checked
```

**Solution**:
```go
// Fixed
if err := json.Unmarshal(params, &toolParams); err != nil {
    return fmt.Errorf("failed to unmarshal params: %w", err)
}
if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
    log.Warn("Failed to parse number", "input", s, "error", err)
}
if err := registry.Register(tool); err != nil {
    return fmt.Errorf("failed to register tool: %w", err)
}
```

### Repeated Strings (goconst)
**Count**: 9 issues
**Problem**: Magic strings repeated multiple times

**Examples**:
```go
// Current
case "tool":      // Used 3 times
case "assistant": // Used 3 times
case "error":     // Used 6 times
```

**Solution**:
```go
// Fixed
const (
    MessageTypeTool      = "tool"
    MessageTypeAssistant = "assistant"
    LogLevelError        = "error"
    LogLevelWarning      = "warning"
    LogLevelInfo         = "info"
)

case MessageTypeTool:
case MessageTypeAssistant:
case LogLevelError:
```

### Deprecated API Usage (staticcheck)
**Count**: 5 issues
**Problem**: Using deprecated Go APIs

**Examples**:
```go
// Current (deprecated)
netErr.Temporary()  // SA1019: Deprecated since Go 1.18
m.suggestionStyle.Copy()  // SA1019: Use assignment instead
```

**Solution**:
```go
// Fixed
// For network errors, check specific error types instead
var timeoutErr *net.OpError
if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
    // Handle timeout
}

// For style copying, use assignment
newStyle := m.suggestionStyle  // Direct assignment
```

---

## 🟢 LOW PRIORITY: Cleanup Issues

### Formatting Issues (gofmt/goimports)
**Count**: 8 issues
**Problem**: Code not properly formatted

**Solution**: Run formatting tools
```bash
# Fix all formatting issues
gofmt -s -w .
goimports -w .
```

**Files affected**:
- `cmd/` (6 files)
- `internal/logger/logger.go`
- `internal/orchestrator/session_manager.go`
- `internal/tui/chat/model.go`

### Unused Code (unused)
**Count**: 15 issues
**Problem**: Variables, fields, and types that are never used

**Examples**:
```go
// Current (unused)
var agentStatusStyle = lipgloss.NewStyle()  // Never used
type tickMsg time.Time                      // Never used
field toolName string                       // Never used
```

**Solution**: Remove unused code or add `//nolint:unused` if intentionally kept

### Code Simplification (gosimple)
**Count**: 3 issues
**Problem**: Code that can be simplified

**Examples**:
```go
// Current
if p.CreateDirsIfNeeded == false {  // S1002
fmt.Sprintf("📊 Codebase Statistics:\n")  // S1039

// Fixed
if !p.CreateDirsIfNeeded {
"📊 Codebase Statistics:\n"
```

---

## 📋 Action Plan

### Phase 1: Critical Security Fixes (1-2 days)
1. **Fix file permissions** (15 issues)
   - Change all `0755` to `0750` for directories
   - Change all `0644` to `0600` for files
   
2. **Add path validation** (12 issues)
   - Implement `filepath.Clean()` and validation
   - Add allowlist for permitted directories

3. **Fix command injection** (8 issues)
   - Add argument validation for exec.Command calls
   - Implement input sanitization

### Phase 2: Error Handling (1 day)
1. **Add error checking** (20 issues)
   - Add proper error handling for all function calls
   - Use appropriate logging levels

2. **Fix unhandled errors** (10 issues)
   - Add error checking for cleanup operations
   - Implement graceful error handling

### Phase 3: Code Quality (0.5 days)
1. **Extract constants** (9 issues)
   - Create constants for repeated strings
   - Organize in appropriate packages

2. **Fix deprecated APIs** (5 issues)
   - Replace deprecated network error handling
   - Update style copying patterns

### Phase 4: Cleanup (0.5 days)
1. **Format code** (8 issues)
   ```bash
   make fmt
   ```

2. **Remove unused code** (15 issues)
   - Delete unused variables and types
   - Add nolint comments where appropriate

---

## 🛠️ Implementation Scripts

### Quick Fix Script
```bash
#!/bin/bash
# quick-lint-fixes.sh

echo "🔧 Applying quick lint fixes..."

# Fix formatting
echo "📝 Fixing formatting..."
gofmt -s -w .
goimports -w .

# Fix simple issues
echo "🔍 Fixing simple code issues..."
find . -name "*.go" -exec sed -i '' 's/== false/== false/g' {} \;
find . -name "*.go" -exec sed -i '' 's/cancelled/canceled/g' {} \;

echo "✅ Quick fixes applied!"
```

### Security Fix Script
```bash
#!/bin/bash
# security-fixes.sh

echo "🔒 Applying security fixes..."

# Fix file permissions
echo "📁 Fixing file permissions..."
find . -name "*.go" -exec sed -i '' 's/0755/0750/g' {} \;
find . -name "*.go" -exec sed -i '' 's/0644/0600/g' {} \;

echo "✅ Security fixes applied!"
```

---

## 📊 Progress Tracking

Create a checklist to track progress:

- [ ] **Security Fixes** (45 issues)
  - [ ] File permissions (15)
  - [ ] Path validation (12) 
  - [ ] Command injection (8)
  - [ ] Unhandled errors (10)

- [ ] **Error Handling** (20 issues)
  - [ ] Add error checking for function calls

- [ ] **Code Quality** (17 issues)
  - [ ] Extract constants (9)
  - [ ] Fix deprecated APIs (5)
  - [ ] Code simplification (3)

- [ ] **Cleanup** (23 issues)
  - [ ] Format code (8)
  - [ ] Remove unused code (15)

---

## 🎯 Expected Results

After implementing all fixes:
- **Security**: 45 security vulnerabilities resolved
- **Reliability**: 30 error handling issues fixed
- **Maintainability**: 17 code quality improvements
- **Cleanliness**: 23 cleanup items resolved

**Total**: ~115 lint issues resolved, significantly improving code quality and security posture. 