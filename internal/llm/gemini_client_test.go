package llm

import (
	"testing"

	"github.com/castrovroberto/CGE/internal/config"
)

func TestGeminiClientCreation(t *testing.T) {
	cfg := config.GeminiConfig{
		APIKey:            "test-api-key",
		RequestTimeout:    300,
		MaxTokens:         4096,
		RequestsPerMinute: 20,
		Temperature:       0.7,
	}

	client := NewGeminiClient(cfg)
	if client == nil {
		t.Fatal("Expected non-nil Gemini client")
	}

	// Verify it implements the Client interface
	var _ Client = client

	// Verify configuration is stored correctly
	if client.config.APIKey != cfg.APIKey {
		t.Errorf("Expected API key %s, got %s", cfg.APIKey, client.config.APIKey)
	}

	if client.config.Temperature != cfg.Temperature {
		t.Errorf("Expected temperature %f, got %f", cfg.Temperature, client.config.Temperature)
	}

	if client.config.MaxTokens != cfg.MaxTokens {
		t.Errorf("Expected max tokens %d, got %d", cfg.MaxTokens, client.config.MaxTokens)
	}
}

func TestGeminiClientCapabilities(t *testing.T) {
	cfg := config.GeminiConfig{
		APIKey:      "test-api-key",
		Temperature: 0.7,
	}

	client := NewGeminiClient(cfg)

	// Test capability methods
	if !client.SupportsNativeFunctionCalling() {
		t.Error("Expected Gemini client to support native function calling")
	}

	if !client.SupportsEmbeddings() {
		t.Error("Expected Gemini client to support embeddings")
	}

	if !client.SupportsDeliberation() {
		t.Error("Expected Gemini client to support deliberation")
	}
}
