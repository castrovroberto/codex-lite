# CGE Agent Tool Suite

This package contains the comprehensive tool suite for the CGE (Code Generation Engine) agent system. The tools provide the agent with capabilities to interact with the filesystem, version control, testing, and development environment.

## Architecture

### Core Components

- **Tool Interface**: All tools implement the `Tool` interface with `Name()`, `Description()`, `Parameters()`, and `Execute()` methods
- **Registry**: Manages tool registration and lookup
- **Factory**: Creates tool registries for different use cases (planning, generation, review)
- **Validator**: Provides common validation and security functions

### Tool Categories

#### File Operations
- **`read_file`**: Read file contents with optional line range
- **`write_file`**: Write or overwrite file contents with directory creation
- **`list_directory`**: List directory contents with optional recursion

#### Code Analysis
- **`codebase_search`**: Semantic search across the codebase
- **`analyze_codebase`**: Basic codebase structure analysis
- **`analyze_advanced`**: Advanced analysis including dependencies and complexity

#### Version Control
- **`git_info`**: Get repository status, branch, and commit history
- **`git_commit`**: Stage files and create commits

#### Development Tools
- **`run_tests`**: Execute tests with structured output parsing
- **`run_linter`**: Run linting tools (go fmt, go vet, golangci-lint)
- **`run_shell_command`**: Execute allowed shell commands safely

#### Code Modification
- **`apply_patch_to_file`**: Apply unified diff patches with backup

## Tool Usage

### Basic Tool Execution

```go
// Create a tool registry
factory := NewToolFactory("/path/to/workspace")
registry := factory.CreateFullRegistry()

// Get a tool
tool, exists := registry.Get("read_file")
if !exists {
    log.Fatal("Tool not found")
}

// Prepare parameters
params := json.RawMessage(`{
    "target_file": "main.go",
    "start_line": 1,
    "end_line": 50
}`)

// Execute the tool
result, err := tool.Execute(context.Background(), params)
if err != nil {
    log.Fatal(err)
}

if result.Success {
    fmt.Printf("Tool output: %+v\n", result.Data)
} else {
    fmt.Printf("Tool error: %s\n", result.Error)
}
```

### Registry Types

The factory provides different registry configurations:

- **`CreatePlanningRegistry()`**: Read-only tools for planning phase
- **`CreateGenerationRegistry()`**: Read/write tools for code generation
- **`CreateReviewRegistry()`**: Full tool suite including testing and linting
- **`CreateFullRegistry()`**: All available tools

## Tool Reference

### File Operations

#### read_file
Reads file contents with optional line range specification.

**Parameters:**
```json
{
    "target_file": "path/to/file.go",
    "start_line": 10,
    "end_line": 50
}
```

**Response:**
```json
{
    "success": true,
    "data": {
        "file_path": "path/to/file.go",
        "content": "file contents...",
        "total_lines": 100,
        "lines_read": 41
    }
}
```

#### write_file
Writes content to a file, creating directories if needed.

**Parameters:**
```json
{
    "file_path": "new/file.go",
    "content": "package main\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}",
    "create_dirs_if_needed": true
}
```

#### list_directory
Lists directory contents with optional recursion.

**Parameters:**
```json
{
    "directory_path": "src",
    "recursive": false,
    "include_hidden": false
}
```

### Development Tools

#### run_tests
Executes tests with structured output parsing.

**Parameters:**
```json
{
    "target_path": "./...",
    "test_pattern": "TestExample.*",
    "verbose": true,
    "coverage": true,
    "timeout_seconds": 300
}
```

**Response:**
```json
{
    "success": true,
    "data": {
        "total_tests": 15,
        "passed_tests": 14,
        "failed_tests": 1,
        "skipped_tests": 0,
        "duration": "2.5s",
        "coverage": "85.2%",
        "results": [
            {
                "name": "TestExample",
                "status": "PASS",
                "duration": "0.1s"
            }
        ]
    }
}
```

#### run_linter
Runs linting tools with structured issue reporting.

**Parameters:**
```json
{
    "target_path": ".",
    "linter": "all",
    "fix": false,
    "timeout_seconds": 120
}
```

**Response:**
```json
{
    "success": false,
    "data": {
        "total_issues": 3,
        "error_count": 1,
        "warning_count": 2,
        "issues": [
            {
                "file": "main.go",
                "line": 15,
                "column": 10,
                "severity": "error",
                "message": "undefined variable",
                "rule": "undeclared-name",
                "linter": "go-vet"
            }
        ]
    }
}
```

### Version Control

#### git_commit
Stages files and creates a commit.

**Parameters:**
```json
{
    "commit_message": "Add new feature",
    "files_to_stage": ["main.go", "README.md"],
    "allow_empty": false
}
```

#### git_info
Gets repository information.

**Parameters:**
```json
{
    "include_commits": true,
    "commit_count": 5
}
```

### Code Modification

#### apply_patch_to_file
Applies a unified diff patch to a file.

**Parameters:**
```json
{
    "file_path": "main.go",
    "patch_content": "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n package main\n \n+import \"fmt\"\n func main() {",
    "backup_original": true
}
```

## Security Features

### Path Validation
- All file paths are validated to prevent directory traversal attacks
- Paths must be within the workspace root
- Dangerous patterns like `..` are rejected

### Command Restrictions
- Shell commands are restricted to an allowed list
- Commands are executed with timeouts
- Working directory is constrained to workspace

### Content Validation
- File content size limits can be enforced
- File extensions can be restricted
- JSON schema validation for parameters

## Error Handling

All tools follow consistent error handling patterns:

1. **Parameter Validation**: Invalid parameters return structured errors
2. **Security Checks**: Path traversal and other security issues are caught early
3. **Execution Errors**: Runtime errors are captured and reported
4. **Rollback Support**: Operations that modify files support rollback on failure

## Extending the Tool Suite

### Creating a New Tool

1. Implement the `Tool` interface:

```go
type MyTool struct {
    workspaceRoot string
    validator     *ToolValidator
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Description of what my tool does"
}

func (t *MyTool) Parameters() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "param1": {"type": "string", "description": "First parameter"}
        },
        "required": ["param1"]
    }`)
}

func (t *MyTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
    // Implementation here
}
```

2. Add to the tool factory:

```go
func (tf *ToolFactory) registerCoreTool(registry *Registry) {
    tools := []Tool{
        // ... existing tools
        NewMyTool(tf.workspaceRoot),
    }
    // ...
}
```

### Best Practices

1. **Use the validator**: Leverage `ToolValidator` for common validation tasks
2. **Provide detailed errors**: Include context in error messages
3. **Support timeouts**: Use context for cancellation
4. **Create backups**: For destructive operations, create backups
5. **Structured output**: Return structured data that can be easily processed
6. **Idempotency**: Design tools to be idempotent where possible

## Testing

### Unit Tests
Each tool should have comprehensive unit tests covering:
- Parameter validation
- Success cases
- Error cases
- Security edge cases

### Integration Tests
Test tools in combination to ensure they work together properly.

### Example Test Structure

```go
func TestMyTool(t *testing.T) {
    workspaceRoot := t.TempDir()
    tool := NewMyTool(workspaceRoot)
    
    t.Run("valid parameters", func(t *testing.T) {
        params := json.RawMessage(`{"param1": "value"}`)
        result, err := tool.Execute(context.Background(), params)
        
        assert.NoError(t, err)
        assert.True(t, result.Success)
    })
    
    t.Run("invalid parameters", func(t *testing.T) {
        params := json.RawMessage(`{}`)
        result, err := tool.Execute(context.Background(), params)
        
        assert.NoError(t, err)
        assert.False(t, result.Success)
        assert.Contains(t, result.Error, "required parameter")
    })
}
```

## Performance Considerations

- **Caching**: Consider caching for expensive operations
- **Streaming**: Use streaming for large file operations
- **Timeouts**: Set appropriate timeouts for all operations
- **Resource Limits**: Implement resource limits to prevent abuse

## Future Enhancements

Planned improvements to the tool suite:

1. **Enhanced Context Retrieval**: Vector-based semantic search
2. **Language-Specific Tools**: Tools for specific programming languages
3. **Remote Operations**: Tools for interacting with remote services
4. **Advanced Analysis**: More sophisticated code analysis capabilities
5. **Workflow Tools**: Tools for managing complex development workflows 