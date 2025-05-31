# Codex-GPT-Engineer (CGE) 🚀

<!-- Optional: Add a logo or a relevant image here -->
<!-- <img src="path/to/your/logo.png" alt="CGE Logo" width="150" style="float: right;"> -->

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/castrovroberto/CGE)
[![CI](https://github.com/castrovroberto/CGE/workflows/CI/badge.svg)](https://github.com/castrovroberto/CGE/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/castrovroberto/CGE)](https://goreportcard.com/report/github.com/castrovroberto/CGE)

**Codex-GPT-Engineer (CGE): Your AI-powered partner for engineering complex software projects.** CGE is a sophisticated command-line tool that leverages LLMs (via Ollama or OpenAI) to assist with project planning, code generation, automated reviews, and iterative development workflows.

CGE features a comprehensive tool suite, robust testing infrastructure, and automated CI/CD pipelines to ensure reliable AI-assisted development workflows.

---

##  Table of Contents

1.  [Overview](#1-overview)
2.  [Key Features](#2-key-features)
3.  [Project Structure](#3-project-structure-)
4.  [Getting Started](#4-getting-started)
    *   [Prerequisites](#prerequisites)
    *   [Installation](#installation)
    *   [Configuration](#configuration)
5.  [Usage](#5-usage)
    *   [Plan Command](#plan-command)
    *   [Generate Command](#generate-command)
    *   [Review Command](#review-command)
    *   [Chat Command](#chat-command)
6.  [Workflow Examples](#6-workflow-examples)
7.  [Testing & Quality Assurance](#7-testing--quality-assurance)
8.  [Docker](#8-docker)
    *   [Building the Docker Image](#building-the-docker-image)
    *   [Running with Docker](#running-with-docker)
9.  [Contributing](#9-contributing)
10. [License](#10-license)
11. [Acknowledgements](#11-acknowledgements)

---

## **1️⃣ Overview**

**Codex-GPT-Engineer (CGE)** is a versatile CLI tool designed to enhance your software engineering workflow through AI-powered assistance. It integrates with Large Language Models (LLMs) like those accessible via Ollama and OpenAI to provide:

*   **🎯 Intelligent Project Planning:** Generate comprehensive development plans based on your requirements and existing codebase context
*   **⚡ Automated Code Generation:** Create new files or modify existing ones based on generated plans
*   **🔍 Automated Review & Iteration:** Validate generated code using tests and linters, and iteratively refine it with LLM assistance
*   **💬 Interactive Chat:** Get real-time coding assistance and explanations
*   **🧠 Context-Aware Analysis:** Deep understanding of your codebase structure, dependencies, and Git history
*   **🛠️ Comprehensive Tool Suite:** 20+ specialized tools for file operations, code analysis, testing, and Git operations
*   **🧪 Robust Testing Framework:** Comprehensive unit and integration tests with mock tool infrastructure
*   **🚀 CI/CD Integration:** Automated testing, security scanning, and regression testing pipelines

The tool is built using Go and provides a modern CLI experience with comprehensive configuration options and enterprise-grade reliability.

---

## **2️⃣ Key Features**

### **🎯 Enhanced Planning**
- **Real Codebase Context:** Analyzes your actual project structure, dependencies, and Git history
- **Structured JSON Plans:** Generates detailed, actionable development plans with task dependencies
- **Effort Estimation:** Provides realistic time estimates for each task
- **Risk Assessment:** Identifies potential challenges and considerations

### **⚡ Code Generation**
- **Plan-Driven Development:** Executes tasks from generated plans in proper dependency order
- **Multiple Modes:** Dry-run preview, direct application, or output to files
- **Task Filtering:** Process specific tasks or subsets of the plan
- **Context-Aware Generation:** Uses project context for consistent code style

### **🔍 Automated Review**
- **Test Integration:** Runs your test suite and analyzes failures
- **Linting Support:** Integrates with code linters for quality checks
- **Iterative Improvement:** Uses LLM to suggest and apply fixes automatically
- **Configurable Cycles:** Set maximum review iterations to prevent infinite loops

### **💬 Interactive Features**
- **Real-time Chat:** Interactive coding assistance with project context
- **Tool Integration:** Access to codebase analysis, Git operations, and file reading
- **Multi-Provider Support:** Works with Ollama (local) and OpenAI (cloud) models

### **🛠️ Comprehensive Tool Suite**
- **File Operations:** Read, write, list directories with security safeguards
- **Code Analysis:** Search, analyze complexity, retrieve context intelligently
- **Testing & Quality:** Run tests, parse results, execute linters
- **Git Integration:** Status, commits, history, and enhanced commit workflows
- **Shell Operations:** Secure command execution with timeout controls
- **Patch Management:** Apply diffs and patches with rollback capabilities

### **🧪 Testing & Quality Assurance**
- **Mock Tool Framework:** Configurable mock tools for testing with behavior simulation
- **Integration Testing:** Comprehensive tool integration tests
- **Security Testing:** Parameter validation and path traversal protection
- **CI/CD Pipelines:** Automated testing across multiple platforms and Go versions
- **Regression Testing:** Nightly regression tests to catch breaking changes

### **🛠️ Developer Experience**
- **Flexible Configuration:** TOML files, environment variables, and CLI flags
- **Rich Logging:** Detailed logging with configurable levels
- **Template System:** Customizable prompts for different use cases
- **Cross-Platform:** Works on macOS, Linux, and Windows
- **Example Cookbooks:** Practical examples and tutorials for common use cases

---

## **3️⃣ Project Structure 📁**

```bash
📂 cge/
┣ 📂 cmd/                   # Cobra command definitions
┃ ┣ 📄 plan.go             # Plan generation command
┃ ┣ 📄 generate.go         # Code generation command  
┃ ┣ 📄 review.go           # Code review command
┃ ┣ 📄 chat.go             # Interactive chat command
┃ ┗ 📄 root.go             # Root command and CLI setup
┣ 📂 internal/              # Core application logic
┃ ┣ 📂 config/             # Configuration management
┃ ┣ 📂 llm/                # LLM client interfaces (Ollama, OpenAI)
┃ ┣ 📂 templates/          # Prompt template engine
┃ ┣ 📂 context/            # Codebase context gathering
┃ ┣ 📂 scanner/            # File system scanning
┃ ┣ 📂 analyzer/           # Code analysis (complexity, dependencies, security)
┃ ┣ 📂 agent/              # Tool system for LLM interactions
┃ ┃ ┣ 📂 testing/          # Mock tools and test infrastructure
┃ ┃ ┣ 📄 *_tool.go         # Individual tool implementations
┃ ┃ ┗ 📄 *_test.go         # Tool unit tests
┃ ┣ 📂 contextkeys/        # Context value keys
┃ ┣ 📂 logger/             # Structured logging
┃ ┗ 📂 tui/                # Terminal UI components
┣ 📂 tests/                 # Test suites
┃ ┗ 📂 integration/        # Integration tests
┣ 📂 examples/              # Usage examples and cookbooks
┃ ┣ 📂 cookbooks/          # Step-by-step tutorials
┃ ┗ 📂 demos/              # Demo applications
┣ 📂 .github/               # CI/CD workflows
┃ ┗ 📂 workflows/          # GitHub Actions
┣ 📂 prompts/               # LLM prompt templates
┃ ┣ 📄 plan.tmpl           # Planning prompt template
┃ ┗ 📄 generate.tmpl       # Code generation template
┣ 📄 codex.toml             # Main configuration file
┣ 📄 go.mod                 # Go module definition
┣ 📄 main.go                # Application entry point
┗ 📄 README.md              # This file
```

---

## **4️⃣ Getting Started**

### **🔹 Prerequisites**

-   **Go:** Version 1.23 or higher
-   **LLM Provider:**
    -   **Ollama:** Local LLM server with models (recommended: `deepseek-coder-v2:16b`)
    -   **OpenAI:** API key for cloud models (optional)
-   **Development Tools:** Your preferred test runner and linter

### **🔹 Installation**

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/castrovroberto/CGE.git
    cd CGE
    ```

2.  **Quick setup (recommended):**
    ```bash
    ./scripts/quick-start.sh
    ```

3.  **Manual build:**
    ```bash
    go build -o cge main.go
    ```

4.  **Install globally (optional):**
    ```bash
    go install
    ```

### **🔹 Configuration**

Create a `codex.toml` file in your project root or home directory:

```toml
version = "0.1.0"

[llm]
  provider = "ollama"  # "ollama" or "openai"
  model = "deepseek-coder-v2:16b"
  ollama_host_url = "http://localhost:11434"
  # For OpenAI: set OPENAI_API_KEY environment variable

[project]
  workspace_root = "."
  # default_ignore_dirs = [".git", "node_modules", "vendor"]
  # default_source_extensions = [".go", ".py", ".js", ".ts"]

[logging]
  level = "info"  # "debug", "info", "warn", "error"

[commands.review]
  test_command = "go test ./..."
  lint_command = "golangci-lint run"
  max_cycles = 3
```

---

## **5️⃣ Usage**

### **🎯 Plan Command**

Generate intelligent development plans based on your goals and codebase context:

```bash
# Basic planning
./cge plan "Add user authentication with JWT tokens"

# Custom output file
./cge plan "Refactor database layer" --output refactor-plan.json

# The plan command analyzes:
# - Current codebase structure and dependencies
# - Git repository status and history  
# - File organization and patterns
# - Existing code complexity and style
```

**Example Plan Output:**
```json
{
  "overall_goal": "Add user authentication with JWT tokens",
  "tasks": [
    {
      "id": "task_1",
      "description": "Create JWT utility functions",
      "files_to_create": ["internal/auth/jwt.go"],
      "estimated_effort": "small",
      "dependencies": [],
      "rationale": "Foundation for JWT token handling"
    },
    {
      "id": "task_2", 
      "description": "Implement authentication middleware",
      "files_to_create": ["internal/middleware/auth.go"],
      "files_to_modify": ["cmd/root.go"],
      "estimated_effort": "medium",
      "dependencies": ["task_1"],
      "rationale": "Middleware to protect routes"
    }
  ],
  "summary": "Implementation plan for JWT authentication system",
  "estimated_total_effort": "medium",
  "risks_and_considerations": [
    "Ensure secure JWT secret management",
    "Consider token refresh strategy"
  ]
}
```

### **⚡ Generate Command**

Execute development plans with AI-powered code generation:

```bash
# Dry run (preview changes)
./cge generate --plan plan.json --dry-run

# Apply changes directly
./cge generate --plan plan.json --apply

# Save changes to directory
./cge generate --plan plan.json --output-dir ./generated

# Process specific tasks
./cge generate --plan plan.json --task auth --dry-run
```

**Features:**
- **Dependency Resolution:** Executes tasks in proper order
- **Context-Aware:** Uses existing code patterns and style
- **Safe Execution:** Dry-run mode for preview
- **Selective Processing:** Filter tasks by name or ID

### **🔍 Review Command**

Validate and improve generated code through automated testing and linting:

```bash
# Basic review with configured commands
./cge review

# Custom test and lint commands
./cge review --test-cmd "go test ./..." --lint-cmd "golangci-lint run"

# Auto-fix issues with LLM assistance
./cge review --auto-fix --max-cycles 3

# Review specific directory
./cge review ./src --auto-fix
```

**Review Process:**
1. **Test Execution:** Runs your test suite
2. **Linting:** Checks code quality and style
3. **Issue Analysis:** Identifies failures and problems
4. **LLM Fixes:** Suggests and applies improvements
5. **Iteration:** Repeats until all issues resolved or max cycles reached

### **💬 Chat Command**

Interactive coding assistance with full project context:

```bash
# Start interactive chat
./cge chat

# Example interactions:
# > "Explain the authentication flow in this codebase"
# > "How can I optimize the database queries in user.go?"
# > "Show me the Git history for the auth module"
```

---

## **6️⃣ Examples and Tutorials**

CGE includes comprehensive examples and cookbooks to help you get started:

### **📚 Available Cookbooks**

- **[JWT Authentication](examples/cookbooks/web-development/jwt-authentication/)** - Complete guide to implementing JWT authentication in a Go web application
- **API Development** - Building RESTful APIs with proper error handling and middleware
- **Database Integration** - Setting up database connections, migrations, and ORM integration
- **Testing Strategies** - Comprehensive testing approaches for Go applications

### **🎯 Quick Start Examples**

```bash
# Explore available examples
ls examples/cookbooks/

# Follow a specific cookbook
cd examples/cookbooks/web-development/jwt-authentication/
cat README.md

# Run example demos
go run examples/demos/function_calling_demo.go
```

### **💡 Usage Patterns**

Each cookbook includes:
- **Step-by-step instructions** with CGE commands
- **Sample code** and configuration files
- **Best practices** and common pitfalls
- **Testing strategies** for the implemented features

📁 **Check out the [examples/](examples/) directory for detailed usage scenarios, best practices, and configuration examples for different project types.**

## **6️⃣ Workflow Examples**

### **Complete Feature Development**

```bash
# 1. Plan the feature
./cge plan "Add rate limiting to API endpoints" --output rate-limit-plan.json

# 2. Review the plan
cat rate-limit-plan.json

# 3. Generate code (dry run first)
./cge generate --plan rate-limit-plan.json --dry-run

# 4. Apply the changes
./cge generate --plan rate-limit-plan.json --apply

# 5. Review and fix issues
./cge review --auto-fix --max-cycles 3

# 6. Interactive refinement
./cge chat
# > "The rate limiting tests are failing, can you help debug?"
```

### **Code Quality Improvement**

```bash
# 1. Plan refactoring
./cge plan "Improve error handling across the codebase" --output error-handling-plan.json

# 2. Generate improvements
./cge generate --plan error-handling-plan.json --apply

# 3. Automated review and fixes
./cge review --auto-fix --test-cmd "go test ./..." --lint-cmd "golangci-lint run"
```

### **Legacy Code Analysis**

```bash
# 1. Analyze existing codebase
./cge plan "Document and refactor the legacy user module" --output legacy-analysis.json

# 2. Interactive exploration
./cge chat
# > "Analyze the complexity of the user module"
# > "What are the main dependencies in this codebase?"
# > "Suggest improvements for the authentication code"
```

---

## **7️⃣ Testing & Quality Assurance**

CGE includes a comprehensive testing infrastructure to ensure reliability and quality:

### **🧪 Test Suite**

```bash
# Run all tests
go test ./...

# Run unit tests only
go test ./internal/...

# Run integration tests
go test ./tests/integration/...

# Run tests with coverage
go test -cover ./...

# Run specific tool tests
go test ./internal/agent/ -v
```

### **🛠️ Development Tools**

CGE includes several tools to help with development and debugging:

```bash
# Use Makefile for common tasks
make help                    # Show all available commands
make test                    # Run unit tests
make test-integration        # Run integration tests
make test-all               # Run all tests
make ci-test                # Run tests like CI
make lint                   # Run linting
make ci-all                 # Run all CI checks locally

# Debug test failures
./scripts/debug-tests.sh     # Comprehensive test debugging
```

### **🔍 Debugging Test Failures**

When tests fail in CI or locally, use these debugging tools:

1. **Debug Script:** Run `./scripts/debug-tests.sh` to collect comprehensive debugging information
2. **CI Simulation:** Use `make ci-test` to run tests exactly like CI
3. **Test Artifacts:** Check uploaded artifacts in GitHub Actions for detailed logs
4. **Environment Check:** Verify Go version compatibility (requires Go 1.23+)

The debug script creates a timestamped directory with:
- Test output logs
- Environment information
- Coverage reports
- Linting results
- Security scan results

### **🔧 Mock Testing Framework**

CGE provides a sophisticated mock tool framework for testing:

- **Configurable Behaviors:** Mock tools can simulate success, failure, and custom responses
- **Parameter Validation:** Automatic validation of tool parameters against JSON schemas
- **Call Counting:** Track how many times tools are called during tests
- **Realistic Fixtures:** Pre-built test data and sample projects for consistent testing

```go
// Example: Testing with mock tools
mockTool := &testing.MockTool{
    ToolName: "test_tool",
    Behavior: testing.MockBehaviorSuccess,
    ResponseData: map[string]interface{}{
        "result": "test output",
    },
}
```

### **🔒 Security Testing**

- **Path Traversal Protection:** Tests ensure tools cannot access files outside workspace
- **Parameter Validation:** Comprehensive validation of all tool inputs
- **Error Handling:** Robust error handling and recovery mechanisms

### **🚀 CI/CD Pipeline**

Our GitHub Actions workflows provide:

- **Multi-Platform Testing:** Tests run on Ubuntu, macOS, and Windows
- **Multiple Go Versions:** Compatibility testing across Go 1.23 and 1.24
- **Enhanced Debugging:** Test artifacts uploaded on failure for easy debugging
- **Security Scanning:** Automated vulnerability scanning with Gosec and govulncheck
- **Code Coverage:** Coverage reporting and tracking with Codecov integration
- **Quality Gates:** Comprehensive quality checks before merge
- **Regression Testing:** Nightly tests to catch breaking changes
- **Docker Testing:** Automated Docker image building and testing

### **📊 Quality Metrics**

- **Test Coverage:** Comprehensive unit and integration test coverage
- **Code Quality:** Automated linting and static analysis
- **Performance:** Benchmarking for critical operations
- **Documentation:** All tools and functions are thoroughly documented

📁 **Check out the [tests/](tests/) directory for detailed test cases and the [.github/workflows/](.github/workflows/) directory for CI/CD configurations.**

## **8️⃣ Docker**

### **Building the Docker Image**

```bash
docker build -t cge:latest .
```

### **Running with Docker**

```bash
# Mount your project directory
docker run -v $(pwd):/workspace -w /workspace cge:latest plan "Add logging to API"

# Interactive mode
docker run -it -v $(pwd):/workspace -w /workspace cge:latest chat
```

---

## **9️⃣ Contributing**

We welcome contributions! Please see our contributing guidelines:

1. **Fork the repository**
2. **Create a feature branch:** `git checkout -b feature/amazing-feature`
3. **Make your changes** and add comprehensive tests
4. **Run the full test suite:** `go test ./...`
5. **Run integration tests:** `go test ./tests/integration/...`
6. **Run the review process:** `./cge review --auto-fix`
7. **Commit your changes:** `git commit -m 'Add amazing feature'`
8. **Push to the branch:** `git push origin feature/amazing-feature`
9. **Open a Pull Request**

### **Development Setup**

```bash
# Install dependencies
go mod download

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test ./tests/integration/... -v

# Build and test
go build -o cge main.go
./cge plan "Test the development setup" --output test-plan.json

# Run security checks (if you have gosec installed)
gosec ./...
```

### **Testing Requirements**

When contributing:

- **Unit Tests:** Add unit tests for all new functions and methods
- **Integration Tests:** Add integration tests for new tools or major features
- **Mock Tests:** Use the mock tool framework for testing tool interactions
- **Security Tests:** Ensure proper parameter validation and security checks
- **Documentation:** Update documentation and examples as needed

### **Code Quality Standards**

- Follow Go best practices and idioms
- Ensure all tests pass before submitting PR
- Add appropriate error handling and logging
- Include security considerations for new tools
- Maintain backward compatibility when possible

---

## **10️⃣ License**

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## **11️⃣ Acknowledgements**

- **Ollama** for providing excellent local LLM infrastructure
- **OpenAI** for powerful cloud-based LLM capabilities
- **Cobra** for the powerful CLI framework
- **Viper** for flexible configuration management
- **Bubble Tea** for the interactive terminal UI components
- **GitHub Actions** for robust CI/CD infrastructure
- **Gosec** for security scanning and vulnerability detection
- **Testify** for enhanced testing capabilities
- The **Go community** for excellent tooling and libraries
- **Contributors** who have helped build and test CGE

---

**Ready to supercharge your development workflow? Get started with CGE today!** 🚀

