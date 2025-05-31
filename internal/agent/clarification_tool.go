package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// ClarificationTool allows the agent to request human clarification
type ClarificationTool struct {
	validator *ToolValidator
}

// NewClarificationTool creates a new clarification request tool
func NewClarificationTool(workspaceRoot string) *ClarificationTool {
	return &ClarificationTool{
		validator: NewToolValidator(workspaceRoot),
	}
}

// Name returns the tool name
func (t *ClarificationTool) Name() string {
	return "request_human_clarification"
}

// Description returns the tool description
func (t *ClarificationTool) Description() string {
	return `Request clarification from the human user when instructions are ambiguous or when confidence is low.

Use this tool when:
- Instructions are unclear or could be interpreted multiple ways
- You have low confidence in your planned action (< 0.7)
- You need additional context that isn't available in the codebase
- There are multiple valid approaches and user preference is needed
- You encounter high-risk operations that need confirmation

The tool will pause agent execution and present your question to the user. The user's response will be added to the conversation for you to continue.

Example usage:
- "I found two different authentication systems. Which one should I modify?"
- "The requirement says 'improve performance' but doesn't specify metrics. What specific improvements are you looking for?"
- "This change will delete existing data. Should I proceed or create a backup first?"`
}

// Parameters returns the tool parameters schema
func (t *ClarificationTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "The specific question or clarification needed from the user",
				"minLength": 10,
				"maxLength": 1000
			},
			"context_summary": {
				"type": "string", 
				"description": "Brief summary of the current context that led to this question",
				"minLength": 5,
				"maxLength": 500
			},
			"confidence_level": {
				"type": "number",
				"description": "Your confidence level in proceeding without clarification (0.0-1.0)",
				"minimum": 0.0,
				"maximum": 1.0
			},
			"urgency": {
				"type": "string",
				"description": "Priority level for this clarification",
				"enum": ["low", "medium", "high", "critical"]
			},
			"suggested_options": {
				"type": "array",
				"description": "Suggested options or approaches for the user to choose from",
				"items": {
					"type": "string",
					"minLength": 5,
					"maxLength": 200
				},
				"maxItems": 5
			}
		},
		"required": ["question", "context_summary", "confidence_level"],
		"additionalProperties": false
	}`)
}

// Execute requests clarification from the user
func (t *ClarificationTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	// Parse parameters
	var request struct {
		Question         string   `json:"question"`
		ContextSummary   string   `json:"context_summary"`
		ConfidenceLevel  float64  `json:"confidence_level"`
		Urgency          string   `json:"urgency"`
		SuggestedOptions []string `json:"suggested_options"`
	}

	if err := json.Unmarshal(params, &request); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Validate parameters
	if request.Question == "" {
		return nil, fmt.Errorf("missing required parameter: question")
	}

	if request.ContextSummary == "" {
		return nil, fmt.Errorf("missing required parameter: context_summary")
	}

	if request.ConfidenceLevel < 0.0 || request.ConfidenceLevel > 1.0 {
		return nil, fmt.Errorf("confidence_level must be between 0.0 and 1.0")
	}

	// Set default urgency if not provided
	if request.Urgency == "" {
		request.Urgency = "medium"
	}

	// Validate urgency level
	validUrgencies := map[string]bool{
		"low": true, "medium": true, "high": true, "critical": true,
	}
	if !validUrgencies[request.Urgency] {
		return nil, fmt.Errorf("urgency must be one of: low, medium, high, critical")
	}

	// Format the clarification request
	clarificationMessage := formatClarificationRequest(request)

	// Return a special result that indicates clarification is needed
	// The orchestrator should detect this and pause for user input
	return &ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"clarification_needed": true,
			"question":             request.Question,
			"context_summary":      request.ContextSummary,
			"confidence_level":     request.ConfidenceLevel,
			"urgency":              request.Urgency,
			"suggested_options":    request.SuggestedOptions,
			"formatted_message":    clarificationMessage,
		},
	}, nil
}

// formatClarificationRequest formats the clarification request for display to the user
func formatClarificationRequest(request struct {
	Question         string   `json:"question"`
	ContextSummary   string   `json:"context_summary"`
	ConfidenceLevel  float64  `json:"confidence_level"`
	Urgency          string   `json:"urgency"`
	SuggestedOptions []string `json:"suggested_options"`
}) string {
	message := fmt.Sprintf(`🤔 CLARIFICATION NEEDED

CONTEXT: %s

QUESTION: %s

CONFIDENCE LEVEL: %.1f/1.0
URGENCY: %s`,
		request.ContextSummary,
		request.Question,
		request.ConfidenceLevel,
		request.Urgency)

	if len(request.SuggestedOptions) > 0 {
		message += "\n\nSUGGESTED OPTIONS:"
		for i, option := range request.SuggestedOptions {
			message += fmt.Sprintf("\n%d. %s", i+1, option)
		}
	}

	message += "\n\nPlease provide your guidance or choose from the options above."
	return message
}
