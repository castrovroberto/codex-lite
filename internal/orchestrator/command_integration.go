package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/templates"
)

// CommandIntegrator provides high-level integration between commands and the orchestrator
type CommandIntegrator struct {
	llmClient          llm.Client
	toolRegistry       *agent.Registry
	config             config.IntegratorConfig
	templateEngine     *templates.Engine
	deliberationConfig config.DeliberationConfig
}

// NewCommandIntegrator creates a new command integrator
func NewCommandIntegrator(llmClient llm.Client, toolRegistry *agent.Registry, cfg config.IntegratorConfig) *CommandIntegrator {
	templateEngine := templates.NewEngine(cfg.PromptsDir)

	return &CommandIntegrator{
		llmClient:      llmClient,
		toolRegistry:   toolRegistry,
		config:         cfg,
		templateEngine: templateEngine,
		// Default deliberation config - can be overridden with SetDeliberationConfig
		deliberationConfig: config.DeliberationConfig{
			Enabled:             false,
			ConfidenceThreshold: 0.7,
			MaxThoughtDepth:     3,
			RequireExplanation:  true,
			ThoughtTimeout:      30,
			EnableReflection:    false,
			VerifyHighRisk:      true,
			RequireConfirmation: false,
			HighRiskPatterns:    []string{"delete", "remove", "drop", "truncate"},
		},
	}
}

// SetDeliberationConfig sets the deliberation configuration
func (ci *CommandIntegrator) SetDeliberationConfig(config config.DeliberationConfig) {
	ci.deliberationConfig = config
}

// RunnerInterface defines the interface for both regular and deliberation runners
type RunnerInterface interface {
	RunWithCommand(ctx context.Context, initialPrompt string, command string) (RunnerResult, error)
	SetConfig(config *RunConfig)
}

// RunnerResult is a common interface for both RunResult and DeliberationResult
type RunnerResult interface {
	GetFinalResponse() string
	GetMessages() []Message
	GetSuccess() bool
	GetError() string
}

// Ensure RunResult implements RunnerResult
func (r *RunResult) GetFinalResponse() string { return r.FinalResponse }
func (r *RunResult) GetMessages() []Message   { return r.Messages }
func (r *RunResult) GetSuccess() bool         { return r.Success }
func (r *RunResult) GetError() string         { return r.Error }

// Ensure DeliberationResult implements RunnerResult
func (dr *DeliberationResult) GetFinalResponse() string { return dr.RunResult.FinalResponse }
func (dr *DeliberationResult) GetMessages() []Message   { return dr.RunResult.Messages }
func (dr *DeliberationResult) GetSuccess() bool         { return dr.RunResult.Success }
func (dr *DeliberationResult) GetError() string         { return dr.RunResult.Error }

// PlanRequest represents a planning request
type PlanRequest struct {
	UserGoal        string
	Model           string
	CodebaseContext interface{} // From context gatherer
}

// PlanResponse represents a planning response
type PlanResponse struct {
	Plan     interface{} `json:"plan"`     // The generated plan
	Messages []Message   `json:"messages"` // Conversation history
	Success  bool        `json:"success"`
}

// ExecutePlan runs the planning orchestrator
func (ci *CommandIntegrator) ExecutePlan(ctx context.Context, req *PlanRequest) (*PlanResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Prepare system prompt for planning
	systemPrompt := `You are an expert software architect and project planner. 

Your task is to analyze the user's goal and the provided codebase context to create a detailed development plan.

You have access to tools to read files and explore the codebase structure. Use these tools to gather any additional context you need.

Your final response must be a valid JSON plan following this structure:
{
  "overall_goal": "string",
  "tasks": [
    {
      "id": "string",
      "description": "string", 
      "files_to_modify": ["string"],
      "files_to_create": ["string"],
      "files_to_delete": ["string"],
      "estimated_effort": "small|medium|large",
      "dependencies": ["string"],
      "rationale": "string"
    }
  ],
  "summary": "string",
  "estimated_total_effort": "string",
  "risks_and_considerations": ["string"]
}

Use tools to explore the codebase as needed, then provide the final plan in JSON format.`

	// Create agent runner with plan configuration
	runner, err := ci.createRunner(systemPrompt, req.Model, PlanRunConfig())
	if err != nil {
		log.Error("Plan orchestration failed", "error", err)
		return nil, fmt.Errorf("plan orchestration failed: %w", err)
	}

	// Prepare initial prompt with context
	initialPrompt := fmt.Sprintf(`User Goal: %s

Please analyze this goal and create a development plan. Use the available tools to explore the codebase structure and gather any additional context you need before creating the plan.

Codebase Context:
%v`, req.UserGoal, req.CodebaseContext)

	// Run the orchestrator
	result, err := runner.RunWithCommand(ctx, initialPrompt, "")
	if err != nil {
		log.Error("Plan orchestration failed", "error", err)
		return nil, fmt.Errorf("plan orchestration failed: %w", err)
	}

	// Parse the final response as JSON plan
	var plan interface{}
	if result.GetFinalResponse() != "" {
		if err := json.Unmarshal([]byte(result.GetFinalResponse()), &plan); err != nil {
			log.Error("Failed to parse plan JSON", "error", err, "response", result.GetFinalResponse())
			return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
		}
	}

	return &PlanResponse{
		Plan:     plan,
		Messages: result.GetMessages(),
		Success:  result.GetSuccess(),
	}, nil
}

// GenerateRequest represents a code generation request
type GenerateRequest struct {
	Task         interface{} // PlanTask
	Plan         interface{} // Overall plan
	Model        string
	DryRun       bool
	ApplyChanges bool
}

// GenerateResponse represents a code generation response
type GenerateResponse struct {
	Changes  []interface{} `json:"changes"`  // Generated changes
	Messages []Message     `json:"messages"` // Conversation history
	Success  bool          `json:"success"`
}

// ExecuteGenerate runs the code generation orchestrator
func (ci *CommandIntegrator) ExecuteGenerate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Prepare system prompt for generation
	systemPrompt := `You are an expert software engineer specializing in code generation.

Your task is to implement the given task by making precise code changes. You have access to tools to:
- Read existing files
- Write new files or modify existing ones
- Apply patches/diffs
- List directory contents
- Run shell commands when necessary

For each change you make:
1. First read the existing file (if modifying)
2. Make the necessary changes using write_file or apply_patch_to_file
3. Ensure your changes are precise and follow best practices

Work systematically through the task requirements. When you have completed all necessary changes, provide a summary of what was implemented.`

	// Create agent runner with generate configuration
	runner, err := ci.createRunner(systemPrompt, req.Model, GenerateRunConfig())
	if err != nil {
		log.Error("Generate orchestration failed", "error", err)
		return nil, fmt.Errorf("generate orchestration failed: %w", err)
	}

	// Prepare initial prompt with task details
	taskJSON, _ := json.MarshalIndent(req.Task, "", "  ")
	planJSON, _ := json.MarshalIndent(req.Plan, "", "  ")

	initialPrompt := fmt.Sprintf(`Task to implement:
%s

Overall Plan Context:
%s

Please implement this task by making the necessary code changes. Use the available tools to read existing files, create new files, and apply modifications as needed.

%s`, string(taskJSON), string(planJSON),
		func() string {
			if req.DryRun {
				return "NOTE: This is a dry run. Do not actually modify files, but describe what changes you would make."
			}
			return ""
		}())

	// Run the orchestrator
	result, err := runner.RunWithCommand(ctx, initialPrompt, "")
	if err != nil {
		log.Error("Generate orchestration failed", "error", err)
		return nil, fmt.Errorf("generate orchestration failed: %w", err)
	}

	// Extract changes from tool calls
	var changes []interface{}
	for _, msg := range result.GetMessages() {
		if msg.Role == "tool" && (msg.Name == "write_file" || msg.Name == "apply_patch_to_file") {
			changes = append(changes, map[string]interface{}{
				"tool":    msg.Name,
				"content": msg.Content,
			})
		}
	}

	return &GenerateResponse{
		Changes:  changes,
		Messages: result.GetMessages(),
		Success:  result.GetSuccess(),
	}, nil
}

// ReviewRequest represents a code review request
type ReviewRequest struct {
	TargetDir  string
	TestOutput string
	LintOutput string
	Model      string
	MaxCycles  int
}

// ReviewResponse represents a code review response
type ReviewResponse struct {
	FixesApplied []string  `json:"fixes_applied"`
	Messages     []Message `json:"messages"`
	Success      bool      `json:"success"`
}

// ExecuteReview runs the code review orchestrator
func (ci *CommandIntegrator) ExecuteReview(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Load the review template
	reviewTemplateData := map[string]interface{}{
		"TargetDir":  req.TargetDir,
		"TestOutput": req.TestOutput,
		"LintOutput": req.LintOutput,
		"MaxCycles":  req.MaxCycles,
	}

	systemPrompt, err := ci.templateEngine.Render("review_orchestrated.tmpl", reviewTemplateData)
	if err != nil {
		// Fallback to hardcoded prompt if template fails
		log.Warn("Failed to load review template, using fallback", "error", err)
		systemPrompt = `You are an expert software engineer specializing in code review and debugging.

Your task is to analyze test failures and linting issues, then fix them by making precise code changes.

You have access to tools to:
- Read files to understand the current code
- Apply patches to fix issues
- Run tests to verify fixes
- Run linters to check code quality

For each issue:
1. Analyze the error message to understand the problem
2. Read the relevant files to see the current code
3. Make targeted fixes using apply_patch_to_file
4. Verify the fix by running tests/linters again

Work systematically through all issues. Focus on making minimal, precise changes that address the root cause.`
	}

	// Create agent runner with review configuration
	runner, err := ci.createRunner(systemPrompt, req.Model, ReviewRunConfig())
	if err != nil {
		log.Error("Review orchestration failed", "error", err)
		return nil, fmt.Errorf("review orchestration failed: %w", err)
	}

	// Prepare initial prompt with test/lint results
	initialPrompt := fmt.Sprintf(`Please analyze and fix the following issues:

Test Output:
%s

Lint Output:
%s

Target Directory: %s

Please use the available tools to read the relevant files, understand the issues, and apply fixes. After making changes, run the tests and linter again to verify the fixes.`,
		req.TestOutput, req.LintOutput, req.TargetDir)

	// Run the orchestrator
	result, err := runner.RunWithCommand(ctx, initialPrompt, "")
	if err != nil {
		log.Error("Review orchestration failed", "error", err)
		return nil, fmt.Errorf("review orchestration failed: %w", err)
	}

	// Extract fixes from tool calls
	var fixes []string
	for _, msg := range result.GetMessages() {
		if msg.Role == "tool" && msg.Name == "apply_patch_to_file" {
			fixes = append(fixes, fmt.Sprintf("Applied patch via %s", msg.Name))
		}
	}

	return &ReviewResponse{
		FixesApplied: fixes,
		Messages:     result.GetMessages(),
		Success:      result.GetSuccess(),
	}, nil
}

// createRunner creates either a regular or deliberation-enabled runner based on configuration
func (ci *CommandIntegrator) createRunner(systemPrompt, model string, runConfig *RunConfig) (RunnerInterface, error) {
	if ci.deliberationConfig.Enabled {
		// Create deliberation-enabled runner
		deliberationRunner := NewDeliberationRunner(
			ci.llmClient,
			ci.toolRegistry,
			systemPrompt,
			model,
			ci.deliberationConfig,
		)
		deliberationRunner.SetConfig(runConfig)
		return &DeliberationRunnerWrapper{deliberationRunner}, nil
	} else {
		// Create regular runner
		regularRunner := NewAgentRunner(ci.llmClient, ci.toolRegistry, systemPrompt, model)
		regularRunner.SetConfig(runConfig)
		return &AgentRunnerWrapper{regularRunner}, nil
	}
}

// AgentRunnerWrapper wraps AgentRunner to implement RunnerInterface
type AgentRunnerWrapper struct {
	*AgentRunner
}

func (arw *AgentRunnerWrapper) RunWithCommand(ctx context.Context, initialPrompt string, command string) (RunnerResult, error) {
	result, err := arw.AgentRunner.RunWithCommand(ctx, initialPrompt, command)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeliberationRunnerWrapper wraps DeliberationRunner to implement RunnerInterface
type DeliberationRunnerWrapper struct {
	*DeliberationRunner
}

func (drw *DeliberationRunnerWrapper) RunWithCommand(ctx context.Context, initialPrompt string, command string) (RunnerResult, error) {
	result, err := drw.DeliberationRunner.RunWithDeliberationAndCommand(ctx, initialPrompt, command)
	if err != nil {
		return nil, err
	}
	return result, nil
}
