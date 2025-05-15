// internal/ollama/client.go
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io" // Added for io.ReadAll and io.NopCloser
	"net/http"
	"strings"
	"time"

	"github.com/castrovroberto/codex-lite/internal/logger"
)

type Request struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type Response struct {
	Response string `json:"response"`
}

// Query sends a request to the specified Ollama host.
// It now accepts a context.Context for cancellation and timeouts.
func Query(ctx context.Context, hostURL, model, prompt string) (string, error) {
	requestBody := Request{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	initialBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("ollama: failed to marshal request body: %w", err)
	}

	if !strings.HasPrefix(hostURL, "http://") && !strings.HasPrefix(hostURL, "https://") {
		hostURL = "http://" + hostURL
	}
	requestURL := strings.TrimSuffix(hostURL, "/") + "/api/generate"

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(initialBodyBytes))
	if err != nil {
		return "", fmt.Errorf("ollama: failed to create initial request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	var (
		resp       *http.Response
		retryCount = 2
		retryDelay = 1 * time.Second
	)

	for i := 0; i <= retryCount; i++ {
		// If this is a retry, the request body needs to be fresh.
		// For the first attempt (i=0), req already has the body.
		// For subsequent attempts (i>0), we use a new buffer for the body.
		currentReq := req
		if i > 0 { // This is a retry
			// Clone the original request for retries to avoid modifying the original one unintentionally.
			clonedReq := req.Clone(ctx)
			// Provide a new io.ReadCloser for the body, as the previous one would have been consumed.
			clonedReq.Body = io.NopCloser(bytes.NewBuffer(initialBodyBytes)) // FIX for issue 1
			currentReq = clonedReq
		}
		
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("ollama: context cancelled before attempt %d: %w", i+1, ctx.Err())
		default:
		}

		resp, err = client.Do(currentReq)

		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			break // Success
		}

		// If error or not a success status, decide whether to retry
		if err != nil || (resp != nil && shouldRetry(resp.StatusCode)) {
			var statusCode int
			if resp != nil {
				statusCode = resp.StatusCode
			}
			logger.Get().Warn("Ollama request failed, considering retry...",
				"attempt", i+1,
				"max_attempts", retryCount+1,
				"url", requestURL,
				"error", err,
				"status_code", statusCode)

			if i < retryCount {
				// Close response body from failed attempt before sleeping/retrying
				if resp != nil {
					resp.Body.Close()
				}
				select {
				case <-time.After(retryDelay):
					retryDelay *= 2 // Exponential backoff
					continue      // Go to next retry iteration
				case <-ctx.Done():
					return "", fmt.Errorf("ollama: context cancelled during retry wait: %w", ctx.Err())
				}
			}
		}
		// If not retryable or max retries reached, break loop to handle error outside
		break
	}

	if err != nil { // Covers network errors or errors from client.Do itself
		if resp != nil { // If there was a response object despite the error
			resp.Body.Close()
		}
		return "", fmt.Errorf("ollama: request failed after all retries: %w", err)
	}

	if resp == nil { // Should not happen if err is nil, but as a safeguard
		return "", fmt.Errorf("ollama: no response received after retries (unexpected)")
	}
	defer resp.Body.Close() // Ensure body is closed on all paths hereafter

	if resp.StatusCode != http.StatusOK {
		// Read the entire body for error reporting, as it can only be read once.
		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("ollama: request failed with status %d and could not read error body: %w", resp.StatusCode, readErr)
		}

		var ollamaError struct {
			Error string `json:"error"`
		}
		// Attempt to parse Ollama's specific JSON error from the buffered body
		if jsonErr := json.NewDecoder(bytes.NewReader(responseBodyBytes)).Decode(&ollamaError); jsonErr == nil && ollamaError.Error != "" {
			return "", fmt.Errorf("ollama: API error - \"%s\" (HTTP %d)", strings.TrimSpace(ollamaError.Error), resp.StatusCode)
		}

		// Fallback: Generic HTTP error, use the read body if available
		if len(responseBodyBytes) > 0 {
			return "", fmt.Errorf("ollama: request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBodyBytes)))
		} else {
			return "", fmt.Errorf("ollama: request failed with status %d (empty error body)", resp.StatusCode)
		}
	}

	// Success path: resp.StatusCode == http.StatusOK
	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama: failed to decode successful response body: %w", err)
	}
	return strings.TrimSpace(result.Response), nil
}

func shouldRetry(status int) bool {
	return status == http.StatusServiceUnavailable || // 503
		status == http.StatusGatewayTimeout || // 504
		status == http.StatusInternalServerError || // 500
		status == http.StatusBadGateway // 502
}