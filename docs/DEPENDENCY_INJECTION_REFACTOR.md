# Dependency Injection Refactor

This document outlines the comprehensive dependency injection (DI) refactor implemented to improve separation of concerns, testability, and maintainability in the CGE codebase.

## Overview

The refactor introduces a centralized dependency injection container that manages the creation and injection of dependencies throughout the application. This addresses the key architectural concerns identified in the codebase analysis.

## Key Components

### 1. Dependency Injection Container (`internal/di/container.go`)

The `Container` struct serves as the composition root for the application, managing all dependencies:

```go
type Container struct {
    config       *config.AppConfig
    fileSystem   FileSystemService
    cmdExecutor  CommandExecutor
    httpClient   HTTPClient
    sessionStore SessionStore
    
    // Services (built lazily)
    llmClient    llm.Client
    toolRegistry *agent.Registry
    agentRunner  *orchestrator.AgentRunner
}
```

**Benefits:**
- **Centralized dependency management**: All dependencies are created and configured in one place
- **Lazy initialization**: Services are created only when needed
- **Easy testing**: Dependencies can be swapped with mocks for testing
- **Configuration isolation**: Each service receives only the configuration it needs

### 2. Service Interfaces (`internal/di/interfaces.go`)

Abstract interfaces for external dependencies:

- `FileSystemService`: Abstracts file system operations
- `CommandExecutor`: Abstracts command execution
- `HTTPClient`: Abstracts HTTP operations
- `SessionStore`: Abstracts session persistence

**Benefits:**
- **Testability**: Easy to mock for unit tests
- **Flexibility**: Can swap implementations without changing business logic
- **Decoupling**: Business logic doesn't depend on concrete implementations

### 3. Enhanced Tool Factory (`internal/agent/tool_factory_enhanced.go`)

Supports dependency injection for agent tools:

```go
type EnhancedToolFactory struct {
    workspaceRoot string
    config        *ToolFactoryConfig
    fileSystem    FileSystemService
    cmdExecutor   CommandExecutor
}
```

**Benefits:**
- **Tool testability**: Tools can be tested with mock file systems and command executors
- **Consistent behavior**: All tools use the same injected dependencies
- **Easy configuration**: Tools receive their dependencies through the factory

### 4. Enhanced Tools

Example: `FileWriteToolEnhanced` uses injected `FileSystemService` instead of direct `os` calls:

```go
// Before (tightly coupled)
if err := os.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
    // handle error
}

// After (dependency injected)
if err := t.fileSystem.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
    // handle error
}
```

**Benefits:**
- **Unit testable**: Can test file operations without touching the real file system
- **Consistent error handling**: All file operations go through the same interface
- **Easy to extend**: Can add features like file operation logging or caching

## Usage Examples

### 1. Production Usage

```go
// In cmd/chat.go
func chatCommand(cmd *cobra.Command, args []string) error {
    // Get configuration
    appCfg := getConfigFromContext(cmd.Context())
    
    // Create DI container with real dependencies
    container := di.NewContainer(appCfg)
    
    // Get services through the container
    chatPresenter := container.GetChatPresenter(ctx, modelName, systemPrompt)
    
    // Use in TUI
    chatModel := chat.NewChatModel(
        chat.WithMessageProvider(chatPresenter),
        // ... other options
    )
}
```

### 2. Testing Usage

```go
func TestChatWithMockDependencies(t *testing.T) {
    cfg := &config.AppConfig{}
    
    // Create container with mock dependencies
    container := di.NewContainer(cfg).
        WithFileSystem(di.NewMockFileSystemService()).
        WithCommandExecutor(di.NewMockCommandExecutor()).
        WithHTTPClient(di.NewMockHTTPClient())
    
    // Test with mocked dependencies
    toolRegistry := container.GetToolRegistry()
    // ... test tool behavior without side effects
}
```

### 3. Tool Testing

```go
func TestFileWriteToolWithMockFS(t *testing.T) {
    mockFS := di.NewMockFileSystemService()
    tool := agent.NewFileWriteToolEnhanced("/workspace", mockFS)
    
    // Test file writing without touching real file system
    result, err := tool.Execute(ctx, params)
    
    // Verify mock was called correctly
    assert.True(t, mockFS.Exists("/workspace/test.txt"))
}
```

## Migration Strategy

The refactor was implemented progressively to minimize disruption:

### Phase 1: Infrastructure (✅ Complete)
- Created DI container and interfaces
- Implemented mock services for testing
- Added enhanced tool factory

### Phase 2: Command Integration (✅ Complete)
- Updated `cmd/chat.go` to use DI container
- Maintained backward compatibility with existing code

### Phase 3: Tool Enhancement (🔄 In Progress)
- Enhanced `FileWriteTool` with dependency injection
- Placeholder implementations for other tools
- Gradual migration of remaining tools

### Phase 4: Advanced Features (📋 Planned)
- Session management with injected store
- HTTP client injection for LLM clients
- Configuration-based service selection

## Benefits Achieved

### 1. **Improved Testability**
- Tools can be tested without file system side effects
- Commands can be tested without external dependencies
- Mock implementations provide predictable behavior

### 2. **Better Separation of Concerns**
- Business logic separated from infrastructure concerns
- Clear boundaries between layers
- Single responsibility principle enforced

### 3. **Enhanced Maintainability**
- Dependencies are explicit and visible
- Easy to understand component relationships
- Centralized configuration management

### 4. **Increased Flexibility**
- Easy to swap implementations (e.g., different storage backends)
- Configuration-driven behavior
- Support for different deployment environments

### 5. **Reduced Coupling**
- Components depend on interfaces, not concrete types
- Changes to one component don't cascade to others
- Easier to refactor and extend

## Testing Strategy

### Unit Tests
- Each component can be tested in isolation
- Mock dependencies provide controlled test environments
- Fast execution without external dependencies

### Integration Tests
- Test component interactions with real dependencies
- Verify end-to-end functionality
- Catch integration issues early

### Example Test Structure
```go
func TestToolWithRealDependencies(t *testing.T) {
    // Test with real file system for integration testing
    container := di.NewContainer(cfg)
    tool := container.GetToolRegistry().Get("write_file")
    // ... test with real dependencies
}

func TestToolWithMockDependencies(t *testing.T) {
    // Test with mocks for unit testing
    mockFS := di.NewMockFileSystemService()
    tool := agent.NewFileWriteToolEnhanced("/workspace", mockFS)
    // ... test with predictable mock behavior
}
```

## Future Enhancements

### 1. Configuration-Based Service Selection
```go
// Select file system implementation based on config
func (c *Container) buildFileSystem() FileSystemService {
    switch c.config.Storage.Type {
    case "s3":
        return NewS3FileSystemService(c.config.Storage.S3)
    case "local":
        return &RealFileSystemService{}
    default:
        return &RealFileSystemService{}
    }
}
```

### 2. Service Lifecycle Management
- Singleton vs. transient services
- Service initialization and cleanup
- Health checks and monitoring

### 3. Advanced Mocking
- Record and replay functionality
- Behavior verification
- Performance testing with controlled delays

## Conclusion

The dependency injection refactor significantly improves the codebase's architecture by:

1. **Making dependencies explicit** - No more hidden dependencies or global state
2. **Enabling comprehensive testing** - Every component can be tested in isolation
3. **Improving maintainability** - Clear separation of concerns and single responsibility
4. **Increasing flexibility** - Easy to extend and modify behavior
5. **Reducing coupling** - Components depend on interfaces, not implementations

This foundation enables more robust development practices and makes the codebase more resilient to change. 