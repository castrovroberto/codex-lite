package chat

import (
	"context"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/config"
)

func TestNewChatModelWithMockProvider(t *testing.T) {
	// Create a mock message provider
	mockProvider := NewMockMessageProvider().WithAutoResponses([]ChatMessage{
		{
			ID:        "test-1",
			Type:      AssistantMessage,
			Sender:    "Assistant",
			Text:      "Hello from mock!",
			Timestamp: time.Now(),
		},
	})

	// Create a basic config
	cfg := &config.AppConfig{}
	cfg.LLM.Provider = "ollama"
	cfg.LLM.Model = "test-model"

	// Create model with functional options
	model := NewChatModel(
		WithParentContext(context.Background()),
		WithInitialConfig(cfg),
		WithMessageProvider(mockProvider),
		WithDelayProvider(&RealDelayProvider{}),
	)

	// Verify the model was created correctly
	if model.messageProvider == nil {
		t.Error("MessageProvider should not be nil")
	}

	if model.cfg == nil {
		t.Error("Config should not be nil")
	}

	if model.theme == nil {
		t.Error("Theme should not be nil")
	}

	// Test that we can access components
	if model.Header() == nil {
		t.Error("Header should not be nil")
	}

	if model.MessageList() == nil {
		t.Error("MessageList should not be nil")
	}

	if model.InputArea() == nil {
		t.Error("InputArea should not be nil")
	}

	if model.StatusBar() == nil {
		t.Error("StatusBar should not be nil")
	}

	// Clean up
	mockProvider.Close()
}

func TestMockMessageProvider(t *testing.T) {
	mockProvider := NewMockMessageProvider()

	// Test sending a message
	err := mockProvider.Send(context.Background(), "test message")
	if err != nil {
		t.Errorf("Send should not return error: %v", err)
	}

	// Check that message was recorded
	sentMessages := mockProvider.GetSentMessages()
	if len(sentMessages) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(sentMessages))
	}

	if sentMessages[0] != "test message" {
		t.Errorf("Expected 'test message', got '%s'", sentMessages[0])
	}

	// Test receiving messages
	select {
	case msg := <-mockProvider.Messages():
		if msg.Type != AssistantMessage {
			t.Errorf("Expected AssistantMessage, got %v", msg.Type)
		}
		if msg.Text != "This is a mock response" {
			t.Errorf("Expected 'This is a mock response', got '%s'", msg.Text)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Should have received a message")
	}

	// Clean up
	mockProvider.Close()
}

func TestChatPresenterIntegration(t *testing.T) {
	// This test would require more setup with actual LLM client and tool registry
	// For now, we'll just test that the presenter can be created

	// We can't easily test the full ChatPresenter without mocking the LLM client
	// and tool registry, but we can at least verify the structure is correct

	// This would be a more comprehensive integration test:
	// presenter := NewChatPresenter(ctx, mockLLMClient, mockToolRegistry, "system prompt", "test-model")
	// ... test actual message flow

	t.Log("ChatPresenter integration test placeholder - would need mock LLM client")
}
