package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestClarificationTool_Execute(t *testing.T) {
	// Create clarification tool
	tool := NewClarificationTool("/tmp")

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

func TestClarificationTool_InvalidParams(t *testing.T) {
	// Create clarification tool
	tool := NewClarificationTool("/tmp")

	// Test invalid JSON
	params := json.RawMessage(`{invalid json}`)

	ctx := context.Background()
	result, err := tool.Execute(ctx, params)

	// Verify error handling
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	if result != nil && result.Success {
		t.Error("Expected failure for invalid JSON")
	}
}

func TestClarificationTool_MissingQuestion(t *testing.T) {
	// Create clarification tool
	tool := NewClarificationTool("/tmp")

	// Test missing required field
	params := json.RawMessage(`{
		"context_summary": "Testing clarification functionality",
		"confidence_level": 0.6,
		"urgency": "medium"
	}`)

	ctx := context.Background()
	result, err := tool.Execute(ctx, params)

	// Verify error handling
	if err == nil {
		t.Error("Expected error for missing question")
	}

	if result != nil && result.Success {
		t.Error("Expected failure for missing question")
	}
}
