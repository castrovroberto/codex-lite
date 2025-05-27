package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/llm"
)

// MockLLMClient for testing
type MockLLMClient struct {
	responses []*llm.FunctionCallResponse
	callIndex int
}

func (m *MockLLMClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	return "Mock response", nil
}

func (m *MockLLMClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []llm.ToolDefinition) (*llm.FunctionCallResponse, error) {
	if m.callIndex >= len(m.responses) {
		return &llm.FunctionCallResponse{
			IsTextResponse: true,
			TextContent:    "Task completed successfully",
		}, nil
	}

	response := m.responses[m.callIndex]
	m.callIndex++
	return response, nil
}

func (m *MockLLMClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	return nil
}

func (m *MockLLMClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	return []string{"mock-model"}, nil
}

func (m *MockLLMClient) SupportsNativeFunctionCalling() bool {
	return true
}

// MockTool for testing
type MockTool struct {
	name        string
	description string
	parameters  json.RawMessage
	result      *agent.ToolResult
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return m.description
}

func (m *MockTool) Parameters() json.RawMessage {
	return m.parameters
}

func (m *MockTool) Execute(ctx context.Context, params json.RawMessage) (*agent.ToolResult, error) {
	return m.result, nil
}

func TestAgentRunner_BasicFunctionCalling(t *testing.T) {
	// Create mock LLM client that will make a function call then return text
	mockClient := &MockLLMClient{
		responses: []*llm.FunctionCallResponse{
			{
				IsTextResponse: false,
				FunctionCall: &llm.FunctionCall{
					Name:      "test_tool",
					Arguments: json.RawMessage(`{"message": "hello"}`),
					ID:        "call_1",
				},
			},
		},
	}

	// Create mock tool
	mockTool := &MockTool{
		name:        "test_tool",
		description: "A test tool",
		parameters:  json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}}`),
		result: &agent.ToolResult{
			Success: true,
			Data:    map[string]interface{}{"response": "Tool executed successfully"},
		},
	}

	// Create registry and register mock tool
	registry := agent.NewRegistry()
	err := registry.Register(mockTool)
	if err != nil {
		t.Fatalf("Failed to register mock tool: %v", err)
	}

	// Create agent runner
	runner := NewAgentRunner(mockClient, registry, "You are a helpful assistant", "mock-model")
	config := DefaultRunConfig()
	config.MaxIterations = 5
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "Please use the test tool")

	// Verify results
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful run, got: %s", result.Error)
	}

	if result.ToolCalls != 1 {
		t.Errorf("Expected 1 tool call, got %d", result.ToolCalls)
	}

	if result.Iterations != 2 {
		t.Errorf("Expected 2 iterations, got %d", result.Iterations)
	}

	if result.FinalResponse != "Task completed successfully" {
		t.Errorf("Expected 'Task completed successfully', got '%s'", result.FinalResponse)
	}
}

func TestAgentRunner_TextOnlyResponse(t *testing.T) {
	// Create mock LLM client that returns only text
	mockClient := &MockLLMClient{
		responses: []*llm.FunctionCallResponse{
			{
				IsTextResponse: true,
				TextContent:    "I can help you with that task. The answer is 42. Task completed successfully.",
			},
		},
	}

	// Create empty registry
	registry := agent.NewRegistry()

	// Create agent runner
	runner := NewAgentRunner(mockClient, registry, "You are a helpful assistant", "mock-model")

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "What is the answer to everything?")

	// Verify results
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful run, got: %s", result.Error)
	}

	if result.ToolCalls != 0 {
		t.Errorf("Expected 0 tool calls, got %d", result.ToolCalls)
	}

	if result.Iterations != 1 {
		t.Errorf("Expected 1 iteration, got %d", result.Iterations)
	}

	if result.FinalResponse != "I can help you with that task. The answer is 42. Task completed successfully." {
		t.Errorf("Unexpected final response: %s", result.FinalResponse)
	}
}

func TestAgentRunner_MaxIterations(t *testing.T) {
	// Create mock LLM client that keeps making function calls
	mockClient := &MockLLMClient{
		responses: []*llm.FunctionCallResponse{
			{
				IsTextResponse: false,
				FunctionCall: &llm.FunctionCall{
					Name:      "test_tool",
					Arguments: json.RawMessage(`{"message": "call1"}`),
					ID:        "call_1",
				},
			},
			{
				IsTextResponse: false,
				FunctionCall: &llm.FunctionCall{
					Name:      "test_tool",
					Arguments: json.RawMessage(`{"message": "call2"}`),
					ID:        "call_2",
				},
			},
			{
				IsTextResponse: false,
				FunctionCall: &llm.FunctionCall{
					Name:      "test_tool",
					Arguments: json.RawMessage(`{"message": "call3"}`),
					ID:        "call_3",
				},
			},
		},
	}

	// Create mock tool
	mockTool := &MockTool{
		name:        "test_tool",
		description: "A test tool",
		parameters:  json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}}`),
		result: &agent.ToolResult{
			Success: true,
			Data:    map[string]interface{}{"response": "Tool executed"},
		},
	}

	// Create registry and register mock tool
	registry := agent.NewRegistry()
	err := registry.Register(mockTool)
	if err != nil {
		t.Fatalf("Failed to register mock tool: %v", err)
	}

	// Create agent runner with low max iterations
	runner := NewAgentRunner(mockClient, registry, "You are a helpful assistant", "mock-model")
	config := DefaultRunConfig()
	config.MaxIterations = 2
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "Keep using the test tool")

	// Verify results
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if result.Success {
		t.Error("Expected unsuccessful run due to max iterations")
	}

	if result.Iterations != 2 {
		t.Errorf("Expected 2 iterations, got %d", result.Iterations)
	}

	if !contains(result.Error, "maximum iterations") {
		t.Errorf("Expected error about max iterations, got: %s", result.Error)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
