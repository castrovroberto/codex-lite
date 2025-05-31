package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/llm"
)

func TestDeliberationRunner_ClarificationRequest(t *testing.T) {
	// Create mock LLM client that will trigger clarification
	mockClient := &MockLLMClient{
		responses: []*llm.FunctionCallResponse{
			{
				IsTextResponse: false,
				FunctionCall: &llm.FunctionCall{
					Name: "request_human_clarification",
					Arguments: json.RawMessage(`{
						"question": "Should I proceed with this approach?",
						"context_summary": "Testing clarification functionality",
						"confidence_level": 0.6,
						"urgency": "medium"
					}`),
					ID: "test-call-1",
				},
			},
		},
	}

	// Create tool registry with clarification tool
	registry := agent.NewRegistry()
	clarificationTool := agent.NewClarificationTool("/tmp")
	registry.Register(clarificationTool)

	// Create deliberation config
	deliberationConfig := config.DeliberationConfig{
		Enabled:             true,
		ConfidenceThreshold: 0.7,
		MaxThoughtDepth:     3,
		RequireExplanation:  true,
		ThoughtTimeout:      30,
		EnableReflection:    false,
		VerifyHighRisk:      true,
		RequireConfirmation: false,
		HighRiskPatterns:    []string{"delete", "remove"},
	}

	// Create deliberation runner
	runner := NewDeliberationRunner(
		mockClient,
		registry,
		"You are a test assistant",
		"test-model",
		deliberationConfig,
	)

	// Set basic config
	runConfig := DefaultRunConfig()
	runConfig.MaxIterations = 2
	runner.SetConfig(runConfig)

	// Run with deliberation
	ctx := context.Background()
	result, err := runner.RunWithDeliberationAndCommand(ctx, "Test clarification request", "test")

	// Verify results
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check that deliberation steps were recorded
	if len(result.DeliberationSteps) == 0 {
		t.Error("Expected deliberation steps to be recorded")
	}

	t.Logf("Deliberation completed with %d steps and average confidence %.2f",
		result.ThoughtCount, result.AverageConfidence)
}

func TestDeliberationRunner_ThoughtGeneration(t *testing.T) {
	// Create mock LLM client that supports deliberation
	mockClient := &MockLLMClient{
		responses: []*llm.FunctionCallResponse{
			{
				IsTextResponse: true,
				TextContent:    "Task completed after deliberation",
			},
		},
	}

	// Create tool registry
	registry := agent.NewRegistry()

	// Create deliberation config
	deliberationConfig := config.DeliberationConfig{
		Enabled:             true,
		ConfidenceThreshold: 0.7,
		MaxThoughtDepth:     3,
		RequireExplanation:  true,
		ThoughtTimeout:      30,
		EnableReflection:    true,
		VerifyHighRisk:      true,
		RequireConfirmation: false,
		HighRiskPatterns:    []string{"delete", "remove"},
	}

	// Create deliberation runner
	runner := NewDeliberationRunner(
		mockClient,
		registry,
		"You are a thoughtful assistant",
		"test-model",
		deliberationConfig,
	)

	// Set basic config
	runConfig := DefaultRunConfig()
	runConfig.MaxIterations = 3
	runner.SetConfig(runConfig)

	// Run with deliberation
	ctx := context.Background()
	result, err := runner.RunWithDeliberationAndCommand(ctx, "Analyze this complex problem", "test")

	// Verify results
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check that thought steps were generated
	foundThought := false
	for _, step := range result.DeliberationSteps {
		if step.Phase == PhaseThought {
			foundThought = true
			break
		}
	}

	if !foundThought {
		t.Error("Expected thought generation steps to be recorded")
	}

	// Check that average confidence was calculated
	if result.AverageConfidence == 0.0 {
		t.Error("Expected average confidence to be calculated")
	}

	t.Logf("Deliberation generated %d thought steps with average confidence %.2f",
		result.ThoughtCount, result.AverageConfidence)
}

func TestClarificationTool_Execute(t *testing.T) {
	// Create clarification tool
	tool := agent.NewClarificationTool("/tmp")

	// Test valid clarification request
	params := json.RawMessage(`{
		"question": "Should I proceed with this approach?",
		"context_summary": "Testing clarification functionality",
		"confidence_level": 0.6,
		"urgency": "medium",
		"suggested_options": ["Option A", "Option B"]
	}`)

	ctx := context.Background()
	result, err := tool.Execute(ctx, params)

	// Verify results
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Success {
		t.Fatalf("Expected successful result, got error: %s", result.Error)
	}

	// Check that clarification data is present
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if clarificationNeeded, exists := data["clarification_needed"]; !exists || !clarificationNeeded.(bool) {
		t.Error("Expected clarification_needed to be true")
	}

	if question, exists := data["question"]; !exists || question.(string) != "Should I proceed with this approach?" {
		t.Error("Expected question to be preserved")
	}

	if confidence, exists := data["confidence_level"]; !exists || confidence.(float64) != 0.6 {
		t.Error("Expected confidence level to be preserved")
	}

	t.Logf("Clarification tool executed successfully")
}
