#!/bin/bash
# security-fixes.sh - Apply security-related lint fixes

set -e

echo "🔒 Applying security fixes to CGE codebase..."

# Fix file permissions
echo "📁 Fixing file permissions..."
echo "  - Changing directory permissions from 0755 to 0750..."
find . -name "*.go" -not -path "./vendor/*" -exec sed -i '' 's/0755/0750/g' {} \;

echo "  - Changing file permissions from 0644 to 0600..."
find . -name "*.go" -not -path "./vendor/*" -exec sed -i '' 's/0644/0600/g' {} \;

# Note: More complex security fixes require manual intervention
echo ""
echo "⚠️  IMPORTANT: Additional security fixes require manual review:"
echo ""
echo "🔍 Path Traversal (G304) - 12 issues:"
echo "  Files: internal/analyzer/, internal/audit/logger.go, etc."
echo "  Action: Add filepath.Clean() and path validation"
echo ""
echo "🔍 Command Injection (G204) - 8 issues:"
echo "  Files: internal/agent/git_*.go, internal/agent/shell_run_tool.go, etc."
echo "  Action: Add argument validation for exec.Command calls"
echo ""
echo "🔍 Unhandled Errors (G104) - 10 issues:"
echo "  Files: cmd/session.go, internal/patchutils/applier.go, etc."
echo "  Action: Add proper error handling for cleanup operations"
echo ""
echo "✅ Automated security fixes applied!"
echo "📋 See LINT_ERROR_ANALYSIS.md for detailed manual fix instructions" 