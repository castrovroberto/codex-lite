package chat

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ChatService interface abstracts the LLM/agent interaction logic
type ChatService interface {
	// SendMessage sends a message to the LLM and returns a command that will produce a response
	SendMessage(ctx context.Context, prompt string) tea.Cmd
}

// DelayProvider interface allows injecting different delay mechanisms for testing
type DelayProvider interface {
	// Delay returns a command that waits for the specified duration
	Delay(duration time.Duration) tea.Cmd
}

// HistoryService interface abstracts chat history persistence
type HistoryService interface {
	// SaveHistory saves the current chat state
	SaveHistory(sessionID, modelName string, messages []chatMessage, startTime time.Time) error
	// LoadHistory loads chat history by session ID
	LoadHistory(sessionID string) (*ChatHistory, error)
}

// Real implementations

// RealChatService provides actual LLM integration
type RealChatService struct {
	// Add fields for actual LLM client when implemented
}

// SendMessage implements ChatService interface
func (r *RealChatService) SendMessage(ctx context.Context, prompt string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Replace with actual LLM call
		// For now, simulate response
		time.Sleep(1 * time.Second)
		return ollamaSuccessResponseMsg{
			response: "This is a simulated response from the LLM.",
			duration: 1 * time.Second,
		}
	}
}

// RealDelayProvider provides actual time delays
type RealDelayProvider struct{}

// Delay implements DelayProvider interface
func (r *RealDelayProvider) Delay(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return delayCompleteMsg{}
	})
}

// Mock implementations for testing

// MockChatService provides controllable responses for testing
type MockChatService struct {
	Responses []MockResponse
	CallCount int
}

// MockResponse represents a predefined response for testing
type MockResponse struct {
	Response string
	Duration time.Duration
	Error    error
}

// SendMessage implements ChatService interface for testing
func (m *MockChatService) SendMessage(ctx context.Context, prompt string) tea.Cmd {
	return func() tea.Msg {
		if m.CallCount >= len(m.Responses) {
			return ollamaErrorMsg(fmt.Errorf("no more mock responses"))
		}

		resp := m.Responses[m.CallCount]
		m.CallCount++

		if resp.Error != nil {
			return ollamaErrorMsg(resp.Error)
		}

		return ollamaSuccessResponseMsg{
			response: resp.Response,
			duration: resp.Duration,
		}
	}
}

// MockDelayProvider provides controllable delays for testing
type MockDelayProvider struct {
	Delays []time.Duration
}

// Delay implements DelayProvider interface for testing
func (m *MockDelayProvider) Delay(duration time.Duration) tea.Cmd {
	// For testing, we can return immediately or track the delay
	return func() tea.Msg {
		return delayCompleteMsg{}
	}
}

// New message type for delay completion
type delayCompleteMsg struct{}
