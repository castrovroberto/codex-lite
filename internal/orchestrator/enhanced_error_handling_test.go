package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFailingTool simulates a tool that fails with standardized errors
type MockFailingTool struct {
	name           string
	failureCount   int
	currentAttempt int
	errorCode      agent.ToolErrorCode
	errorMessage   string
}

func NewMockFailingTool(name string, failureCount int, errorCode agent.ToolErrorCode, errorMessage string) *MockFailingTool {
	return &MockFailingTool{
		name:         name,
		failureCount: failureCount,
		errorCode:    errorCode,
		errorMessage: errorMessage,
	}
}

func (t *MockFailingTool) Name() string {
	return t.name
}

func (t *MockFailingTool) Description() string {
	return "A mock tool that fails for testing retry mechanisms"
}

func (t *MockFailingTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"input": map[string]interface{}{
				"type":        "string",
				"description": "Test input parameter",
			},
		},
		"required": []string{"input"},
	}

	data, _ := json.Marshal(schema)
	return json.RawMessage(data)
}

func (t *MockFailingTool) Execute(ctx context.Context, args json.RawMessage) (*agent.ToolResult, error) {
	t.currentAttempt++

	// Fail for the specified number of attempts, then succeed
	if t.currentAttempt <= t.failureCount {
		standardizedError := agent.NewStandardizedError(
			t.errorCode,
			t.errorMessage,
			"Please check your parameters and try again",
		).WithDetail("attempt", t.currentAttempt).WithDetail("tool", t.name)

		result := &agent.ToolResult{
			Success: false,
			Error:   standardizedError.Error(),
		}
		result.SetError(standardizedError)

		return result, nil
	}

	// Success after failures
	return &agent.ToolResult{
		Success: true,
		Data:    map[string]interface{}{"result": "success", "attempts": t.currentAttempt},
	}, nil
}

// MockLLMClientForRetry simulates an LLM that responds to retry prompts
type MockLLMClientForRetry struct {
	responses []llm.FunctionCallResponse
	callCount int
}

func NewMockLLMClientForRetry() *MockLLMClientForRetry {
	return &MockLLMClientForRetry{
		responses: []llm.FunctionCallResponse{},
		callCount: 0,
	}
}

func (m *MockLLMClientForRetry) AddResponse(response llm.FunctionCallResponse) {
	m.responses = append(m.responses, response)
}

func (m *MockLLMClientForRetry) GenerateWithFunctions(ctx context.Context, model, prompt, systemPrompt string, tools []llm.ToolDefinition) (*llm.FunctionCallResponse, error) {
	if m.callCount >= len(m.responses) {
		// Default final response
		return &llm.FunctionCallResponse{
			IsTextResponse: true,
			TextContent:    "Task completed after retries",
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++
	return &response, nil
}

func (m *MockLLMClientForRetry) Generate(ctx context.Context, model, prompt, systemPrompt string) (string, error) {
	return "Mock response", nil
}

func (m *MockLLMClientForRetry) GenerateThought(ctx context.Context, modelName, prompt, context string) (*llm.ThoughtResponse, error) {
	return &llm.ThoughtResponse{
		ThoughtContent: "Mock thought",
		ReasoningSteps: []string{"Mock reasoning"},
		Confidence:     0.8,
	}, nil
}

func (m *MockLLMClientForRetry) AssessConfidence(ctx context.Context, modelName, thought, proposedAction string) (*llm.ConfidenceAssessment, error) {
	return &llm.ConfidenceAssessment{
		Score:          0.8,
		Recommendation: "proceed",
	}, nil
}

func (m *MockLLMClientForRetry) SupportsDeliberation() bool {
	return true
}

// Additional methods to implement llm.Client interface
func (m *MockLLMClientForRetry) Generate(ctx context.Context, modelName, prompt, systemPrompt string, tools []map[string]interface{}) (string, error) {
	return "Mock response", nil
}

func (m *MockLLMClientForRetry) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	out <- "Mock stream response"
	close(out)
	return nil
}

func (m *MockLLMClientForRetry) ListAvailableModels(ctx context.Context) ([]string, error) {
	return []string{"test-model"}, nil
}

func (m *MockLLMClientForRetry) SupportsNativeFunctionCalling() bool {
	return true
}

func (m *MockLLMClientForRetry) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockLLMClientForRetry) SupportsEmbeddings() bool {
	return true
}

func TestEnhancedErrorHandlingRetryMechanism(t *testing.T) {
	// Create a mock tool that fails twice then succeeds
	failingTool := NewMockFailingTool("test_tool", 2, agent.ErrorCodeInvalidParameters, "Invalid parameter format")

	// Create tool registry
	registry := agent.NewRegistry()
	registry.Register(failingTool)

	// Create mock LLM client
	mockLLM := NewMockLLMClientForRetry()

	// First call - tool call that will fail
	mockLLM.AddResponse(llm.FunctionCallResponse{
		IsTextResponse: false,
		FunctionCall: &llm.FunctionCall{
			ID:        "call_1",
			Name:      "test_tool",
			Arguments: json.RawMessage(`{"input": "test"}`),
		},
	})

	// Second call - retry after first failure
	mockLLM.AddResponse(llm.FunctionCallResponse{
		IsTextResponse: false,
		FunctionCall: &llm.FunctionCall{
			ID:        "call_2",
			Name:      "test_tool",
			Arguments: json.RawMessage(`{"input": "test_retry1"}`),
		},
	})

	// Third call - retry after second failure
	mockLLM.AddResponse(llm.FunctionCallResponse{
		IsTextResponse: false,
		FunctionCall: &llm.FunctionCall{
			ID:        "call_3",
			Name:      "test_tool",
			Arguments: json.RawMessage(`{"input": "test_retry2"}`),
		},
	})

	// Create enhanced agent runner with retry configuration
	runner := NewEnhancedAgentRunner(mockLLM, registry, "Test system prompt", "test-model")

	config := &RunConfig{
		MaxIterations:         10,
		MaxToolRetries:        3,
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: false,
		TimeoutSeconds:        30,
	}
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "Test the failing tool")

	// Verify results
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.ToolRetries) // Should have 2 retries
	assert.Equal(t, 1, result.ToolCalls)   // Should count as 1 successful tool call

	// Verify error analytics
	analytics := runner.GetErrorAnalytics()
	assert.Equal(t, 3, analytics["total_attempts"])  // 3 total attempts
	assert.Equal(t, 2, analytics["failed_attempts"]) // 2 failed attempts
	assert.Greater(t, analytics["retry_rate"], 0.0)  // Should have retry rate > 0

	// Verify tool attempts tracking
	attempts := analytics["tool_attempts"].([]ToolCallAttempt)
	assert.Len(t, attempts, 3)

	// First two attempts should have errors
	assert.NotEmpty(t, attempts[0].ErrorMessage)
	assert.Equal(t, string(agent.ErrorCodeInvalidParameters), attempts[0].ErrorCode)
	assert.NotEmpty(t, attempts[1].ErrorMessage)
	assert.Equal(t, string(agent.ErrorCodeInvalidParameters), attempts[1].ErrorCode)

	// Third attempt should succeed
	assert.Empty(t, attempts[2].ErrorMessage)
	assert.Empty(t, attempts[2].ErrorCode)
}

func TestEnhancedErrorHandlingMaxRetriesExceeded(t *testing.T) {
	// Create a tool that always fails
	failingTool := NewMockFailingTool("always_fail_tool", 10, agent.ErrorCodeFileNotFound, "File not found")

	// Create tool registry
	registry := agent.NewRegistry()
	registry.Register(failingTool)

	// Create mock LLM client
	mockLLM := NewMockLLMClientForRetry()

	// Add multiple tool call responses (will all fail)
	for i := 0; i < 5; i++ {
		mockLLM.AddResponse(llm.FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &llm.FunctionCall{
				ID:        "call_" + string(rune(i+1)),
				Name:      "always_fail_tool",
				Arguments: json.RawMessage(`{"input": "test"}`),
			},
		})
	}

	// Create enhanced agent runner with limited retries
	runner := NewEnhancedAgentRunner(mockLLM, registry, "Test system prompt", "test-model")

	config := &RunConfig{
		MaxIterations:         10,
		MaxToolRetries:        2, // Only allow 2 retries
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: false,
		TimeoutSeconds:        30,
	}
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "Test the always failing tool")

	// Verify results
	require.NoError(t, err)
	assert.True(t, result.Success)         // Should complete with final text response
	assert.Equal(t, 2, result.ToolRetries) // Should have exactly 2 retries
	assert.Equal(t, 1, result.ToolCalls)   // Should count as 1 tool call (failed)

	// Verify error analytics
	analytics := runner.GetErrorAnalytics()
	assert.Equal(t, 3, analytics["total_attempts"])  // 3 total attempts (1 + 2 retries)
	assert.Equal(t, 3, analytics["failed_attempts"]) // All 3 failed

	// Verify error history tracking
	errorHistory := analytics["error_history"].(map[string]int)
	assert.Equal(t, 3, errorHistory[string(agent.ErrorCodeFileNotFound)])
}

func TestEnhancedErrorHandlingNonRetriableErrors(t *testing.T) {
	// Create a tool that fails with a non-retriable error
	failingTool := NewMockFailingTool("non_retriable_tool", 10, agent.ErrorCodeUnsupportedOperation, "Operation not supported")

	// Create tool registry
	registry := agent.NewRegistry()
	registry.Register(failingTool)

	// Create mock LLM client
	mockLLM := NewMockLLMClientForRetry()

	// Add tool call response
	mockLLM.AddResponse(llm.FunctionCallResponse{
		IsTextResponse: false,
		FunctionCall: &llm.FunctionCall{
			ID:        "call_1",
			Name:      "non_retriable_tool",
			Arguments: json.RawMessage(`{"input": "test"}`),
		},
	})

	// Create enhanced agent runner
	runner := NewEnhancedAgentRunner(mockLLM, registry, "Test system prompt", "test-model")

	config := &RunConfig{
		MaxIterations:         10,
		MaxToolRetries:        3,
		RetryWithModification: true,
		EnableErrorAnalysis:   true,
		AbortOnRepeatedErrors: false,
		TimeoutSeconds:        30,
	}
	runner.SetConfig(config)

	// Run the agent
	ctx := context.Background()
	result, err := runner.Run(ctx, "Test the non-retriable tool")

	// Verify results
	require.NoError(t, err)
	assert.True(t, result.Success)         // Should complete with final text response
	assert.Equal(t, 0, result.ToolRetries) // Should have 0 retries (non-retriable error)
	assert.Equal(t, 1, result.ToolCalls)   // Should count as 1 tool call (failed)

	// Verify error analytics
	analytics := runner.GetErrorAnalytics()
	assert.Equal(t, 1, analytics["total_attempts"])  // Only 1 attempt
	assert.Equal(t, 1, analytics["failed_attempts"]) // 1 failed attempt
	assert.Equal(t, 0.0, analytics["retry_rate"])    // No retries
}

func TestStandardizedErrorFormatting(t *testing.T) {
	// Test the standardized error formatting for LLM
	err := agent.NewStandardizedError(
		agent.ErrorCodeInvalidPathFormat,
		"Path contains invalid characters",
		"Use relative paths without special characters",
	).WithDetail("path", "/invalid/../path").WithDetail("suggested", "/valid/path")

	formatted := err.FormatForLLM()

	// Verify the formatted error contains all expected elements
	assert.Contains(t, formatted, "Path contains invalid characters")
	assert.Contains(t, formatted, "Use relative paths without special characters")
	assert.Contains(t, formatted, "/invalid/../path")
	assert.Contains(t, formatted, "/valid/path")
}

func TestToolParameterValidation(t *testing.T) {
	validator := agent.NewToolValidator("/test/workspace")

	// Test file path validation
	t.Run("ValidFilePath", func(t *testing.T) {
		err := validator.ValidateFilePath("valid/file.txt")
		assert.NoError(t, err)
	})

	t.Run("InvalidFilePathWithDotDot", func(t *testing.T) {
		err := validator.ValidateFilePath("../invalid/path")
		assert.Error(t, err)

		// Should be a standardized error
		standardizedErr, ok := err.(*agent.StandardizedToolError)
		assert.True(t, ok)
		assert.Equal(t, agent.ErrorCodePathOutsideWorkspace, standardizedErr.Code)
	})

	t.Run("EmptyFilePath", func(t *testing.T) {
		err := validator.ValidateFilePath("")
		assert.Error(t, err)

		// Should be a standardized error
		standardizedErr, ok := err.(*agent.StandardizedToolError)
		assert.True(t, ok)
		assert.Equal(t, agent.ErrorCodeMissingParameter, standardizedErr.Code)
	})
}

func TestEnhancedFileWriteToolValidation(t *testing.T) {
	// Test the enhanced FileWriteTool with better validation
	tool := agent.NewFileWriteTool("/test/workspace")

	t.Run("InvalidFileWriteMissingPath", func(t *testing.T) {
		args := json.RawMessage(`{
			"content": "Hello, World!"
		}`)

		ctx := context.Background()
		result, err := tool.Execute(ctx, args)

		require.NoError(t, err)
		assert.False(t, result.Success)

		// Should be a standardized validation error
		require.NotNil(t, result.StandardizedError)
		assert.Equal(t, agent.ErrorCodeMissingParameter, result.StandardizedError.Code)
	})

	t.Run("InvalidFileWriteUnsafePath", func(t *testing.T) {
		args := json.RawMessage(`{
			"file_path": "../unsafe/path.txt",
			"content": "Hello, World!"
		}`)

		ctx := context.Background()
		result, err := tool.Execute(ctx, args)

		require.NoError(t, err)
		assert.False(t, result.Success)

		// Should be a standardized path validation error
		require.NotNil(t, result.StandardizedError)
		assert.Equal(t, agent.ErrorCodePathOutsideWorkspace, result.StandardizedError.Code)
	})
}

func TestErrorCodeSuggestions(t *testing.T) {
	suggestions := agent.GetErrorCodeSuggestions()

	// Verify that we have suggestions for key error codes
	assert.Contains(t, suggestions, agent.ErrorCodeInvalidParameters)
	assert.Contains(t, suggestions, agent.ErrorCodeMissingParameter)
	assert.Contains(t, suggestions, agent.ErrorCodeFileNotFound)
	assert.Contains(t, suggestions, agent.ErrorCodePathOutsideWorkspace)

	// Verify suggestions are not empty
	assert.NotEmpty(t, suggestions[agent.ErrorCodeInvalidParameters])
	assert.NotEmpty(t, suggestions[agent.ErrorCodeMissingParameter])
}

func TestToolResultErrorHandling(t *testing.T) {
	// Test the enhanced ToolResult error handling
	result := &agent.ToolResult{
		Success: false,
	}

	// Test setting a standardized error
	standardizedErr := agent.NewStandardizedError(
		agent.ErrorCodeFileNotFound,
		"Test file not found",
		"Check the file path and try again",
	)

	result.SetError(standardizedErr)

	// Verify both error fields are set correctly
	assert.False(t, result.Success)
	assert.Equal(t, standardizedErr.Error(), result.Error)
	assert.Equal(t, standardizedErr, result.StandardizedError)

	// Test setting a regular error
	result2 := &agent.ToolResult{
		Success: false,
	}

	regularErr := fmt.Errorf("regular error")
	result2.SetError(regularErr)

	assert.False(t, result2.Success)
	assert.Equal(t, "regular error", result2.Error)
	assert.Nil(t, result2.StandardizedError)
}
