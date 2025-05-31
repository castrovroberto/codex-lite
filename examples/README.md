# CGE Examples

This directory contains practical examples of how to use CGE's main features.

## Quick Start Workflow

Here's a typical workflow using CGE's three main commands:

### 1. Plan Phase
```bash
# Generate a development plan for adding a new feature
./cge plan "Add user authentication with JWT tokens" --output auth_plan.json

# Plan a refactoring task
./cge plan "Refactor the database layer to use repository pattern" --output refactor_plan.json
```

### 2. Generate Phase
```bash
# Preview what changes would be made (dry run)
./cge generate --plan auth_plan.json --dry-run

# Apply changes directly to codebase
./cge generate --plan auth_plan.json --apply

# Generate changes to a separate directory for review
./cge generate --plan auth_plan.json --output-dir ./generated_changes

# Process only specific tasks
./cge generate --plan auth_plan.json --task "authentication" --dry-run
```

### 3. Review Phase
```bash
# Review the current codebase
./cge review

# Review with automatic fixes
./cge review --auto-fix --max-cycles 5

# Review specific directory with custom commands
./cge review ./src --test-cmd "npm test" --lint-cmd "eslint ."
```

## Example Scenarios

### Scenario 1: Adding a New API Endpoint
```bash
# 1. Plan the feature
./cge plan "Add REST API endpoint for user profile management with CRUD operations" --output profile_api_plan.json

# 2. Review the plan (check the generated JSON)
cat profile_api_plan.json

# 3. Generate the code (dry run first)
./cge generate --plan profile_api_plan.json --dry-run

# 4. Apply the changes
./cge generate --plan profile_api_plan.json --apply

# 5. Review and validate
./cge review --auto-fix
```

### Scenario 2: Refactoring Legacy Code
```bash
# 1. Plan the refactoring
./cge plan "Refactor the monolithic service into microservices architecture" --output microservices_plan.json

# 2. Generate changes incrementally
./cge generate --plan microservices_plan.json --task "user-service" --output-dir ./user_service_changes
./cge generate --plan microservices_plan.json --task "auth-service" --output-dir ./auth_service_changes

# 3. Review each service separately
./cge review ./user_service_changes --test-cmd "go test ./user-service/..."
./cge review ./auth_service_changes --test-cmd "go test ./auth-service/..."
```

### Scenario 3: Bug Fix Workflow
```bash
# 1. Plan the bug fix
./cge plan "Fix memory leak in the connection pool manager" --output bugfix_plan.json

# 2. Apply the fix
./cge generate --plan bugfix_plan.json --apply

# 3. Validate the fix
./cge review --auto-fix --max-cycles 3
```

## Tips and Best Practices

1. **Always start with a dry run**: Use `--dry-run` to preview changes before applying them
2. **Use descriptive plan names**: Clear descriptions help the LLM generate better plans
3. **Review incrementally**: For large changes, process tasks one at a time
4. **Configure your tools**: Set up proper test and lint commands in `codex.toml`
5. **Version control**: Commit your changes before running CGE commands
6. **Iterative approach**: Use the review command to continuously improve code quality

## Configuration Examples

### For Go Projects
```toml
[commands.review]
  test_command = "go test ./..."
  lint_command = "golangci-lint run"
  max_cycles = 3
```

### For Node.js Projects
```toml
[commands.review]
  test_command = "npm test"
  lint_command = "eslint . && prettier --check ."
  max_cycles = 5
```

### For Python Projects
```toml
[commands.review]
  test_command = "python -m pytest"
  lint_command = "flake8 . && black --check ."
  max_cycles = 4
``` 