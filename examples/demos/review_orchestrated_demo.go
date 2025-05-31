package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// DemoReviewLLMClient simulates an LLM that can perform code review tasks
type DemoReviewLLMClient struct {
	step int
}

func (d *DemoReviewLLMClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	return "Demo response", nil
}

func (d *DemoReviewLLMClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []llm.ToolDefinition) (*llm.FunctionCallResponse, error) {
	d.step++

	switch d.step {
	case 1:
		// First: Parse test results to understand failures
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "parse_test_results",
				Arguments: []byte(`{"test_output": "=== RUN TestExample\n--- FAIL: TestExample (0.00s)\n    example_test.go:10: expected 5, got 3\nFAIL\nexit status 1"}`),
				ID:        "call_1",
			},
		}, nil
	case 2:
		// Second: Parse lint results
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "parse_lint_results",
				Arguments: []byte(`{"lint_output": "example.go:5:1: exported function Example should have comment or be unexported (missing-doc)"}`),
				ID:        "call_2",
			},
		}, nil
	case 3:
		// Third: Read the problematic file
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "read_file",
				Arguments: []byte(`{"target_file": "example.go"}`),
				ID:        "call_3",
			},
		}, nil
	case 4:
		// Fourth: Apply a patch to fix the issue
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "apply_patch_to_file",
				Arguments: []byte(`{"file_path": "example.go", "patch_content": "@@ -2,6 +2,7 @@\n package main\n \n import \"fmt\"\n \n+// Example demonstrates a simple function\n func Example() int {\n     return 5\n }"}`),
				ID:        "call_4",
			},
		}, nil
	case 5:
		// Fifth: Run tests to verify the fix
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "run_tests",
				Arguments: []byte(`{"target_path": "."}`),
				ID:        "call_5",
			},
		}, nil
	default:
		// Final response
		return &llm.FunctionCallResponse{
			IsTextResponse: true,
			TextContent: `Code review completed successfully! 

## Issues Identified and Fixed:

1. **Test Failure**: TestExample was failing because the function returned 3 instead of 5
   - Root cause: Logic error in the Example function
   - Fix: Updated the function to return the correct value

2. **Linting Issue**: Missing documentation for exported function
   - Root cause: Function Example lacked proper documentation
   - Fix: Added appropriate comment above the function

## Verification Results:
- ✅ All tests now pass
- ✅ All linting issues resolved

The code is now in a clean, working state with proper documentation and passing tests.`,
		}, nil
	}
}

func (d *DemoReviewLLMClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	return nil
}

func (d *DemoReviewLLMClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	return []string{"demo-review-model"}, nil
}

func (d *DemoReviewLLMClient) SupportsNativeFunctionCalling() bool {
	return true
}

func (d *DemoReviewLLMClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (d *DemoReviewLLMClient) SupportsEmbeddings() bool {
	return false
}

func main() {
	fmt.Println("🔍 CGE Orchestrated Review Demo")
	fmt.Println("===============================")

	// Get current working directory as workspace
	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	fmt.Printf("Workspace: %s\n\n", workspaceRoot)

	// Create demo LLM client
	llmClient := &DemoReviewLLMClient{}

	// Create tool registry with review tools (including new parse tools)
	factory := agent.NewToolFactory(workspaceRoot)
	registry := factory.CreateReviewRegistry()

	fmt.Printf("Available review tools: %v\n\n", registry.GetToolNames())

	// Create agent runner with review-specific system prompt
	systemPrompt := `You are an expert software engineer specializing in code review and debugging.

Your task is to analyze test failures and linting issues, then systematically fix them using the available tools.

Available tools include:
- parse_test_results: Parse raw test output into structured data
- parse_lint_results: Parse raw lint output into structured data  
- read_file: Read file contents to understand current code
- apply_patch_to_file: Apply targeted patches to fix issues
- run_tests: Execute tests to verify fixes
- run_linter: Run linting tools to check code quality

Follow this process:
1. Parse the test and lint outputs to understand issues
2. Read relevant files to understand the current code
3. Apply targeted fixes using patches
4. Verify fixes by running tests and linters
5. Provide a summary of changes made`

	runner := orchestrator.NewAgentRunner(
		llmClient,
		registry,
		systemPrompt,
		"demo-review-model",
	)

	// Configure the runner for review tasks
	config := orchestrator.ReviewRunConfig()
	config.MaxIterations = 10
	runner.SetConfig(config)

	// Simulate a review request with test and lint failures
	ctx := context.Background()
	userRequest := `Please review and fix the following issues:

Test Output:
=== RUN TestExample
--- FAIL: TestExample (0.00s)
    example_test.go:10: expected 5, got 3
FAIL
exit status 1

Lint Output:
example.go:5:1: exported function Example should have comment or be unexported (missing-doc)

Please analyze these issues and apply appropriate fixes.`

	fmt.Printf("🚀 Starting orchestrated review...\n")
	fmt.Printf("Request: %s\n\n", userRequest)

	// Run the orchestrated review
	result, err := runner.Run(ctx, userRequest)
	if err != nil {
		log.Fatalf("Review failed: %v", err)
	}

	// Display results
	fmt.Printf("📊 Review Results:\n")
	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Tool Calls: %d\n", result.ToolCalls)
	fmt.Printf("\n📝 Final Response:\n%s\n", result.FinalResponse)

	// Show the conversation flow
	fmt.Printf("\n🔄 Conversation Flow:\n")
	for i, msg := range result.Messages {
		switch msg.Role {
		case "user":
			fmt.Printf("%d. User: %s\n", i+1, truncateString(msg.Content, 100))
		case "assistant":
			if msg.ToolCall != nil {
				fmt.Printf("%d. Assistant: Called tool '%s'\n", i+1, msg.ToolCall.Name)
			} else {
				fmt.Printf("%d. Assistant: %s\n", i+1, truncateString(msg.Content, 100))
			}
		case "tool":
			fmt.Printf("%d. Tool (%s): %s\n", i+1, msg.Name, truncateString(msg.Content, 100))
		}
	}

	fmt.Printf("\n✅ Orchestrated review demo completed successfully!\n")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
