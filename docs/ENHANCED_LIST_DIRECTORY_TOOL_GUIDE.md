# Enhanced List Directory Tool Guide

## Overview

The enhanced `list_directory` tool is a powerful and versatile replacement for the original tool that addresses the common issue: **"Tool execution failed: directory path is outside workspace root"**. This new implementation provides configurable security controls, intelligent path resolution, and advanced features while maintaining security best practices.

## Problem Solved

### Original Issue
The original `list_directory` tool had a hard-coded restriction that prevented listing directories outside the workspace root, leading to frequent errors when users tried to explore parent directories, system directories, or other project locations.

### Solution Features
1. **Configurable Workspace Restrictions**: Control whether outside workspace access is allowed
2. **Allowed Roots**: Specify additional trusted directories for listing
3. **Smart Path Resolution**: Intelligent handling of common directory patterns and path variations
4. **Enhanced Security**: Granular control over access permissions with clear audit trails
5. **Advanced Filtering**: Pattern-based file filtering and multiple sorting options
6. **Performance Controls**: Configurable limits to prevent resource exhaustion

## Configuration Options

### Basic Configuration Structure

```toml
[tools.list_directory]
# Allow listing directories outside the workspace root
allow_outside_workspace = false

# Additional allowed root directories for listing
allowed_roots = [
    "~/Documents",
    "~/Downloads", 
    "/tmp"
]

# Maximum recursion depth limit
max_depth_limit = 10

# Maximum number of files to return in one operation
max_files_limit = 1000

# Whether to automatically resolve symbolic links
auto_resolve_symlinks = false

# Enable smart path resolution for common directory names
smart_path_resolution = true
```

### Configuration Options Explained

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `allow_outside_workspace` | boolean | `false` | Enables listing directories outside workspace root |
| `allowed_roots` | array | `[]` | List of additional root directories that are allowed |
| `max_depth_limit` | integer | `10` | Maximum recursion depth for directory traversal |
| `max_files_limit` | integer | `1000` | Maximum number of files to return in one operation |
| `auto_resolve_symlinks` | boolean | `false` | Whether to follow and resolve symbolic links |
| `smart_path_resolution` | boolean | `true` | Enable intelligent path resolution for common patterns |

## New Features

### 1. Smart Path Resolution

The tool can intelligently resolve common directory name patterns:

```bash
# These all work even if the exact name doesn't exist:
"docs" → tries: docs, documentation, doc
"src" → tries: src, source, lib  
"test" → tries: test, tests, __tests__, spec
"config" → tries: config, configs, configuration, settings
"scripts" → tries: scripts, script, bin
"build" → tries: build, dist, target, out
```

### 2. Advanced Path Handling

- **Home Directory Expansion**: Use `~` for home directory access
- **Relative and Absolute Paths**: Supports both with proper validation
- **Path Normalization**: Automatically cleans and resolves path issues

### 3. Pattern Filtering

Filter files using glob patterns:

```json
{
  "directory_path": ".",
  "recursive": true,
  "pattern": "*.go",
  "max_depth": 3
}
```

### 4. Flexible Sorting

Multiple sorting options:
- `name`: Alphabetical by name
- `size`: By file size
- `modified`: By modification time
- `type`: Directories first, then files
- `type_name`: Directories first, then alphabetical (default)

### 5. Enhanced File Information

Each file entry now includes:

```json
{
  "name": "example.go",
  "path": "internal/agent/example.go",
  "absolute_path": "/full/path/to/file",
  "is_directory": false,
  "size": 1024,
  "mod_time": "2024-01-01T12:00:00Z",
  "permissions": "-rw-r--r--",
  "is_source_file": true,
  "is_symlink": false,
  "link_target": ""
}
```

### 6. Security and Access Control

The tool provides detailed information about path resolution and access control:

```json
{
  "path_resolution": {
    "original_path": "~/Documents",
    "resolved_path": "/Users/user/Documents",
    "is_absolute": true,
    "is_in_workspace": false,
    "allowed_by_rule": "allowed_root: ~/Documents"
  }
}
```

## Usage Examples

### Basic Usage (Backward Compatible)

```json
{
  "directory_path": ".",
  "recursive": false,
  "include_hidden": false
}
```

### Advanced Usage with All Features

```json
{
  "directory_path": "src",
  "recursive": true,
  "include_hidden": false,
  "max_depth": 3,
  "pattern": "*.go",
  "sort_by": "modified",
  "smart_resolve": true
}
```

### Outside Workspace Access (When Enabled)

```json
{
  "directory_path": "~/Documents/projects",
  "recursive": false,
  "include_hidden": false,
  "smart_resolve": true
}
```

## Security Considerations

### Default Security Posture
- **Workspace-only access by default**: The tool maintains the original security behavior by default
- **Explicit configuration required**: Outside workspace access must be explicitly enabled
- **Allowed roots validation**: Only pre-configured directories are accessible outside workspace

### Access Control Rules

1. **Within Workspace**: Always allowed (rule: "within_workspace")
2. **Workspace Root**: Special case, always allowed (rule: "workspace_root")
3. **Allowed Roots**: Explicitly configured directories (rule: "allowed_root: path")
4. **Outside Access Enabled**: General outside access when configured (rule: "outside_access_enabled")

### Audit Trail

Every operation includes detailed path resolution information for security auditing:
- Original path requested
- Resolved absolute path
- Access control rule applied
- Whether path is within workspace

## Integration Guide

### Using Default Configuration

```go
// Use existing factory (maintains backward compatibility)
factory := agent.NewToolFactory(workspaceRoot)
registry := factory.CreateRegistry()
```

### Using Custom Configuration (Recommended Pattern)

```go
// Load configuration from config file
cfg := config.GetConfig()
toolFactoryConfig := cfg.GetToolFactoryConfig()

// Create factory with complete tool configuration
factory := agent.NewToolFactoryWithConfig(workspaceRoot, toolFactoryConfig)
registry := factory.CreateRegistry()
```

### Using Individual Tool Configuration

```go
// Load configuration from config file
cfg := config.GetConfig()
listDirConfig := cfg.GetListDirectoryConfig()

// Create factory and set specific tool config
factory := agent.NewToolFactory(workspaceRoot)
factory.SetListDirectoryConfig(listDirConfig)
registry := factory.CreateRegistry()
```

### Direct Tool Creation

```go
// Create with default configuration
tool := agent.NewListDirTool(workspaceRoot)

// Create with custom configuration
customConfig := agent.ListDirToolConfig{
    AllowOutsideWorkspace: true,
    AllowedRoots: []string{"/tmp", "~/Documents"},
    MaxDepthLimit: 5,
    MaxFilesLimit: 500,
    SmartPathResolution: true,
}
tool := agent.NewListDirToolWithConfig(workspaceRoot, customConfig)
```

## Migration from Original Tool

### Backward Compatibility
The enhanced tool is **100% backward compatible** with the original `list_directory` tool. Existing code will continue to work without changes.

### Configuration Migration
To enable enhanced features, add the `[tools.list_directory]` section to your `codex.toml` configuration file.

### Common Migration Patterns

1. **Enable Outside Workspace Access**:
```toml
[tools.list_directory]
allow_outside_workspace = true
allowed_roots = ["~/Projects", "/opt/workspace"]
```

2. **Increase Performance Limits**:
```toml
[tools.list_directory]
max_files_limit = 5000
max_depth_limit = 15
```

3. **Enable All Features**:
```toml
[tools.list_directory]
allow_outside_workspace = true
allowed_roots = ["~", "/usr/local", "/opt"]
max_depth_limit = 20
max_files_limit = 2000
auto_resolve_symlinks = true
smart_path_resolution = true
```

## Benefits

### For Users
- **No more "outside workspace root" errors** when configured appropriately
- **Intelligent path resolution** reduces guesswork
- **Advanced filtering** improves productivity
- **Flexible access control** balances security and usability

### For Developers
- **Configurable security** meets different deployment requirements
- **Rich metadata** enables better tooling
- **Performance controls** prevent resource exhaustion
- **Clear audit trails** support security compliance

### For LLM Agents
- **Reduced dependency on user guidance** through smart resolution
- **Better error messages** with actionable information
- **Flexible exploration** capabilities with safety guardrails
- **Comprehensive context** for better decision making

## Testing

Run the demonstration script to see all features in action:

```bash
go run examples/enhanced_list_dir_demo.go
```

This demo showcases:
- Basic directory listing
- Outside workspace access (when configured)
- Smart path resolution
- Pattern filtering and sorting
- Custom configuration features

## Performance Characteristics

- **Memory Usage**: Bounded by `max_files_limit` configuration
- **Execution Time**: Limited by filesystem performance and `max_depth_limit`
- **CPU Usage**: Minimal overhead for path resolution and filtering
- **Security Overhead**: Negligible impact from access control validation

## Best Practices

1. **Start with Default Configuration**: Use secure defaults, enable features as needed
2. **Configure Allowed Roots Carefully**: Only add directories that are truly needed
3. **Set Appropriate Limits**: Balance functionality with resource constraints  
4. **Monitor Access Patterns**: Use audit trails to understand usage patterns
5. **Regular Security Review**: Periodically review and update allowed roots

## Troubleshooting

### Common Issues

1. **"Access denied: directory path is outside workspace root"**
   - Solution: Enable `allow_outside_workspace` or add path to `allowed_roots`

2. **"No smart resolution found for X"**  
   - Solution: The directory doesn't match common patterns, use exact path

3. **"Results truncated due to file limit"**
   - Solution: Increase `max_files_limit` or use more specific patterns

4. **Performance Issues**
   - Solution: Reduce `max_depth_limit`, use patterns to filter, or increase limits carefully

### Debug Information

The tool provides comprehensive debug information in responses:
- Path resolution details
- Access control decisions
- Performance metrics (file/directory counts)
- Truncation warnings 