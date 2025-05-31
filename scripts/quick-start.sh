#!/bin/bash

# CGE Quick Start Script
# This script helps you get started with CGE quickly

set -e

echo "🚀 CGE Quick Start Setup"
echo "========================"

# Check if we're in the CGE directory
if [ ! -f "main.go" ] || [ ! -f "codex.toml" ]; then
    echo "❌ Please run this script from the CGE project root directory"
    exit 1
fi

# Build CGE
echo "📦 Building CGE..."
go build -o cge main.go
echo "✅ CGE built successfully"

# Check if Ollama is running (optional)
echo "🔍 Checking Ollama connection..."
if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "✅ Ollama is running and accessible"
else
    echo "⚠️  Ollama not detected at localhost:11434"
    echo "   Make sure Ollama is installed and running, or update codex.toml with your Ollama host"
fi

# Show available commands
echo ""
echo "🎯 Available Commands:"
echo "====================="
./cge --help

echo ""
echo "📚 Quick Examples:"
echo "=================="
echo "1. Generate a development plan:"
echo "   ./cge plan \"Add user authentication\" --output auth-plan.json"
echo ""
echo "2. Preview code generation:"
echo "   ./cge generate --plan auth-plan.json --dry-run"
echo ""
echo "3. Review your codebase:"
echo "   ./cge review --auto-fix"
echo ""
echo "4. Start interactive chat:"
echo "   ./cge chat"
echo ""
echo "📖 For more examples, check out the examples/ directory"
echo ""
echo "🎉 CGE is ready to use! Happy coding!" 