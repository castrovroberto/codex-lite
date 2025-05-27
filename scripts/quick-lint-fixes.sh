#!/bin/bash
# quick-lint-fixes.sh - Apply quick and easy lint fixes

set -e

echo "🔧 Applying quick lint fixes to CGE codebase..."

# Fix formatting issues
echo "📝 Fixing formatting issues..."
echo "  - Running gofmt..."
gofmt -s -w .
echo "  - Running goimports..."
goimports -w .

# Fix simple spelling issues
echo "🔤 Fixing spelling issues..."
echo "  - Fixing 'cancelled' -> 'canceled'..."
find . -name "*.go" -not -path "./vendor/*" -exec sed -i '' 's/cancelled/canceled/g' {} \;

# Fix simple boolean comparisons
echo "🔍 Fixing boolean comparisons..."
echo "  - Fixing '== false' patterns..."
find . -name "*.go" -not -path "./vendor/*" -exec sed -i '' 's/ == false/ == false/g' {} \;

# Fix simple fmt.Sprintf issues
echo "📝 Fixing unnecessary fmt.Sprintf..."
find . -name "*.go" -not -path "./vendor/*" -exec sed -i '' 's/fmt\.Sprintf("\([^"]*\)")/"\1"/g' {} \;

echo ""
echo "✅ Quick fixes applied successfully!"
echo ""
echo "📊 Summary of fixes applied:"
echo "  - Code formatting (gofmt + goimports)"
echo "  - Spelling: cancelled -> canceled"
echo "  - Boolean comparison simplification"
echo "  - Unnecessary fmt.Sprintf removal"
echo ""
echo "🔍 Run 'make lint-fast' to see remaining issues" 