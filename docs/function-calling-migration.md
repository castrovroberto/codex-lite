# Function-Calling Migration Guide

This document explains the changes made to CGE's prompt templates to support function calling and provides guidance for understanding the new behavior.

## Overview

CGE has been updated to use function calling for all side effects instead of generating plain text responses. This improves reliability, safety, and enables more sophisticated agent behavior.

## Key Changes

### 1. System Prompt Updates

**Before:**
```markdown
You are CGE, a specialized AI assistant expert in software development and coding.
Your primary goal is to help users understand, write, debug, and improve code.
If you are provided with tools, use them when appropriate to gather context or perform actions.
```

**After:**
```markdown
You are CGE, a specialized AI assistant expert in software development and coding.
Your primary goal is to help users understand, write, debug, and improve code through function calls and structured responses.

## Function Calling Guidelines
- ALWAYS use function calls for side effects like reading files, writing files, running commands
- When you need to read a file, use the `read_file` function
- When you need to write or modify a file, use the `write_file` or `apply_patch_to_file` functions
- Provide final textual responses only when you have completed all necessary function calls
```

### 2. Template Refactoring

#### Plan Template (`prompts/plan.tmpl`)

**Key Changes:**
- Added instructions to use tools for context gathering
- Emphasized function calls before making planning decisions
- Added safety guidelines for file path validation
- Enhanced workflow instructions

**New Workflow:**
1. Use `read_file` to examine specific files mentioned in the goal
2. Use `codebase_search` to find relevant code patterns
3. Use `list_directory` to explore project structure
4. Create comprehensive plan based on gathered context

#### Generate Template (`prompts/generate.tmpl`)

**Key Changes:**
- Removed direct content generation in JSON responses
- Added function-calling workflow instructions
- Emphasized reading files before modifying them
- Added safety measures and validation guidelines

**New Workflow:**
1. Read relevant existing files to understand current state
2. Search codebase for similar patterns
3. Implement changes using `write_file` or `apply_patch_to_file`
4. Provide summary of work completed

#### Review Template (`prompts/review.tmpl`)

**Key Changes:**
- Added structured analysis and fix workflow
- Emphasized function calls for running tests and applying fixes
- Added iteration strategy for complex issues
- Enhanced error handling guidance

**New Workflow:**
1. Analyze issues using `read_file` and `codebase_search`
2. Implement fixes using `apply_patch_to_file`
3. Validate fixes using `run_tests` and `run_linter`
4. Iterate until all issues are resolved

## Function Calling Examples

### Reading Files
```json
{
  "function": "read_file",
  "parameters": {
    "target_file": "src/main.go",
    "start_line": 1,
    "end_line": 50
  }
}
```

### Writing Files
```json
{
  "function": "write_file",
  "parameters": {
    "file_path": "src/new_feature.go",
    "content": "package main\n\nfunc NewFeature() {\n    // Implementation\n}",
    "create_dirs_if_needed": true
  }
}
```

### Applying Patches
```json
{
  "function": "apply_patch_to_file",
  "parameters": {
    "file_path": "src/existing.go",
    "patch_content": "--- a/src/existing.go\n+++ b/src/existing.go\n@@ -10,3 +10,4 @@\n func example() {\n     // existing code\n+    // new line\n }"
  }
}
```

### Running Tests
```json
{
  "function": "run_tests",
  "parameters": {
    "target_path": "./...",
    "verbose": true,
    "timeout_seconds": 300
  }
}
```

## Safety Features

### Path Validation
- All file paths must be relative to the workspace root
- Absolute paths are rejected
- Path traversal attempts (e.g., `../../../etc/passwd`) are blocked

### Command Validation
- Shell commands are validated against an allowlist
- Dangerous commands (e.g., `rm -rf`, `sudo`) are blocked
- Timeouts prevent long-running commands from hanging

### Backup Creation
- Tools automatically create backups before destructive operations
- Rollback capabilities for failed operations
- Error handling with clear feedback

## Migration Benefits

### 1. Improved Reliability
- Function calls provide structured, validated operations
- Reduced risk of malformed file operations
- Better error handling and recovery

### 2. Enhanced Safety
- Path validation prevents directory traversal attacks
- Command validation blocks dangerous operations
- Automatic backup creation for destructive changes

### 3. Better Debugging
- Function calls are logged for audit trails
- Structured parameters make debugging easier
- Clear separation between planning and execution

### 4. More Sophisticated Behavior
- Agents can gather context before making decisions
- Iterative workflows for complex tasks
- Better integration between different operations

## Troubleshooting

### Common Issues

#### 1. "Path outside workspace" Error
**Cause:** Using absolute paths or path traversal
**Solution:** Use relative paths from workspace root
```json
// ❌ Wrong
{"target_file": "/absolute/path/file.go"}
{"target_file": "../../../outside/workspace"}

// ✅ Correct
{"target_file": "src/file.go"}
{"target_file": "internal/package/file.go"}
```

#### 2. "Command not allowed" Error
**Cause:** Attempting to run blocked shell commands
**Solution:** Use allowed commands or specific tools
```json
// ❌ Wrong
{"command": "sudo rm -rf /"}

// ✅ Correct
{"command": "go test ./..."}
{"command": "git status"}
```

#### 3. Function Call Not Recognized
**Cause:** Using old template format or incorrect function names
**Solution:** Use correct function names and parameters
```json
// ❌ Wrong
{"action": "read", "file": "test.go"}

// ✅ Correct
{"function": "read_file", "parameters": {"target_file": "test.go"}}
```

### Debugging Tips

1. **Check Function Names:** Ensure you're using the correct function names (`read_file`, `write_file`, etc.)

2. **Validate Parameters:** All parameters must match the expected JSON schema

3. **Use Relative Paths:** Always use paths relative to the workspace root

4. **Check Logs:** Function calls and their results are logged for debugging

5. **Test Incrementally:** Make small changes and validate them before proceeding

## Best Practices

### 1. Always Read Before Writing
```json
// First, read the existing file
{"function": "read_file", "parameters": {"target_file": "src/main.go"}}

// Then, modify it based on current content
{"function": "apply_patch_to_file", "parameters": {...}}
```

### 2. Use Appropriate Tools
- Use `apply_patch_to_file` for targeted changes
- Use `write_file` for new files or complete rewrites
- Use `codebase_search` to understand existing patterns

### 3. Validate Changes
```json
// After making changes, run tests
{"function": "run_tests", "parameters": {"target_path": "./..."}}

// Check for linting issues
{"function": "run_linter", "parameters": {"target_path": "."}}
```

### 4. Handle Errors Gracefully
- Check function call results before proceeding
- Provide clear error messages
- Use rollback capabilities when available

## Backward Compatibility

### Template Compatibility
- Old templates will continue to work but won't benefit from function calling
- Gradual migration is supported
- New features require updated templates

### Command Compatibility
- Existing commands work with both old and new templates
- `--use-orchestrator` flag enables function calling mode
- Default behavior remains unchanged for compatibility

## Performance Considerations

### Function Call Overhead
- Each function call has some overhead
- Batch operations when possible
- Use appropriate timeouts

### Context Management
- Function calls help manage context more efficiently
- Reduced need to pass large amounts of data in prompts
- Better memory usage for large codebases

## Future Enhancements

### Planned Features
1. **Enhanced Tool Suite:** More specialized tools for specific tasks
2. **Better Error Recovery:** Automatic rollback and retry mechanisms
3. **Performance Optimization:** Caching and batching improvements
4. **Advanced Validation:** More sophisticated safety checks

### Migration Path
1. **Phase 1:** Function calling templates (✅ Complete)
2. **Phase 2:** Enhanced tool suite
3. **Phase 3:** Advanced orchestration features
4. **Phase 4:** Performance optimizations

## Support

For issues or questions about the function-calling migration:

1. Check this documentation first
2. Review the troubleshooting section
3. Check the test suite for examples
4. File an issue with detailed error messages and context

## Examples Repository

See the `examples/` directory for complete examples of:
- Function-calling workflows
- Template usage patterns
- Common use cases
- Best practices implementation 