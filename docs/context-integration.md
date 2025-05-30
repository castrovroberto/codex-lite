# Context Integration in CGE

CGE now features comprehensive context integration that makes the LLM aware of its operational environment, including the working directory, Git repository status, project structure, and more. This document explains how these features work and how to use them effectively.

## Overview

The context integration system automatically gathers and provides workspace context to the LLM, enabling it to make more informed decisions and provide more relevant assistance. The system includes:

- **Workspace awareness**: Understanding of the current working directory and project structure
- **Git integration**: Real-time Git repository status and branch information
- **Project type detection**: Automatic identification of project types (Go, Node.js, Python, etc.)
- **Dependency analysis**: Understanding of project dependencies and frameworks
- **Tool-based context gathering**: Proactive use of context-aware tools

## Key Components

### 1. Enhanced System Prompt

The system prompt (`system-prompt.md`) has been enhanced with:

- **Environmental Awareness**: Instructions about available context information
- **Context-Aware Tools**: Guidance on using `git_info`, `list_directory`, and `codebase_search`
- **Workspace Understanding**: Protocol for gathering context before making changes
- **Context Integration Guidelines**: How to factor in workspace context when making decisions

### 2. Context Integrator Service

The `ContextIntegrator` (`internal/context/integrator.go`) provides:

```go
type ContextIntegrator struct {
    workspaceRoot string
    gatherer      *Gatherer
    toolRegistry  *agent.Registry
}
```

**Key Methods:**
- `GatherWorkspaceContext(ctx)`: Collects comprehensive workspace information
- `FormatContextForPrompt(context)`: Formats context for LLM consumption
- Integration with existing tools (`git_info`, `list_directory`)

### 3. Dependency Injection Integration

The DI container (`internal/di/container.go`) now includes:

- **Context-aware chat presenters**: `GetChatPresenterWithContext()`
- **Absolute workspace root handling**: Ensures consistent path resolution
- **Context integrator management**: Lazy initialization of context services

### 4. Enhanced Configuration

The configuration file (`codex.toml`) now supports:

```toml
[project]
  enable_context_integration = true
  auto_gather_context = true
  context_cache_duration = "5m"

[tools.git_info]
  include_commits_by_default = true
  default_commit_count = 5
  include_status_details = true

[logging.components]
  context = "debug"  # Enable detailed context logging
```

## How It Works

### 1. Automatic Context Gathering

When the LLM starts a session, the context integrator automatically:

1. **Gathers Git information** using the `git_info` tool
2. **Analyzes project structure** using the `list_directory` tool
3. **Detects project type** based on configuration files
4. **Identifies dependencies** from package managers
5. **Formats context** for inclusion in the system prompt

### 2. Workspace Context Structure

```go
type WorkspaceContext struct {
    WorkspaceRoot    string                 `json:"workspace_root"`
    ProjectType      string                 `json:"project_type"`
    GitInfo          *GitContextInfo        `json:"git_info,omitempty"`
    ProjectStructure *ProjectStructureInfo  `json:"project_structure"`
    Dependencies     *DependencyInfo        `json:"dependencies,omitempty"`
    RecentActivity   *RecentActivityInfo    `json:"recent_activity,omitempty"`
    Environment      map[string]interface{} `json:"environment,omitempty"`
    Timestamp        time.Time              `json:"timestamp"`
}
```

### 3. Context-Aware Prompting

The system automatically prepends context information to prompts:

```
## 🔍 Workspace Context

**Working Directory:** `/Users/user/project`
**Project Type:** go_project
**Git Branch:** main
**Git Status:** clean
**Recent Commits:**
- `abc123`: Initial commit
- `def456`: Add new feature
**Package Manager:** go_modules
**Languages:** go

---
*This context was gathered automatically. Use `git_info` and `list_directory` tools for more detailed, real-time information.*
```

## Usage Examples

### 1. Chat with Context Integration

```bash
# Start chat with automatic context integration
cge chat

# The LLM will automatically be aware of:
# - Current Git branch and status
# - Project structure and type
# - Available dependencies
# - Working directory location
```

### 2. Planning with Context

```bash
# Generate plans with workspace awareness
cge plan "Add authentication to the API"

# The LLM will consider:
# - Existing project structure
# - Current dependencies (e.g., existing auth libraries)
# - Git branch status
# - Project conventions
```

### 3. Code Generation with Context

```bash
# Generate code that fits the project
cge generate plan.json

# The LLM will:
# - Follow existing code patterns
# - Use appropriate import styles
# - Respect project directory structure
# - Consider existing dependencies
```

## Context-Aware Tools

### git_info Tool

Provides real-time Git repository information:

```json
{
  "branch": "feature/new-auth",
  "status": {
    "clean": false,
    "changes": {
      "M ": 2,
      "??": 1
    }
  },
  "recent_commits": [
    {
      "hash": "abc123",
      "author": "developer",
      "message": "WIP: authentication system"
    }
  ]
}
```

### list_directory Tool

Explores project structure with security validation:

```json
{
  "files": [
    {
      "name": "main.go",
      "type": "file",
      "size": 1024,
      "absolute_path": "/project/main.go"
    }
  ],
  "directories": [
    {
      "name": "internal",
      "type": "directory",
      "file_count": 15
    }
  ]
}
```

### Enhanced Decision Making

The LLM now considers:

1. **Git Status**: Avoids conflicts with uncommitted changes
2. **Project Structure**: Follows established patterns and conventions
3. **Dependencies**: Leverages existing libraries and frameworks
4. **File Organization**: Respects directory structure and naming conventions
5. **Project Type**: Uses language-specific best practices

## Configuration Options

### Context Integration Settings

```toml
[project]
  # Enable automatic context gathering
  enable_context_integration = true
  
  # Gather context automatically on session start
  auto_gather_context = true
  
  # Cache context for performance
  context_cache_duration = "5m"

[tools.git_info]
  # Include commit history by default
  include_commits_by_default = true
  
  # Number of recent commits to include
  default_commit_count = 5
  
  # Include detailed status information
  include_status_details = true

[logging.components]
  # Enable detailed context logging for debugging
  context = "debug"
```

### Security Considerations

The context integration system respects security boundaries:

- **Workspace isolation**: Only accesses files within the configured workspace
- **Path validation**: Validates all file paths for security
- **Sensitive data masking**: Masks API keys and sensitive information
- **Audit logging**: Logs all context gathering activities

## Troubleshooting

### Common Issues

1. **Context not being gathered**
   - Check `enable_context_integration = true` in config
   - Verify workspace root is correctly set
   - Check logs for context gathering errors

2. **Git information missing**
   - Ensure you're in a Git repository
   - Check Git is installed and accessible
   - Verify Git repository is properly initialized

3. **Project type not detected**
   - Ensure configuration files exist (go.mod, package.json, etc.)
   - Check workspace root points to project root
   - Verify file permissions allow reading

### Debug Logging

Enable detailed context logging:

```toml
[logging.components]
  context = "debug"
```

This will show:
- Context gathering attempts
- Tool execution results
- Project type detection logic
- Context formatting process

## Best Practices

### 1. Workspace Organization

- Keep configuration files in the project root
- Use consistent directory structures
- Maintain clean Git history for better context

### 2. Configuration

- Set appropriate workspace root in `codex.toml`
- Enable context integration for better assistance
- Configure tool-specific settings for optimal performance

### 3. Usage Patterns

- Let the LLM gather context before complex operations
- Use context-aware tools (`git_info`, `list_directory`) for real-time information
- Provide additional context when the automatic gathering is insufficient

## Future Enhancements

Planned improvements include:

- **Deep context analysis**: More sophisticated project understanding
- **Cross-file analysis**: Understanding relationships between files
- **Dependency tracking**: Tracking changes in dependencies over time
- **Context prediction**: Predicting what context will be needed
- **Smart suggestions**: Context-aware suggestions and completions

## Integration with Existing Features

The context integration works seamlessly with:

- **Session management**: Context is preserved across sessions
- **Tool execution**: All tools benefit from workspace awareness
- **Code generation**: Generated code respects project conventions
- **Planning**: Plans consider existing project structure
- **Review**: Reviews understand project context and standards

This context integration makes CGE significantly more intelligent and helpful by providing the LLM with comprehensive understanding of your workspace and project. 