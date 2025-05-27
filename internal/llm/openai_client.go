package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/contextkeys"
)

// OpenAIClient implements the Client interface for OpenAI API
type OpenAIClient struct {
	apiKey  string
	baseURL string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
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
	req, err := http.NewRequestWithContext(ctx, "GET", oc.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
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

// makeRequest makes a non-streaming request to OpenAI API
func (oc *OpenAIClient) makeRequest(ctx context.Context, request OpenAIRequest) (*OpenAIResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oc.baseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Sending OpenAI request", "model", request.Model)

	client := &http.Client{Timeout: 60 * time.Second}
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

	req, err := http.NewRequestWithContext(ctx, "POST", oc.baseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("openai: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", "application/json")

	log.Debug("Sending OpenAI streaming request", "model", request.Model)

	client := &http.Client{Timeout: 120 * time.Second}
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
