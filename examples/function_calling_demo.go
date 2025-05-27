package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// DemoLLMClient simulates an LLM that can make function calls
type DemoLLMClient struct {
	step int
}

func (d *DemoLLMClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	return "Demo response", nil
}

func (d *DemoLLMClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []llm.ToolDefinition) (*llm.FunctionCallResponse, error) {
	d.step++

	switch d.step {
	case 1:
		// First call: List directory to see what's there
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "list_directory",
				Arguments: json.RawMessage(`{"directory_path": ".", "recursive": false}`),
				ID:        "call_1",
			},
		}, nil
	case 2:
		// Second call: Create a hello world file
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "write_file",
				Arguments: json.RawMessage(`{"file_path": "hello_world.go", "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}"}`),
				ID:        "call_2",
			},
		}, nil
	case 3:
		// Third call: Read the file back to verify
		return &llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				Name:      "read_file",
				Arguments: json.RawMessage(`{"target_file": "hello_world.go"}`),
				ID:        "call_3",
			},
		}, nil
	default:
		// Final response
		return &llm.FunctionCallResponse{
			IsTextResponse: true,
			TextContent:    "Task completed successfully! I've created a hello_world.go file with a simple Go program that prints 'Hello, World!'. The file has been verified and is ready to run.",
		}, nil
	}
}

func (d *DemoLLMClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	return nil
}

func (d *DemoLLMClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	return []string{"demo-model"}, nil
}

func (d *DemoLLMClient) SupportsNativeFunctionCalling() bool {
	return true
}

func (d *DemoLLMClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a mock embedding vector for demo purposes
	return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (d *DemoLLMClient) SupportsEmbeddings() bool {
	return false // Demo client doesn't really support embeddings
}

func main() {
	fmt.Println("🚀 CGE Function-Calling Infrastructure Demo")
	fmt.Println("==========================================")

	// Get current working directory as workspace
	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	fmt.Printf("Workspace: %s\n\n", workspaceRoot)

	// Create demo LLM client
	llmClient := &DemoLLMClient{}

	// Create tool registry with generation tools
	factory := agent.NewToolFactory(workspaceRoot)
	registry := factory.CreateGenerationRegistry()

	fmt.Printf("Available tools: %v\n\n", registry.GetToolNames())

	// Create agent runner
	systemPrompt := `You are a helpful coding assistant. You can use the available tools to:
- Read and write files
- List directory contents  
- Search through code
- Apply patches to files
- Perform Git operations

Use the tools to complete the user's request step by step.`

	runner := orchestrator.NewAgentRunner(
		llmClient,
		registry,
		systemPrompt,
		"demo-model",
	)

	// Configure the runner with custom max iterations
	config := orchestrator.DefaultRunConfig()
	config.MaxIterations = 10
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	userRequest := "Create a simple Hello World Go program and verify it was created correctly"

	fmt.Printf("User Request: %s\n", userRequest)
	fmt.Println("\n🔄 Agent Execution:")
	fmt.Println("-------------------")

	result, err := runner.Run(ctx, userRequest)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// Display results
	fmt.Printf("\n📊 Execution Summary:\n")
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Tool Calls: %d\n", result.ToolCalls)

	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}

	fmt.Printf("\n💬 Final Response:\n%s\n", result.FinalResponse)

	// Show message history
	fmt.Printf("\n📝 Conversation History:\n")
	for i, msg := range result.Messages {
		fmt.Printf("%d. [%s] ", i+1, msg.Role)
		if msg.ToolCall != nil {
			fmt.Printf("Called tool: %s\n", msg.ToolCall.Name)
		} else if msg.Content != "" {
			// Truncate long content for display
			content := msg.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("%s\n", content)
		}
	}

	// Check if the file was actually created
	fmt.Printf("\n🔍 Verification:\n")
	if _, err := os.Stat("hello_world.go"); err == nil {
		fmt.Println("✅ hello_world.go file was successfully created!")

		// Read and display the content
		content, err := os.ReadFile("hello_world.go")
		if err == nil {
			fmt.Printf("\n📄 File Content:\n%s\n", string(content))
		}
	} else {
		fmt.Println("❌ hello_world.go file was not found")
	}

	fmt.Println("\n🎉 Demo completed!")
}
