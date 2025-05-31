package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/castrovroberto/CGE/internal/config"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements the Client interface for Google Gemini API
type GeminiClient struct {
	config config.GeminiConfig
	client *genai.Client
}

// NewGeminiClient creates a new Gemini client with the provided configuration
func NewGeminiClient(cfg config.GeminiConfig) *GeminiClient {
	return &GeminiClient{
		config: cfg,
	}
}

// initClient initializes the Gemini client if not already initialized
func (gc *GeminiClient) initClient(ctx context.Context) error {
	if gc.client != nil {
		return nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(gc.config.APIKey))
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}
	gc.client = client
	return nil
}

// Generate performs a non-streaming generation request to Gemini
func (gc *GeminiClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	if err := gc.initClient(ctx); err != nil {
		return "", err
	}

	model := gc.client.GenerativeModel(modelName)

	// Configure model parameters
	if gc.config.MaxTokens > 0 {
		model.SetMaxOutputTokens(int32(gc.config.MaxTokens))
	}
	if gc.config.Temperature >= 0 {
		model.SetTemperature(float32(gc.config.Temperature))
	}

	// Build the conversation parts
	var parts []genai.Part

	// Add system prompt if provided
	if systemPrompt != "" {
		parts = append(parts, genai.Text("System: "+systemPrompt))
	}

	// Add the main prompt
	parts = append(parts, genai.Text(prompt))

	// Add tool definitions if provided
	if len(tools) > 0 {
		toolPrompt := gc.formatToolsForPrompt(tools)
		parts = append(parts, genai.Text(toolPrompt))
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", fmt.Errorf("gemini generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("gemini: no content in response")
	}

	return gc.extractTextFromContent(resp.Candidates[0].Content), nil
}

// GenerateWithFunctions performs a generation request with function calling support for Gemini
func (gc *GeminiClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []ToolDefinition) (*FunctionCallResponse, error) {
	if err := gc.initClient(ctx); err != nil {
		return nil, err
	}

	model := gc.client.GenerativeModel(modelName)

	// Configure model parameters
	if gc.config.MaxTokens > 0 {
		model.SetMaxOutputTokens(int32(gc.config.MaxTokens))
	}
	if gc.config.Temperature >= 0 {
		model.SetTemperature(float32(gc.config.Temperature))
	}

	// Configure function calling if tools are provided
	if len(tools) > 0 {
		geminiTools := gc.convertToGeminiTools(tools)
		model.Tools = geminiTools
	}

	// Build the conversation parts
	var parts []genai.Part

	// Add system prompt if provided
	if systemPrompt != "" {
		parts = append(parts, genai.Text("System: "+systemPrompt))
	}

	// Add the main prompt
	parts = append(parts, genai.Text(prompt))

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return nil, fmt.Errorf("gemini generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("gemini: no content in response")
	}

	candidate := resp.Candidates[0]

	// Check if there are function calls
	for _, part := range candidate.Content.Parts {
		if funcCall, ok := part.(*genai.FunctionCall); ok {
			// Convert function call arguments to JSON
			args, err := json.Marshal(funcCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function call arguments: %w", err)
			}

			return &FunctionCallResponse{
				IsTextResponse: false,
				FunctionCall: &FunctionCall{
					Name:      funcCall.Name,
					Arguments: args,
				},
			}, nil
		}
	}

	// No function calls, return as text response
	return &FunctionCallResponse{
		IsTextResponse: true,
		TextContent:    gc.extractTextFromContent(candidate.Content),
	}, nil
}

// Stream performs a streaming generation request to Gemini
func (gc *GeminiClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	defer close(out)

	if err := gc.initClient(ctx); err != nil {
		return err
	}

	model := gc.client.GenerativeModel(modelName)

	// Configure model parameters
	if gc.config.MaxTokens > 0 {
		model.SetMaxOutputTokens(int32(gc.config.MaxTokens))
	}
	if gc.config.Temperature >= 0 {
		model.SetTemperature(float32(gc.config.Temperature))
	}

	// Build the conversation parts
	var parts []genai.Part

	// Add system prompt if provided
	if systemPrompt != "" {
		parts = append(parts, genai.Text("System: "+systemPrompt))
	}

	// Add the main prompt
	parts = append(parts, genai.Text(prompt))

	// Add tool definitions if provided
	if len(tools) > 0 {
		toolPrompt := gc.formatToolsForPrompt(tools)
		parts = append(parts, genai.Text(toolPrompt))
	}

	iter := model.GenerateContentStream(ctx, parts...)

	for {
		resp, err := iter.Next()
		if err != nil {
			if err.Error() == "iterator done" {
				break
			}
			return fmt.Errorf("gemini streaming failed: %w", err)
		}

		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			text := gc.extractTextFromContent(resp.Candidates[0].Content)
			if text != "" {
				select {
				case out <- text:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	return nil
}

// ListAvailableModels retrieves a list of models available through Gemini
func (gc *GeminiClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	if err := gc.initClient(ctx); err != nil {
		return nil, err
	}

	iter := gc.client.ListModels(ctx)
	var models []string

	for {
		model, err := iter.Next()
		if err != nil {
			if err.Error() == "iterator done" {
				break
			}
			return nil, fmt.Errorf("failed to list Gemini models: %w", err)
		}
		models = append(models, model.Name)
	}

	return models, nil
}

// SupportsNativeFunctionCalling returns true since Gemini supports native function calling
func (gc *GeminiClient) SupportsNativeFunctionCalling() bool {
	return true
}

// Embed generates embeddings for the given text using Gemini
func (gc *GeminiClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := gc.initClient(ctx); err != nil {
		return nil, err
	}

	// Use the embedding model
	embeddingModel := gc.client.EmbeddingModel("text-embedding-004")

	res, err := embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("gemini embedding failed: %w", err)
	}

	return res.Embedding.Values, nil
}

// SupportsEmbeddings returns true since Gemini supports text embeddings
func (gc *GeminiClient) SupportsEmbeddings() bool {
	return true
}

// GenerateThought performs a deliberation step using Gemini
func (gc *GeminiClient) GenerateThought(ctx context.Context, modelName, prompt, context string) (*ThoughtResponse, error) {
	thoughtPrompt := fmt.Sprintf(`
Please think through this situation carefully and provide a structured response.

Context: %s

Request: %s

Please respond with your thoughts in the following JSON format:
{
  "thought_content": "your detailed reasoning here",
  "confidence": 0.85,
  "reasoning_steps": ["step 1", "step 2", "step 3"],
  "suggested_action": "what you recommend doing",
  "uncertainty": "areas where you're uncertain"
}`, context, prompt)

	response, err := gc.Generate(ctx, modelName, thoughtPrompt, "You are a thoughtful AI assistant that provides structured reasoning.", nil)
	if err != nil {
		return nil, err
	}

	var thought ThoughtResponse
	if err := json.Unmarshal([]byte(response), &thought); err != nil {
		// If JSON parsing fails, create a basic response
		return &ThoughtResponse{
			ThoughtContent:  response,
			Confidence:      0.5,
			ReasoningSteps:  []string{"Analyzed the request"},
			SuggestedAction: "Proceed with caution",
			Uncertainty:     "Unable to parse structured response",
		}, nil
	}

	return &thought, nil
}

// AssessConfidence evaluates confidence in a proposed action using Gemini
func (gc *GeminiClient) AssessConfidence(ctx context.Context, modelName, thought, proposedAction string) (*ConfidenceAssessment, error) {
	confidencePrompt := fmt.Sprintf(`
Please assess the confidence level for the following proposed action based on the reasoning provided.

Reasoning: %s

Proposed Action: %s

Please respond with a confidence assessment in the following JSON format:
{
  "score": 0.85,
  "factors": {
    "clarity": 0.9,
    "completeness": 0.8,
    "risk_level": 0.7
  },
  "uncertainties": ["potential issue 1", "potential issue 2"],
  "recommendation": "proceed",
  "metadata": {
    "analysis_time": "2024-01-01T10:00:00Z",
    "complexity": "medium"
  }
}

Score should be between 0.0 and 1.0. Recommendation should be "proceed", "retry", or "abort".`, thought, proposedAction)

	response, err := gc.Generate(ctx, modelName, confidencePrompt, "You are an expert at assessing confidence and risk in decision-making.", nil)
	if err != nil {
		return nil, err
	}

	var assessment ConfidenceAssessment
	if err := json.Unmarshal([]byte(response), &assessment); err != nil {
		// If JSON parsing fails, create a basic response
		return &ConfidenceAssessment{
			Score:          0.5,
			Factors:        map[string]float64{"analysis": 0.5},
			Uncertainties:  []string{"Unable to parse structured response"},
			Recommendation: "retry",
			Metadata:       map[string]interface{}{"error": "json_parse_failed"},
		}, nil
	}

	return &assessment, nil
}

// SupportsDeliberation returns true since we've implemented deliberation methods
func (gc *GeminiClient) SupportsDeliberation() bool {
	return true
}

// Helper methods

// extractTextFromContent extracts text content from Gemini response
func (gc *GeminiClient) extractTextFromContent(content *genai.Content) string {
	var textParts []string
	for _, part := range content.Parts {
		if text, ok := part.(genai.Text); ok {
			textParts = append(textParts, string(text))
		}
	}
	return strings.Join(textParts, "")
}

// formatToolsForPrompt formats tools for inclusion in prompts (fallback when native function calling isn't used)
func (gc *GeminiClient) formatToolsForPrompt(tools []map[string]interface{}) string {
	if len(tools) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n\nAvailable tools:\n")

	for _, tool := range tools {
		if function, ok := tool["function"].(map[string]interface{}); ok {
			if name, ok := function["name"].(string); ok {
				if description, ok := function["description"].(string); ok {
					builder.WriteString(fmt.Sprintf("- %s: %s\n", name, description))
					if params, ok := function["parameters"]; ok {
						if paramsBytes, err := json.Marshal(params); err == nil {
							builder.WriteString(fmt.Sprintf("  Parameters: %s\n", string(paramsBytes)))
						}
					}
				}
			}
		}
	}

	builder.WriteString("\nTo use a tool, respond with JSON in this format:\n")
	builder.WriteString(`{"name": "tool_name", "arguments": {"param1": "value1", "param2": "value2"}}`)
	builder.WriteString("\n\nIf you don't need to use a tool, respond normally with text.\n")

	return builder.String()
}

// convertToGeminiTools converts ToolDefinition to Gemini function declarations
func (gc *GeminiClient) convertToGeminiTools(tools []ToolDefinition) []*genai.Tool {
	var geminiTools []*genai.Tool

	for _, tool := range tools {
		// Parse the parameters schema
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			continue // Skip malformed tools
		}

		// Convert the schema to Gemini's Schema format
		geminiSchema := gc.convertToGeminiSchema(schema)

		// Create function declaration
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  geminiSchema,
		}

		// Create tool with function declaration
		geminiTool := &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl},
		}

		geminiTools = append(geminiTools, geminiTool)
	}

	return geminiTools
}

// convertToGeminiSchema converts a JSON schema map to Gemini's Schema format
func (gc *GeminiClient) convertToGeminiSchema(schema map[string]interface{}) *genai.Schema {
	geminiSchema := &genai.Schema{}

	// Handle type
	if typeVal, ok := schema["type"].(string); ok {
		switch typeVal {
		case "object":
			geminiSchema.Type = genai.TypeObject
		case "array":
			geminiSchema.Type = genai.TypeArray
		case "string":
			geminiSchema.Type = genai.TypeString
		case "number":
			geminiSchema.Type = genai.TypeNumber
		case "integer":
			geminiSchema.Type = genai.TypeInteger
		case "boolean":
			geminiSchema.Type = genai.TypeBoolean
		}
	}

	// Handle description
	if desc, ok := schema["description"].(string); ok {
		geminiSchema.Description = desc
	}

	// Handle properties for object types
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		geminiSchema.Properties = make(map[string]*genai.Schema)
		for key, propSchema := range properties {
			if propMap, ok := propSchema.(map[string]interface{}); ok {
				geminiSchema.Properties[key] = gc.convertToGeminiSchema(propMap)
			}
		}
	}

	// Handle items for array types
	if items, ok := schema["items"].(map[string]interface{}); ok {
		geminiSchema.Items = gc.convertToGeminiSchema(items)
	}

	// Handle required fields
	if required, ok := schema["required"].([]interface{}); ok {
		var requiredStrings []string
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				requiredStrings = append(requiredStrings, reqStr)
			}
		}
		geminiSchema.Required = requiredStrings
	}

	// Handle enum
	if enum, ok := schema["enum"].([]interface{}); ok {
		var enumStrings []string
		for _, enumVal := range enum {
			if enumStr, ok := enumVal.(string); ok {
				enumStrings = append(enumStrings, enumStr)
			}
		}
		geminiSchema.Enum = enumStrings
	}

	return geminiSchema
}
