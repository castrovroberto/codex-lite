package chat

import (
	"context"
	"sync"
	"time"
)

// MockMessageProvider implements MessageProvider for testing
type MockMessageProvider struct {
	sentMessages  []string
	messagesChan  chan ChatMessage
	closed        bool
	mu            sync.Mutex
	autoResponses []ChatMessage // Predefined responses to send back
	responseIndex int           // Index of next response to send
	sendDelay     time.Duration // Delay before sending response
}

// NewMockMessageProvider creates a new mock message provider
func NewMockMessageProvider() *MockMessageProvider {
	return &MockMessageProvider{
		sentMessages: make([]string, 0),
		messagesChan: make(chan ChatMessage, 10),
		autoResponses: []ChatMessage{
			{
				ID:        "mock-1",
				Type:      AssistantMessage,
				Sender:    "Assistant",
				Text:      "This is a mock response",
				Timestamp: time.Now(),
			},
		},
	}
}

// WithAutoResponses sets predefined responses for the mock
func (m *MockMessageProvider) WithAutoResponses(responses []ChatMessage) *MockMessageProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoResponses = responses
	m.responseIndex = 0
	return m
}

// WithSendDelay sets a delay before responses are sent
func (m *MockMessageProvider) WithSendDelay(delay time.Duration) *MockMessageProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendDelay = delay
	return m
}

// Send implements MessageProvider.Send
func (m *MockMessageProvider) Send(ctx context.Context, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return context.Canceled
	}

	// Record the sent message
	m.sentMessages = append(m.sentMessages, prompt)

	// Send an auto response if configured
	if len(m.autoResponses) > 0 {
		response := m.autoResponses[m.responseIndex%len(m.autoResponses)]
		m.responseIndex++

		// Send response asynchronously with optional delay
		go func() {
			if m.sendDelay > 0 {
				time.Sleep(m.sendDelay)
			}

			m.mu.Lock()
			if !m.closed {
				select {
				case m.messagesChan <- response:
				default:
					// Channel full, ignore
				}
			}
			m.mu.Unlock()
		}()
	}

	return nil
}

// Messages implements MessageProvider.Messages
func (m *MockMessageProvider) Messages() <-chan ChatMessage {
	return m.messagesChan
}

// Close implements MessageProvider.Close
func (m *MockMessageProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		close(m.messagesChan)
	}
	return nil
}

// GetSentMessages returns all messages that were sent through this provider
func (m *MockMessageProvider) GetSentMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to avoid race conditions
	result := make([]string, len(m.sentMessages))
	copy(result, m.sentMessages)
	return result
}

// SendMessage manually sends a message through the provider (for testing)
func (m *MockMessageProvider) SendMessage(msg ChatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		select {
		case m.messagesChan <- msg:
		default:
			// Channel full, ignore
		}
	}
}
