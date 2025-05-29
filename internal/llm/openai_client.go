package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/contextkeys"
)

// OpenAIClient implements the Client interface for OpenAI API
type OpenAIClient struct {
	config config.OpenAIConfig
}

// NewOpenAIClient creates a new OpenAI client with the provided configuration
func NewOpenAIClient(cfg config.OpenAIConfig) *OpenAIClient {
	return &OpenAIClient{
		config: cfg,
	}
}

// OpenAI API request/response structures
type OpenAIMessage struct {
	Role         string              `json:"role"`
	Content      string              `json:"content,omitempty"`
	ToolCalls    []OpenAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string              `json:"tool_call_id,omitempty"`
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
	Name         string              `json:"name,omitempty"`
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAITool struct {
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type OpenAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Generate performs a non-streaming generation request to OpenAI
func (oc *OpenAIClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	messages := []OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	if systemPrompt != "" {
		messages = append([]OpenAIMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	request := OpenAIRequest{
		Model:    modelName,
		Messages: messages,
	}

	if len(tools) > 0 {
		openaiTools := make([]OpenAITool, len(tools))
		for i, tool := range tools {
			openaiTools[i] = OpenAITool{
				Type:     "function",
				Function: tool,
			}
		}
		request.Tools = openaiTools
	}

	response, err := oc.makeRequest(ctx, request)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}

// GenerateWithFunctions performs a generation request with function calling support for OpenAI
func (oc *OpenAIClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []ToolDefinition) (*FunctionCallResponse, error) {
	messages := []OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	if systemPrompt != "" {
		messages = append([]OpenAIMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	request := OpenAIRequest{
		Model:    modelName,
		Messages: messages,
	}

	if len(tools) > 0 {
		openaiTools := make([]OpenAITool, len(tools))
		for i, tool := range tools {
			var functionDef map[string]interface{}
			if err := json.Unmarshal(tool.Function.Parameters, &functionDef); err != nil {
				return nil, fmt.Errorf("failed to parse tool parameters: %w", err)
			}

			openaiTools[i] = OpenAITool{
				Type: "function",
				Function: map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  functionDef,
				},
			}
		}
		request.Tools = openaiTools
	}

	response, err := oc.makeRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices in response")
	}

	choice := response.Choices[0]

	// Check if there are tool calls
	if len(choice.Message.ToolCalls) > 0 {
		// Return the first tool call (for simplicity, we handle one at a time)
		toolCall := choice.Message.ToolCalls[0]
		return &FunctionCallResponse{
			IsTextResponse: false,
			FunctionCall: &FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: json.RawMessage(toolCall.Function.Arguments),
				ID:        toolCall.ID,
			},
		}, nil
	}

	// No tool calls, return as text response
	return &FunctionCallResponse{
		IsTextResponse: true,
		TextContent:    choice.Message.Content,
	}, nil
}

// Stream performs a streaming generation request to OpenAI
func (oc *OpenAIClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	defer close(out)

	messages := []OpenAIMessage{
		{Role: "user", Content: prompt},
	}

	if systemPrompt != "" {
		messages = append([]OpenAIMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	request := OpenAIRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   true,
	}

	if len(tools) > 0 {
		openaiTools := make([]OpenAITool, len(tools))
		for i, tool := range tools {
			openaiTools[i] = OpenAITool{
				Type:     "function",
				Function: tool,
			}
		}
		request.Tools = openaiTools
	}

	return oc.makeStreamRequest(ctx, request, out)
}

// ListAvailableModels retrieves a list of available models from OpenAI
func (oc *OpenAIClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", oc.config.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: oc.config.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("openai: failed to decode response: %w", err)
	}

	models := make([]string, len(modelsResp.Data))
	for i, model := range modelsResp.Data {
		models[i] = model.ID
	}

	return models, nil
}

// SupportsNativeFunctionCalling returns true for OpenAI as it has native function calling
func (oc *OpenAIClient) SupportsNativeFunctionCalling() bool {
	return true
}

// Embed generates embeddings for the given text using OpenAI's embedding models
func (oc *OpenAIClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// Default embedding model - could be made configurable
	embeddingModel := "text-embedding-ada-002"

	requestPayload := map[string]interface{}{
		"model": embeddingModel,
		"input": text,
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("openai embed: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oc.config.BaseURL+"/embeddings", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("openai embed: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: oc.config.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed: request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai embed: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp OpenAIErrorResponse
		if json.Unmarshal(responseBody, &errorResp) == nil {
			return nil, fmt.Errorf("openai embed: API error - %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("openai embed: API returned status %d", resp.StatusCode)
	}

	// Parse the embedding response
	var embeddingResponse struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.Unmarshal(responseBody, &embeddingResponse); err != nil {
		return nil, fmt.Errorf("openai embed: failed to parse response: %w", err)
	}

	if len(embeddingResponse.Data) == 0 {
		return nil, fmt.Errorf("openai embed: no embedding data in response")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(embeddingResponse.Data[0].Embedding))
	for i, v := range embeddingResponse.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// SupportsEmbeddings returns true as OpenAI supports embedding models
func (oc *OpenAIClient) SupportsEmbeddings() bool {
	return true
}

// GenerateThought performs deliberation step for internal reasoning using OpenAI
func (oc *OpenAIClient) GenerateThought(ctx context.Context, modelName, prompt, context string) (*ThoughtResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Create a structured prompt for thought generation
	thoughtPrompt := fmt.Sprintf(`
You are an AI assistant that thinks carefully before acting. Please analyze the following situation step by step.

CONTEXT: %s

CURRENT SITUATION: %s

Please provide your analysis in the following structured format:

THOUGHT PROCESS:
1. Key factors to consider:
2. Potential risks:
3. Potential benefits:
4. Confidence level (0.0-1.0):
5. Suggested action:
6. Areas of uncertainty:

Be thorough in your analysis and provide a confidence score between 0.0 and 1.0.`, context, prompt)

	messages := []OpenAIMessage{
		{Role: "system", Content: "You are a careful, analytical assistant that provides structured reasoning before taking action."},
		{Role: "user", Content: thoughtPrompt},
	}

	request := OpenAIRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: 0.3, // Lower temperature for more consistent reasoning
		MaxTokens:   1000,
	}

	response, err := oc.makeRequest(ctx, request)
	if err != nil {
		log.Error("Thought generation failed", "error", err)
		return nil, fmt.Errorf("openai thought generation failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("openai thought generation: no response choices")
	}

	content := response.Choices[0].Message.Content

	// Parse the structured response
	thoughtResponse := &ThoughtResponse{
		ThoughtContent: content,
		Confidence:     0.5, // Default confidence
		ReasoningSteps: []string{},
	}

	// Extract structured information from the response
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract confidence score
		if strings.Contains(strings.ToLower(line), "confidence") {
			for _, word := range strings.Fields(line) {
				if score, err := strconv.ParseFloat(word, 64); err == nil && score >= 0.0 && score <= 1.0 {
					thoughtResponse.Confidence = score
					break
				}
			}
		}

		// Extract reasoning steps
		if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") ||
			strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "4.") ||
			strings.HasPrefix(line, "5.") || strings.HasPrefix(line, "6.") {
			thoughtResponse.ReasoningSteps = append(thoughtResponse.ReasoningSteps, line)
		}

		// Extract suggested action
		if strings.Contains(strings.ToLower(line), "suggested action") {
			thoughtResponse.SuggestedAction = line
		}

		// Extract uncertainty
		if strings.Contains(strings.ToLower(line), "uncertainty") {
			thoughtResponse.Uncertainty = line
		}
	}

	log.Debug("Generated thought with OpenAI", "confidence", thoughtResponse.Confidence, "reasoning_steps", len(thoughtResponse.ReasoningSteps))
	return thoughtResponse, nil
}

// AssessConfidence evaluates confidence in a proposed action using OpenAI
func (oc *OpenAIClient) AssessConfidence(ctx context.Context, modelName, thought, proposedAction string) (*ConfidenceAssessment, error) {
	log := contextkeys.LoggerFromContext(ctx)

	confidencePrompt := fmt.Sprintf(`
You are an expert evaluator assessing the confidence and risks of proposed actions.

PREVIOUS REASONING: %s

PROPOSED ACTION: %s

Please provide a detailed confidence assessment in the following format:

CONFIDENCE ASSESSMENT:
1. Overall confidence score (0.0-1.0):
2. Supporting factors:
3. Risk factors:
4. Uncertainties:
5. Recommendation (proceed/retry/abort):
6. Rationale for recommendation:

Be precise with your confidence score and provide clear reasoning.`, thought, proposedAction)

	messages := []OpenAIMessage{
		{Role: "system", Content: "You are a careful evaluator that provides detailed confidence assessments for proposed actions."},
		{Role: "user", Content: confidencePrompt},
	}

	request := OpenAIRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: 0.2, // Very low temperature for consistent evaluation
		MaxTokens:   800,
	}

	response, err := oc.makeRequest(ctx, request)
	if err != nil {
		log.Error("Confidence assessment failed", "error", err)
		return nil, fmt.Errorf("openai confidence assessment failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("openai confidence assessment: no response choices")
	}

	content := response.Choices[0].Message.Content

	// Parse the confidence assessment
	assessment := &ConfidenceAssessment{
		Score:          0.5, // Default
		Factors:        make(map[string]float64),
		Uncertainties:  []string{},
		Recommendation: "proceed", // Default
		Metadata:       map[string]interface{}{"raw_response": content},
	}

	// Parse the structured response
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract confidence score
		if strings.Contains(strings.ToLower(line), "confidence score") {
			for _, word := range strings.Fields(line) {
				if score, err := strconv.ParseFloat(word, 64); err == nil && score >= 0.0 && score <= 1.0 {
					assessment.Score = score
					break
				}
			}
		}

		// Extract recommendation
		lower := strings.ToLower(line)
		if strings.Contains(lower, "recommendation") {
			if strings.Contains(lower, "abort") {
				assessment.Recommendation = "abort"
			} else if strings.Contains(lower, "retry") {
				assessment.Recommendation = "retry"
			} else if strings.Contains(lower, "proceed") {
				assessment.Recommendation = "proceed"
			}
		}

		// Extract uncertainties
		if strings.Contains(strings.ToLower(line), "uncertainties") ||
			strings.Contains(strings.ToLower(line), "risk factors") {
			if strings.HasPrefix(line, "- ") {
				assessment.Uncertainties = append(assessment.Uncertainties, strings.TrimPrefix(line, "- "))
			}
		}
	}

	log.Debug("Generated confidence assessment with OpenAI", "score", assessment.Score, "recommendation", assessment.Recommendation)
	return assessment, nil
}

// SupportsDeliberation returns true for OpenAI as it can handle structured reasoning well
func (oc *OpenAIClient) SupportsDeliberation() bool {
	return true
}

// makeRequest makes a non-streaming request to OpenAI API
func (oc *OpenAIClient) makeRequest(ctx context.Context, request OpenAIRequest) (*OpenAIResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oc.config.BaseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Sending OpenAI request", "model", request.Model)

	client := &http.Client{Timeout: oc.config.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp OpenAIErrorResponse
		if json.Unmarshal(responseBody, &errorResp) == nil {
			return nil, fmt.Errorf("openai: API error - %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("openai: API returned status %d", resp.StatusCode)
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(responseBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("openai: failed to parse response: %w", err)
	}

	log.Debug("OpenAI request successful", "model", openaiResp.Model)
	return &openaiResp, nil
}

// makeStreamRequest makes a streaming request to OpenAI API
func (oc *OpenAIClient) makeStreamRequest(ctx context.Context, request OpenAIRequest, out chan<- string) error {
	log := contextkeys.LoggerFromContext(ctx)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oc.config.BaseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Sending OpenAI streaming request", "model", request.Model)

	client := &http.Client{Timeout: oc.config.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var errorResp OpenAIErrorResponse
		if json.Unmarshal(bodyBytes, &errorResp) == nil {
			return fmt.Errorf("openai: API error - %s", errorResp.Error.Message)
		}
		return fmt.Errorf("openai: API returned status %d", resp.StatusCode)
	}

	// Parse Server-Sent Events
	scanner := NewSSEScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			select {
			case out <- chunk.Choices[0].Delta.Content:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openai: stream error: %w", err)
	}

	log.Debug("OpenAI stream completed successfully")
	return nil
}

// SSEScanner is a simple scanner for Server-Sent Events
type SSEScanner struct {
	reader io.Reader
	buffer []byte
	pos    int
	err    error
}

func NewSSEScanner(r io.Reader) *SSEScanner {
	return &SSEScanner{reader: r}
}

func (s *SSEScanner) Scan() bool {
	if s.err != nil {
		return false
	}

	// Read more data if needed
	if s.pos >= len(s.buffer) {
		buf := make([]byte, 4096)
		n, err := s.reader.Read(buf)
		if err != nil {
			s.err = err
			return false
		}
		s.buffer = buf[:n]
		s.pos = 0
	}

	// Find next line
	for s.pos < len(s.buffer) && s.buffer[s.pos] != '\n' {
		s.pos++
	}

	if s.pos < len(s.buffer) {
		s.pos++ // Skip the newline
		return true
	}

	return false
}

func (s *SSEScanner) Text() string {
	if s.pos > 0 {
		end := s.pos - 1
		if end > 0 && s.buffer[end-1] == '\r' {
			end--
		}
		return string(s.buffer[:end])
	}
	return ""
}

func (s *SSEScanner) Err() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}
