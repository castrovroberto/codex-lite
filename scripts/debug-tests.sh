#!/bin/bash

# Test debugging script for CGE
# This script helps debug test failures by running tests with verbose output
# and collecting debugging information.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run a command and capture output
run_with_output() {
    local cmd="$1"
    local output_file="$2"
    
    print_status "Running: $cmd"
    if eval "$cmd" 2>&1 | tee "$output_file"; then
        print_success "Command completed successfully"
        return 0
    else
        print_error "Command failed"
        return 1
    fi
}

# Create debug output directory
DEBUG_DIR="debug-output-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$DEBUG_DIR"

print_status "Starting test debugging session"
print_status "Debug output will be saved to: $DEBUG_DIR"

# Collect environment information
print_status "Collecting environment information..."
{
    echo "=== Environment Information ==="
    echo "Date: $(date)"
    echo "Go version: $(go version)"
    echo "OS: $(uname -a)"
    echo "PWD: $(pwd)"
    echo "GOPATH: ${GOPATH:-not set}"
    echo "GOROOT: ${GOROOT:-not set}"
    echo "GO111MODULE: ${GO111MODULE:-not set}"
    echo ""
    echo "=== Go Environment ==="
    go env
    echo ""
    echo "=== Git Information ==="
    git status --porcelain
    git log --oneline -5
} > "$DEBUG_DIR/environment.txt"

# Check Go module status
print_status "Checking Go module status..."
run_with_output "go mod verify" "$DEBUG_DIR/mod-verify.txt"
run_with_output "go mod download" "$DEBUG_DIR/mod-download.txt"

# Run tests with verbose output
print_status "Running unit tests with verbose output..."
if ! run_with_output "go test -v -race -coverprofile=$DEBUG_DIR/coverage.out ./..." "$DEBUG_DIR/unit-tests.txt"; then
    print_error "Unit tests failed"
    UNIT_TESTS_FAILED=1
else
    print_success "Unit tests passed"
    UNIT_TESTS_FAILED=0
fi

# Run integration tests
print_status "Running integration tests..."
if ! run_with_output "go test -v -tags=integration ./tests/integration/..." "$DEBUG_DIR/integration-tests.txt"; then
    print_error "Integration tests failed"
    INTEGRATION_TESTS_FAILED=1
else
    print_success "Integration tests passed"
    INTEGRATION_TESTS_FAILED=0
fi

# Generate coverage report if tests passed
if [ $UNIT_TESTS_FAILED -eq 0 ]; then
    print_status "Generating coverage report..."
    go tool cover -html="$DEBUG_DIR/coverage.out" -o "$DEBUG_DIR/coverage.html"
    go tool cover -func="$DEBUG_DIR/coverage.out" > "$DEBUG_DIR/coverage-summary.txt"
fi

# Run linting
print_status "Running linting checks..."
run_with_output "go vet ./..." "$DEBUG_DIR/vet.txt"

# Check formatting
print_status "Checking code formatting..."
{
    echo "=== Formatting Check ==="
    if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
        echo "The following files are not formatted:"
        gofmt -s -l .
        echo "FORMATTING_ISSUES=true"
    else
        echo "All files are properly formatted"
        echo "FORMATTING_ISSUES=false"
    fi
} > "$DEBUG_DIR/formatting.txt"

# Run golangci-lint if available
if command -v golangci-lint >/dev/null 2>&1; then
    print_status "Running golangci-lint..."
    run_with_output "golangci-lint run --timeout=5m" "$DEBUG_DIR/golangci-lint.txt" || true
else
    print_warning "golangci-lint not found, skipping"
fi

# Run security checks if available
if command -v gosec >/dev/null 2>&1; then
    print_status "Running security scan..."
    run_with_output "gosec ./..." "$DEBUG_DIR/gosec.txt" || true
else
    print_warning "gosec not found, skipping security scan"
fi

# Create summary
print_status "Creating debug summary..."
{
    echo "=== Test Debug Summary ==="
    echo "Generated: $(date)"
    echo "Debug Directory: $DEBUG_DIR"
    echo ""
    echo "=== Test Results ==="
    echo "Unit Tests: $([ $UNIT_TESTS_FAILED -eq 0 ] && echo "PASSED" || echo "FAILED")"
    echo "Integration Tests: $([ $INTEGRATION_TESTS_FAILED -eq 0 ] && echo "PASSED" || echo "FAILED")"
    echo ""
    echo "=== Files Generated ==="
    ls -la "$DEBUG_DIR/"
    echo ""
    echo "=== Next Steps ==="
    if [ $UNIT_TESTS_FAILED -eq 1 ] || [ $INTEGRATION_TESTS_FAILED -eq 1 ]; then
        echo "1. Check the test output files for specific error messages"
        echo "2. Review the environment.txt for any configuration issues"
        echo "3. Ensure all dependencies are properly installed"
        echo "4. Check for Go version compatibility issues"
    else
        echo "All tests passed! Check the coverage report for test coverage information."
    fi
} > "$DEBUG_DIR/summary.txt"

# Print summary
print_status "Debug session completed"
echo ""
cat "$DEBUG_DIR/summary.txt"
echo ""
print_status "All debug information saved to: $DEBUG_DIR"

# Exit with appropriate code
if [ $UNIT_TESTS_FAILED -eq 1 ] || [ $INTEGRATION_TESTS_FAILED -eq 1 ]; then
    exit 1
else
    exit 0
fi 