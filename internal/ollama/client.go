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

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/logger"
)

const (
	// maxRetries is the number of retries for Ollama requests, excluding the initial attempt.
	// So, total attempts = 1 (initial) + maxRetries.
	maxRetries = 2 // Results in up to 3 total attempts
)

// Sentinel errors for common Ollama client issues.
var (
	// ErrOllamaHostUnreachable indicates the Ollama host could not be reached or did not respond.
	ErrOllamaHostUnreachable = errors.New("ollama: host unreachable or not responding")
	// ErrOllamaModelNotFound indicates the specified model was not found by the Ollama server.
	ErrOllamaModelNotFound = errors.New("ollama: model not found by the server")
	// ErrOllamaInvalidResponse indicates an invalid, unexpected, or unparseable response from the Ollama server.
	ErrOllamaInvalidResponse = errors.New("ollama: invalid or unexpected response from server")
	// ErrOllamaBadRequest indicates a client-side error in the request construction (e.g., bad JSON).
	ErrOllamaBadRequest = errors.New("ollama: bad request to server")
)

// QueryRequest represents the structure for the Ollama API request body.
type QueryRequest struct {
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	Stream    bool   `json:"stream"` // Hardcoded to false for now
	KeepAlive string `json:"keep_alive,omitempty"`
}

// QueryResponse is a simplified structure for the non-streaming Ollama API response.
// We are primarily interested in the 'response' field for non-streaming.
type QueryResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"` // This holds the actual text response
	Done      bool      `json:"done"`
	// Other fields like context, total_duration, etc., are ignored for now.
}

// ErrorResponse represents the structure of an error response from the Ollama API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Query sends a query request to the Ollama API and returns the response text.
// It includes retry logic for transient network errors and specific server errors.
func Query(ctx context.Context, ollamaHostURL, modelName, prompt string) (string, error) {
	appCfg := config.GetConfig() // Get current app configuration

	apiURL := strings.TrimSuffix(ollamaHostURL, "/") + "/api/generate"

	queryReq := QueryRequest{
		Model:     modelName,
		Prompt:    prompt,
		Stream:    false, // Not supporting streaming in this simplified client
		KeepAlive: appCfg.OllamaKeepAlive,
	}

	requestBodyBytes, err := json.Marshal(queryReq)
	if err != nil {
		return "", fmt.Errorf("%w: failed to marshal request body: %v", ErrOllamaBadRequest, err)
	}

	var finalErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Prepare request for this attempt
		reqCtx := ctx // Use the original context for timeout across retries
		if attempt > 0 {
			// Apply backoff delay for retries
			backoffDuration := time.Duration(attempt*2) * time.Second // Simple exponential backoff (2s, 4s, ...)
			logger.Get().Debug("Retrying Ollama request", "attempt", attempt, "model", modelName, "backoff", backoffDuration)
			select {
			case <-time.After(backoffDuration):
				// Continue to retry
			case <-ctx.Done():
				return "", fmt.Errorf("ollama query context cancelled during retry backoff: %w", ctx.Err())
			}
		}

		// Create a new request for each attempt to ensure headers and body are fresh,
		// especially if the context has a timeout that might have been hit by the http.Client
		// on a previous attempt.
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, bytes.NewBuffer(requestBodyBytes))
		if err != nil {
			// This is a client-side error before the request is even sent. Unlikely to succeed on retry.
			finalErr = fmt.Errorf("%w: failed to create HTTP request: %v", ErrOllamaBadRequest, err)
			break // Don't retry if request creation fails
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		httpClient := &http.Client{} // Consider making this configurable or reusing it if safe
		resp, err := httpClient.Do(req)

		if err != nil {
			finalErr = err // Store the error to be potentially wrapped later
			var netErr net.Error
			if errors.As(err, &netErr) {
				// This is a network error (e.g., DNS, connection refused, timeout during connection)
				logger.Get().Warn("Ollama request network error", "attempt", attempt, "model", modelName, "error", err)
				finalErr = fmt.Errorf("%w: %v", ErrOllamaHostUnreachable, err) // Wrap as specific error
				if netErr.Timeout() || !netErr.Temporary() {
					// If it's a timeout or not a temporary error, don't retry further for this type.
					return "", finalErr
				}
				// For temporary network errors, continue to the next retry iteration.
				continue
			}
			// For non-network errors from client.Do, it's less clear if retrying helps.
			// We'll let it retry for now, but it might be better to break for some.
			logger.Get().Warn("Ollama request client.Do error", "attempt", attempt, "model", modelName, "error", err)
			finalErr = fmt.Errorf("ollama: http client error: %w", err) // Generic wrapper
			continue                                                   // Retry for now
		}

		// We have a response, process it.
		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() // Close body immediately after reading

		if readErr != nil {
			logger.Get().Error("Failed to read Ollama response body", "attempt", attempt, "model", modelName, "status_code", resp.StatusCode, "error", readErr)
			finalErr = fmt.Errorf("%w: failed to read response body (HTTP %d): %v", ErrOllamaInvalidResponse, resp.StatusCode, readErr)
			// This is likely a persistent issue with the response or our handling, retry might not help.
			if resp.StatusCode == http.StatusOK { // If status was OK but body read failed
				return "", finalErr // Don't retry if status was OK
			}
			continue // Retry if status was not OK and body read failed
		}

		if resp.StatusCode == http.StatusOK {
			var ollamaResp QueryResponse
			if err := json.Unmarshal(responseBodyBytes, &ollamaResp); err != nil {
				logger.Get().Error("Failed to unmarshal Ollama success response", "attempt", attempt, "model", modelName, "error", err, "body", string(responseBodyBytes))
				return "", fmt.Errorf("%w: failed to unmarshal successful response JSON: %v", ErrOllamaInvalidResponse, err)
			}
			if !ollamaResp.Done {
				logger.Get().Warn("Ollama response 'done' field is false in non-streaming mode", "model", modelName, "response_body", string(responseBodyBytes))
				// Proceeding anyway, as 'response' field might still be useful.
			}
			return strings.TrimSpace(ollamaResp.Response), nil // SUCCESS
		}

		// Handle non-OK HTTP status codes
		logger.Get().Warn("Ollama API returned non-OK status", "attempt", attempt, "model", modelName, "status_code", resp.StatusCode, "body", string(responseBodyBytes))
		var errorResp ErrorResponse
		if json.Unmarshal(responseBodyBytes, &errorResp) == nil && errorResp.Error != "" {
			// We have a structured error message from Ollama
			errMsgLower := strings.ToLower(errorResp.Error)
			if strings.Contains(errMsgLower, "model") && (strings.Contains(errMsgLower, "not found") || strings.Contains(errMsgLower, "unknown")) {
				finalErr = fmt.Errorf("%w: %s (HTTP %d)", ErrOllamaModelNotFound, strings.TrimSpace(errorResp.Error), resp.StatusCode)
				return "", finalErr // Model not found, no point retrying
			}
			// For other structured API errors
			finalErr = fmt.Errorf("%w: %s (HTTP %d)", ErrOllamaInvalidResponse, strings.TrimSpace(errorResp.Error), resp.StatusCode)
		} else {
			// Unstructured error or failed to parse error body
			finalErr = fmt.Errorf("%w: received HTTP status %d from server. Body: %s", ErrOllamaInvalidResponse, resp.StatusCode, string(responseBodyBytes))
		}

		// Decide whether to retry based on status code for non-OK responses
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
			// Don't retry on these client-side or definitive errors.
			return "", finalErr
		}
		// For other server-side errors (5xx), the loop will continue to retry if attempts remain.
	}

	// If loop finishes, all retries have been exhausted
	if finalErr != nil {
		return "", fmt.Errorf("ollama: query failed after %d attempts: %w", maxRetries+1, finalErr)
	}

	// Should ideally not be reached if logic is correct, implies an unhandled case.
	return "", fmt.Errorf("ollama: unexpected termination of query logic after all retries")
}