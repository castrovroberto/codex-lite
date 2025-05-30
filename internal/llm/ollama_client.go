package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/contextkeys"
)

// OllamaClient implements the Client interface for Ollama.
type OllamaClient struct {
	config config.OllamaConfig
}

// NewOllamaClient creates a new Ollama client with the provided configuration.
func NewOllamaClient(cfg config.OllamaConfig) *OllamaClient {
	return &OllamaClient{
		config: cfg,
	}
}

// OllamaRequest represents the request structure for Ollama's /api/generate and /api/chat endpoints.
// Note: This might need to be adjusted for chat vs. generate, especially with system prompts and tools.
type OllamaRequest struct {
	Model     string                   `json:"model"`
	Prompt    string                   `json:"prompt,omitempty"` // Used for /api/generate
	System    string                   `json:"system,omitempty"` // For system prompt
	Stream    bool                     `json:"stream"`
	KeepAlive string                   `json:"keep_alive,omitempty"`
	Tools     []map[string]interface{} `json:"tools,omitempty"` // Experimental: Ollama's tool support might require specific formatting or might not be standard via /api/generate.
	// Messages  []OllamaMessage `json:"messages,omitempty"` // Used for /api/chat
}

// OllamaMessage is used for the /api/chat endpoint if we choose to use it.
/* type OllamaMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
	// Images []string `json:"images,omitempty"` // For multimodal models
} */

// OllamaResponse represents a single (non-streaming) response from Ollama's /api/generate.
// Or a single chunk in a streaming response.
type OllamaResponse struct {
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	Response        string    `json:"response"` // The actual text response for /api/generate
	Done            bool      `json:"done"`
	Context         []int     `json:"context,omitempty"`
	TotalDuration   int64     `json:"total_duration,omitempty"`
	LoadDuration    int64     `json:"load_duration,omitempty"`
	PromptEvalCount int       `json:"prompt_eval_count,omitempty"`
	EvalCount       int       `json:"eval_count,omitempty"`
	EvalDuration    int64     `json:"eval_duration,omitempty"`
	// Message         OllamaMessage `json:"message,omitempty"` // Used for /api/chat responses
}

// OllamaErrorResponse represents an error response from Ollama.
type OllamaErrorResponse struct {
	Error string `json:"error"`
}

// Sentinel errors for specific Ollama client issues.
var (
	ErrOllamaHostUnreachable = errors.New("ollama: host unreachable or not responding")
	ErrOllamaModelNotFound   = errors.New("ollama: model not found by server")
	ErrOllamaInvalidResponse = errors.New("ollama: invalid or unexpected response from server")
)

// Generate performs a non-streaming generation request to Ollama.
func (oc *OllamaClient) Generate(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}) (string, error) {
	log := contextkeys.LoggerFromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/generate", strings.TrimRight(oc.config.HostURL, "/"))

	// Construct the prompt for Ollama. If a system prompt is provided, it's typically prepended.
	// Tools are not standard in Ollama's /api/generate in a structured way like OpenAI.
	// We might need to format tool descriptions into the prompt itself if needed for Ollama.
	// For now, systemPrompt is directly used if available in OllamaRequest.

	requestPayload := OllamaRequest{
		Model:     modelName,
		Prompt:    prompt,
		System:    systemPrompt, // Ollama's /api/generate supports a 'system' field
		Stream:    false,
		KeepAlive: oc.config.KeepAlive,
		// Tools: tools, // How tools are passed to Ollama's generate endpoint needs clarification. Might be part of prompt.
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		log.Error("Failed to marshal Ollama request body", "error", err)
		return "", fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	maxRetries := 2 // Example, could be from config
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			log.Info("Context canceled before Ollama request attempt", "attempt", i)
			return "", ctx.Err()
		default:
		}

		req, reqErr := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
		if reqErr != nil {
			log.Error("Failed to create HTTP request for Ollama", "error", reqErr)
			return "", fmt.Errorf("ollama: failed to create HTTP request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")

		log.Debug("Sending Ollama query", "url", apiURL, "model", modelName, "attempt", i+1)
		httpClient := &http.Client{Timeout: oc.config.RequestTimeout}
		resp, httpErr := httpClient.Do(req)
		lastErr = httpErr

		if httpErr != nil {
			log.Warn("Ollama request HTTP error", "attempt", i+1, "error", httpErr)
			var netErr net.Error
			if errors.As(httpErr, &netErr) && (netErr.Timeout() || !netErr.Temporary()) {
				log.Error("Ollama host likely unreachable or permanent network issue", "url", apiURL, "error", httpErr)
				return "", fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, httpErr)
			}
			if i == maxRetries {
				log.Error("Ollama request failed after all retries due to HTTP error", "error", httpErr)
				return "", fmt.Errorf("ollama: request failed after %d retries: %w", maxRetries+1, httpErr)
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		defer resp.Body.Close()
		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Error("Failed to read Ollama response body", "status", resp.StatusCode, "error", readErr)
			lastErr = fmt.Errorf("%w: failed to read response body: %v", ErrOllamaInvalidResponse, readErr)
			if i == maxRetries {
				return "", lastErr
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var ollamaResp OllamaResponse
			if err := json.Unmarshal(responseBodyBytes, &ollamaResp); err != nil {
				log.Error("Failed to unmarshal successful Ollama response", "status", resp.StatusCode, "body_snippet", string(responseBodyBytes[:min(len(responseBodyBytes), 200)]), "error", err)
				lastErr = fmt.Errorf("%w: failed to parse success response: %v", ErrOllamaInvalidResponse, err)
				if i == maxRetries {
					return "", lastErr
				}
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			log.Debug("Ollama query successful", "model_returned", ollamaResp.Model)
			return ollamaResp.Response, nil
		}

		log.Warn("Ollama API returned non-OK status", "status", resp.StatusCode, "body_snippet", string(responseBodyBytes[:min(len(responseBodyBytes), 200)]))
		var ollamaErrorResp OllamaErrorResponse
		if json.Unmarshal(responseBodyBytes, &ollamaErrorResp) == nil && ollamaErrorResp.Error != "" {
			errMsgLower := strings.ToLower(ollamaErrorResp.Error)
			if strings.Contains(errMsgLower, "model") && (strings.Contains(errMsgLower, "not found") || strings.Contains(errMsgLower, "does not exist")) {
				log.Error("Ollama model not found by server", "model_requested", modelName, "server_error", ollamaErrorResp.Error)
				return "", fmt.Errorf("%w: %s (model: %s)", ErrOllamaModelNotFound, ollamaErrorResp.Error, modelName)
			}
			lastErr = fmt.Errorf("ollama: API error - \"%s\" (HTTP %d)", strings.TrimSpace(ollamaErrorResp.Error), resp.StatusCode)
		} else {
			lastErr = fmt.Errorf("ollama: API returned status %d with unparsed error", resp.StatusCode)
		}

		if i == maxRetries {
			log.Error("Ollama request failed after all retries with non-OK status", "final_error", lastErr)
			return "", lastErr
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}

	if lastErr == nil {
		lastErr = errors.New("ollama: unknown error after retry loop")
	}
	return "", lastErr
}

// Stream performs a streaming generation request to Ollama.
func (oc *OllamaClient) Stream(ctx context.Context, modelName, prompt string, systemPrompt string, tools []map[string]interface{}, out chan<- string) error {
	defer close(out) // Ensure channel is closed when function exits
	log := contextkeys.LoggerFromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/generate", strings.TrimRight(oc.config.HostURL, "/"))

	requestPayload := OllamaRequest{
		Model:     modelName,
		Prompt:    prompt,
		System:    systemPrompt,
		Stream:    true,
		KeepAlive: oc.config.KeepAlive,
		// Tools: tools, // As with Generate, tool handling needs review for Ollama stream
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		log.Error("Failed to marshal Ollama streaming request body", "error", err)
		return fmt.Errorf("ollama stream: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		log.Error("Failed to create HTTP request for Ollama stream", "error", err)
		return fmt.Errorf("ollama stream: failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: oc.config.RequestTimeout} // Timeout for the entire stream might need adjustment
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Ollama streaming request failed", "error", err)
		// Check for host unreachable specifically
		var netErr net.Error
		if errors.As(err, &netErr) && (netErr.Timeout() || !netErr.Temporary()) {
			return fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, err)
		}
		return fmt.Errorf("ollama stream: request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Error("Ollama stream API returned non-OK status", "status", resp.StatusCode, "body", string(bodyBytes))
		var ollamaErrorResp OllamaErrorResponse
		if json.Unmarshal(bodyBytes, &ollamaErrorResp) == nil && ollamaErrorResp.Error != "" {
			if strings.Contains(strings.ToLower(ollamaErrorResp.Error), "model not found") {
				return fmt.Errorf("%w: %s (model: %s)", ErrOllamaModelNotFound, ollamaErrorResp.Error, modelName)
			}
			return fmt.Errorf("ollama stream: API error - \"%s\" (HTTP %d)", ollamaErrorResp.Error, resp.StatusCode)
		}
		return fmt.Errorf("ollama stream: API returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var ollamaResp OllamaResponse
		if err := decoder.Decode(&ollamaResp); err != nil {
			if errors.Is(err, io.EOF) {
				break // End of stream
			}
			log.Error("Failed to decode Ollama stream chunk", "error", err)
			return fmt.Errorf("ollama stream: failed to decode chunk: %w", err)
		}

		select {
		case out <- ollamaResp.Response:
		case <-ctx.Done():
			log.Info("Context canceled during Ollama stream processing")
			return ctx.Err()
		}

		if ollamaResp.Done {
			break
		}
	}

	log.Debug("Ollama stream completed successfully")
	return nil
}

// OllamaTag represents a single tag from Ollama's /api/tags response.
type OllamaTag struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
}

// OllamaTagsResponse represents the response from Ollama's /api/tags.
type OllamaTagsResponse struct {
	Models []OllamaTag `json:"models"`
}

// ListAvailableModels retrieves a list of available models from Ollama.
func (oc *OllamaClient) ListAvailableModels(ctx context.Context) ([]string, error) {
	log := contextkeys.LoggerFromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/tags", strings.TrimRight(oc.config.HostURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		log.Error("Failed to create HTTP request for Ollama /api/tags", "error", err)
		return nil, fmt.Errorf("ollama listmodels: failed to create request: %w", err)
	}

	httpClient := &http.Client{Timeout: oc.config.RequestTimeout} // Use a reasonable timeout
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Ollama /api/tags request failed", "error", err)
		var netErr net.Error
		if errors.As(err, &netErr) && (netErr.Timeout() || !netErr.Temporary()) {
			return nil, fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, err)
		}
		return nil, fmt.Errorf("ollama listmodels: request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Error("Ollama /api/tags returned non-OK status", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, fmt.Errorf("ollama listmodels: API returned status %d", resp.StatusCode)
	}

	var tagsResponse OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResponse); err != nil {
		log.Error("Failed to decode Ollama /api/tags response", "error", err)
		return nil, fmt.Errorf("%w: failed to parse /api/tags response: %v", ErrOllamaInvalidResponse, err)
	}

	modelNames := make([]string, len(tagsResponse.Models))
	for i, tag := range tagsResponse.Models {
		modelNames[i] = tag.Name
	}

	log.Debug("Successfully listed Ollama models", "count", len(modelNames))
	return modelNames, nil
}

// GenerateWithFunctions performs a generation request with function calling support for Ollama
func (oc *OllamaClient) GenerateWithFunctions(ctx context.Context, modelName, prompt string, systemPrompt string, tools []ToolDefinition) (*FunctionCallResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Ollama doesn't have native function calling, so we embed tool definitions in the prompt
	enhancedPrompt := prompt
	if len(tools) > 0 {
		enhancedPrompt = prompt + FormatToolCallForPrompt(tools)
	}

	// Use the existing Generate method
	response, err := oc.Generate(ctx, modelName, enhancedPrompt, systemPrompt, nil)
	if err != nil {
		return nil, err
	}

	// Parse the response to see if it's a function call or text
	functionCallResponse, parseErr := ParseFunctionCall(response)
	if parseErr != nil {
		log.Warn("Failed to parse function call response, treating as text", "error", parseErr)
		return &FunctionCallResponse{
			IsTextResponse: true,
			TextContent:    response,
		}, nil
	}

	return functionCallResponse, nil
}

// SupportsNativeFunctionCalling returns false for Ollama as it doesn't have native function calling
func (oc *OllamaClient) SupportsNativeFunctionCalling() bool {
	return false
}

// Embed generates embeddings for the given text using Ollama's embedding models
func (oc *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// Use Ollama's /api/embeddings endpoint
	apiURL := fmt.Sprintf("%s/api/embeddings", strings.TrimRight(oc.config.HostURL, "/"))

	// Default embedding model - could be made configurable
	embeddingModel := "nomic-embed-text"

	requestPayload := map[string]interface{}{
		"model":  embeddingModel,
		"prompt": text,
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		log.Error("Failed to marshal Ollama embedding request body", "error", err)
		return nil, fmt.Errorf("ollama embed: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		log.Error("Failed to create HTTP request for Ollama embeddings", "error", err)
		return nil, fmt.Errorf("ollama embed: failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: oc.config.RequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Ollama embedding request failed", "error", err)
		var netErr net.Error
		if errors.As(err, &netErr) && (netErr.Timeout() || !netErr.Temporary()) {
			return nil, fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, err)
		}
		return nil, fmt.Errorf("ollama embed: request error: %w", err)
	}
	defer resp.Body.Close()

	responseBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read Ollama embedding response body", "error", err)
		return nil, fmt.Errorf("ollama embed: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error("Ollama embedding API returned non-OK status", "status", resp.StatusCode, "body", string(responseBodyBytes))
		var ollamaErrorResp OllamaErrorResponse
		if json.Unmarshal(responseBodyBytes, &ollamaErrorResp) == nil && ollamaErrorResp.Error != "" {
			if strings.Contains(strings.ToLower(ollamaErrorResp.Error), "model not found") {
				return nil, fmt.Errorf("%w: %s (model: %s)", ErrOllamaModelNotFound, ollamaErrorResp.Error, embeddingModel)
			}
			return nil, fmt.Errorf("ollama embed: API error - \"%s\" (HTTP %d)", ollamaErrorResp.Error, resp.StatusCode)
		}
		return nil, fmt.Errorf("ollama embed: API returned status %d", resp.StatusCode)
	}

	// Parse the embedding response
	var embeddingResponse struct {
		Embedding []float64 `json:"embedding"`
	}

	if err := json.Unmarshal(responseBodyBytes, &embeddingResponse); err != nil {
		log.Error("Failed to unmarshal Ollama embedding response", "error", err)
		return nil, fmt.Errorf("ollama embed: failed to parse response: %w", err)
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(embeddingResponse.Embedding))
	for i, v := range embeddingResponse.Embedding {
		embedding[i] = float32(v)
	}

	log.Debug("Ollama embedding generated successfully", "dimension", len(embedding))
	return embedding, nil
}

// SupportsEmbeddings returns true as Ollama supports embedding models like nomic-embed-text
func (oc *OllamaClient) SupportsEmbeddings() bool {
	return true
}

// GenerateThought performs deliberation step for internal reasoning (fallback implementation)
func (oc *OllamaClient) GenerateThought(ctx context.Context, modelName, prompt, context string) (*ThoughtResponse, error) {
	log := contextkeys.LoggerFromContext(ctx)

	// For Ollama, we'll simulate deliberation using structured prompts
	thoughtPrompt := fmt.Sprintf(`
Think step by step about this situation. Provide your analysis in the following format:

CONTEXT: %s

SITUATION: %s

ANALYSIS:
- What are the key factors to consider?
- What are the potential risks?
- What are the potential benefits?
- What is your confidence level (0.0-1.0)?
- What action would you suggest?

Please structure your response clearly and provide a confidence score.`, context, prompt)

	response, err := oc.Generate(ctx, modelName, thoughtPrompt, "You are a careful, analytical assistant that thinks before acting.", nil)
	if err != nil {
		log.Error("Thought generation failed", "error", err)
		return nil, fmt.Errorf("ollama thought generation failed: %w", err)
	}

	// Parse the structured response
	thoughtResponse := &ThoughtResponse{
		ThoughtContent: response,
		Confidence:     0.5, // Default confidence
		ReasoningSteps: []string{},
	}

	// Try to extract confidence score from response
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), "confidence") {
			// Simple pattern matching for confidence scores
			for _, word := range strings.Fields(line) {
				if score, err := strconv.ParseFloat(word, 64); err == nil && score >= 0.0 && score <= 1.0 {
					thoughtResponse.Confidence = score
					break
				}
			}
		}
		if strings.HasPrefix(strings.ToLower(line), "- ") {
			thoughtResponse.ReasoningSteps = append(thoughtResponse.ReasoningSteps, strings.TrimPrefix(line, "- "))
		}
	}

	log.Debug("Generated thought with Ollama", "confidence", thoughtResponse.Confidence, "reasoning_steps", len(thoughtResponse.ReasoningSteps))
	return thoughtResponse, nil
}

// AssessConfidence evaluates confidence in a proposed action (fallback implementation)
func (oc *OllamaClient) AssessConfidence(ctx context.Context, modelName, thought, proposedAction string) (*ConfidenceAssessment, error) {
	log := contextkeys.LoggerFromContext(ctx)

	confidencePrompt := fmt.Sprintf(`
Assess the confidence in this proposed action based on the reasoning provided.

REASONING: %s

PROPOSED ACTION: %s

Please evaluate:
1. How confident are you in this action? (0.0 = no confidence, 1.0 = completely confident)
2. What are the main factors supporting this decision?
3. What are the main uncertainties or risks?
4. Would you recommend to: proceed, retry, or abort?

Provide your assessment with a confidence score between 0.0 and 1.0.`, thought, proposedAction)

	response, err := oc.Generate(ctx, modelName, confidencePrompt, "You are a careful evaluator that assesses the confidence and risks of proposed actions.", nil)
	if err != nil {
		log.Error("Confidence assessment failed", "error", err)
		return nil, fmt.Errorf("ollama confidence assessment failed: %w", err)
	}

	// Parse the confidence assessment
	assessment := &ConfidenceAssessment{
		Score:          0.5, // Default
		Factors:        make(map[string]float64),
		Uncertainties:  []string{},
		Recommendation: "proceed", // Default
		Metadata:       map[string]interface{}{"raw_response": response},
	}

	// Simple parsing for confidence score and recommendation
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for confidence scores
		if strings.Contains(strings.ToLower(line), "confidence") {
			for _, word := range strings.Fields(line) {
				if score, err := strconv.ParseFloat(word, 64); err == nil && score >= 0.0 && score <= 1.0 {
					assessment.Score = score
					break
				}
			}
		}

		// Look for recommendations
		lower := strings.ToLower(line)
		if strings.Contains(lower, "recommend") {
			if strings.Contains(lower, "abort") {
				assessment.Recommendation = "abort"
			} else if strings.Contains(lower, "retry") {
				assessment.Recommendation = "retry"
			} else if strings.Contains(lower, "proceed") {
				assessment.Recommendation = "proceed"
			}
		}

		// Extract uncertainties
		if strings.Contains(strings.ToLower(line), "uncertainty") || strings.Contains(strings.ToLower(line), "risk") {
			if strings.HasPrefix(line, "- ") {
				assessment.Uncertainties = append(assessment.Uncertainties, strings.TrimPrefix(line, "- "))
			}
		}
	}

	log.Debug("Generated confidence assessment with Ollama", "score", assessment.Score, "recommendation", assessment.Recommendation)
	return assessment, nil
}

// SupportsDeliberation returns false for Ollama as it doesn't have native deliberation support
// but we provide fallback implementations
func (oc *OllamaClient) SupportsDeliberation() bool {
	return false // Using fallback implementations
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
