package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	// slog is not directly used here if we rely on context logger, but good for reference
	// "log/slog"

	"github.com/castrovroberto/codex-lite/internal/contextkeys" // For retrieving logger from context
)

// Request structure for Ollama API
type Request struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"` // Set to false for single response
	// TODO: Add other options like System, Template, Context, Options from AppConfig
	KeepAlive string `json:"keep_alive,omitempty"` // From AppConfig
}

// Response structure for Ollama API (non-streaming)
type Response struct {
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	Response        string    `json:"response"`
	Done            bool      `json:"done"`
	Context         []int     `json:"context,omitempty"`          // If we want to support conversation context
	TotalDuration   int64     `json:"total_duration,omitempty"`   // Nanoseconds
	LoadDuration    int64     `json:"load_duration,omitempty"`    // Nanoseconds
	PromptEvalCount int       `json:"prompt_eval_count,omitempty"`
	EvalCount       int       `json:"eval_count,omitempty"`
	EvalDuration    int64     `json:"eval_duration,omitempty"`    // Nanoseconds
}

// ErrorResponse structure for Ollama API errors
type ErrorResponse struct {
	Error string `json:"error"`
}

// Define sentinel errors for specific Ollama client issues.
var (
	ErrOllamaHostUnreachable = errors.New("ollama: host unreachable or not responding")
	ErrOllamaModelNotFound   = errors.New("ollama: model not found by server")
	ErrOllamaInvalidResponse = errors.New("ollama: invalid or unexpected response from server")
)

// Query sends a query request to the Ollama API and returns the response text.
// It includes basic retry logic and respects the context for cancellation and values (like logger).
func Query(ctx context.Context, ollamaHostURL, modelName, prompt string) (string, error) {
	log := contextkeys.LoggerFromContext(ctx) // Retrieve logger from context
	appCfg := contextkeys.ConfigFromContext(ctx) // Retrieve config for KeepAlive and other potential options

	apiURL := fmt.Sprintf("%s/api/generate", strings.TrimRight(ollamaHostURL, "/"))

	requestPayload := Request{
		Model:     modelName,
		Prompt:    prompt,
		Stream:    false, // For simple synchronous response
		KeepAlive: appCfg.OllamaKeepAlive,
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		log.Error("Failed to marshal Ollama request body", "error", err)
		return "", fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	// Retry loop (initial attempt + retryCount from config, or fixed)
	// For simplicity, using a fixed retry count here. Could come from appCfg.
	maxRetries := 2 // Example: 1 initial attempt + 2 retries = 3 total
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			log.Info("Context cancelled before Ollama request attempt", "attempt", i)
			return "", ctx.Err()
		default:
		}

		req, reqErr := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
		if reqErr != nil {
			log.Error("Failed to create HTTP request for Ollama", "error", reqErr)
			return "", fmt.Errorf("ollama: failed to create HTTP request: %w", reqErr) // Non-retryable client-side error
		}
		req.Header.Set("Content-Type", "application/json")

		log.Debug("Sending Ollama query", "url", apiURL, "model", modelName, "attempt", i+1)
		client := &http.Client{} // Consider creating a shared client if performance is critical
		resp, httpErr := client.Do(req)
		lastErr = httpErr // Store the latest error

		if httpErr != nil {
			log.Warn("Ollama request HTTP error", "attempt", i+1, "error", httpErr)
			var netErr net.Error
			if errors.As(httpErr, &netErr) && (netErr.Timeout() || !netErr.Temporary()) {
				// Non-temporary network error or timeout, potentially host unreachable
				log.Error("Ollama host likely unreachable or permanent network issue", "url", apiURL, "error", httpErr)
				return "", fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, httpErr) // Return sentinel error
			}
			// For temporary errors, continue to retry logic (if retries left)
			if i == maxRetries {
				log.Error("Ollama request failed after all retries due to HTTP error", "error", httpErr)
				return "", fmt.Errorf("ollama: request failed after %d retries: %w", maxRetries+1, httpErr)
			}
			// Wait before retrying
			time.Sleep(time.Second * time.Duration(i+1)) // Simple exponential backoff
			continue                                      // Next retry iteration
		}

		// Process response
		defer resp.Body.Close()
		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Error("Failed to read Ollama response body", "status", resp.StatusCode, "error", readErr)
			lastErr = fmt.Errorf("%w: failed to read response body: %v", ErrOllamaInvalidResponse, readErr)
			// This is likely a server-side issue or network interruption during read.
			// If retries are left, we could retry, but for now, let's assume it's a fatal error for this attempt.
			if i == maxRetries {
				return "", lastErr
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var ollamaResp Response
			if err := json.Unmarshal(responseBodyBytes, &ollamaResp); err != nil {
				log.Error("Failed to unmarshal successful Ollama response", "status", resp.StatusCode, "body_snippet", string(responseBodyBytes[:min(len(responseBodyBytes), 200)]), "error", err)
				lastErr = fmt.Errorf("%w: failed to parse success response: %v", ErrOllamaInvalidResponse, err)
				// Treat as a server error for retry purposes
				if i == maxRetries { return "", lastErr }
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			log.Debug("Ollama query successful", "model_returned", ollamaResp.Model, "duration_ns", ollamaResp.TotalDuration)
			return ollamaResp.Response, nil // Success
		}

		// Handle non-OK status codes
		log.Warn("Ollama API returned non-OK status", "status", resp.StatusCode, "body_snippet", string(responseBodyBytes[:min(len(responseBodyBytes), 200)]))
		var ollamaErrorResponse ErrorResponse
		if json.Unmarshal(responseBodyBytes, &ollamaErrorResponse) == nil && ollamaErrorResponse.Error != "" {
			errMsgLower := strings.ToLower(ollamaErrorResponse.Error)
			if strings.Contains(errMsgLower, "model") && (strings.Contains(errMsgLower, "not found") || strings.Contains(errMsgLower, "does not exist")) {
				log.Error("Ollama model not found by server", "model_requested", modelName, "server_error", ollamaErrorResponse.Error)
				return "", fmt.Errorf("%w: %s (model: %s)", ErrOllamaModelNotFound, ollamaErrorResponse.Error, modelName) // Non-retryable specific error
			}
			// Other parsed API errors
			lastErr = fmt.Errorf("ollama: API error - \"%s\" (HTTP %d)", strings.TrimSpace(ollamaErrorResponse.Error), resp.StatusCode)
		} else {
			// Unparsed error or empty error message
			lastErr = fmt.Errorf("ollama: API returned status %d with unparsed error", resp.StatusCode)
		}

		if i == maxRetries {
			log.Error("Ollama request failed after all retries with non-OK status", "final_error", lastErr)
			return "", lastErr
		}
		time.Sleep(time.Second * time.Duration(i+1)) // Wait before retry
	}

	// Should not be reached if loop logic is correct, but as a fallback:
	if lastErr == nil {
		lastErr = errors.New("ollama: unknown error after retry loop")
	}
	return "", lastErr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}