// internal/ollama/client.go
package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
func Query(hostURL, model, prompt string) (string, error) {
	requestBody := Request{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("ollama: failed to marshal request: %w", err)
	}

	// Ensure hostURL has a scheme and ends with /api/generate
	if !strings.HasPrefix(hostURL, "http://") && !strings.HasPrefix(hostURL, "https://") {
		hostURL = "http://" + hostURL // Default to http if no scheme
	}
	url := strings.TrimSuffix(hostURL, "/") + "/api/generate"

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("ollama: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Attempt to read error body if any for more context
		var errorBody bytes.Buffer
		_, _ = errorBody.ReadFrom(resp.Body) // Ignore error from reading error body itself
		return "", fmt.Errorf("ollama: received non-OK HTTP status %d: %s", resp.StatusCode, errorBody.String())
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama: failed to decode response: %w", err)
	}

	return result.Response, nil
}