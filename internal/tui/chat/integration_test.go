package chat

import (
	"context"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModelWithMockDependencies(t *testing.T) {
	t.Run("successful_message_flow", func(t *testing.T) {
		// Create model with mock dependencies using new constructor
		cfg := &config.AppConfig{}
		ctx := context.Background()

		// Create a mock message provider that mimics the old chat service behavior
		mockProvider := NewMockMessageProvider().WithAutoResponses([]ChatMessage{
			{
				ID:        "mock-1",
				Type:      AssistantMessage,
				Sender:    "Assistant",
				Text:      "Mock response from LLM",
				Timestamp: time.Now(),
			},
		})

		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Simulate user typing and sending a message
		model.inputArea.SetValue("test message")

		// Process enter key to send message
		enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, cmd := model.Update(enterMsg)
		model = updatedModel.(Model)

		// Verify model state after sending
		assert.True(t, model.loading, "Model should be in loading state")
		assert.Equal(t, "", model.inputArea.GetValue(), "Input should be cleared")

		// Verify message was added to message list
		messages := model.messageList.GetMessages()
		assert.Greater(t, len(messages), 2, "Should have welcome message, user message, and placeholder")

		// Find the user message (second-to-last, since placeholder is last)
		userMessage := messages[len(messages)-2]
		assert.Equal(t, "test message", userMessage.text, "User message should be added")
		assert.Equal(t, "You", userMessage.sender, "Sender should be 'You'")

		// Verify placeholder message was added
		placeholderMessage := messages[len(messages)-1]
		assert.Equal(t, "Thinking...", placeholderMessage.text, "Placeholder should be added")
		assert.Equal(t, "Assistant", placeholderMessage.sender, "Placeholder sender should be 'Assistant'")
		assert.True(t, placeholderMessage.placeholder, "Message should be marked as placeholder")

		// Verify command was produced
		assert.NotNil(t, cmd, "Enter should produce command")

		// Simulate receiving a message from the provider
		chatMsg := chatMsgWrapper{
			ChatMessage: ChatMessage{
				ID:        "test-1",
				Type:      AssistantMessage,
				Sender:    "Assistant",
				Text:      "Mock response from LLM",
				Timestamp: time.Now(),
			},
		}
		updatedModel, _ = model.Update(chatMsg)
		model = updatedModel.(Model)

		// Verify model state after response
		assert.False(t, model.loading, "Model should not be in loading state")

		// Verify response message was added
		messages = model.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.Equal(t, "Mock response from LLM", lastMessage.text, "Response message should be added")
		assert.Equal(t, "Assistant", lastMessage.sender, "Sender should be 'Assistant'")
		assert.True(t, lastMessage.isMarkdown, "Response should be markdown")

		// Clean up
		mockProvider.Close()
	})

	t.Run("error_message_flow", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Send a message
		model.inputArea.SetValue("test message")
		enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := model.Update(enterMsg)
		model = updatedModel.(Model)

		// Simulate an error message from the provider
		errorMsg := chatMsgWrapper{
			ChatMessage: ChatMessage{
				ID:        "error-1",
				Type:      ErrorMessage,
				Sender:    "System",
				Text:      "Test error message",
				Timestamp: time.Now(),
			},
		}
		updatedModel, _ = model.Update(errorMsg)
		model = updatedModel.(Model)

		// Verify error handling
		assert.False(t, model.loading, "Model should not be in loading state after error")

		// Verify error message was added
		messages := model.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.Contains(t, lastMessage.text, "Test error message", "Error message should be added")
		assert.Equal(t, "System", lastMessage.sender, "Error sender should be 'System'")

		// Clean up
		mockProvider.Close()
	})

	t.Run("suggestion_workflow", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Simulate typing slash command
		model.inputArea.SetValue("/h")
		model.inputArea.UpdateSuggestions("/h")

		// Verify suggestions appear
		assert.True(t, model.inputArea.HasSuggestions(), "Should have suggestions for /h")

		// Simulate down arrow navigation
		downMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, cmd := model.Update(downMsg)
		model = updatedModel.(Model)

		// Should handle navigation
		assert.Nil(t, cmd, "Navigation should not produce external commands")

		// Simulate tab to apply suggestion
		tabMsg := tea.KeyMsg{Type: tea.KeyTab}
		updatedModel, _ = model.Update(tabMsg)
		model = updatedModel.(Model)

		// Should apply suggestion and clear suggestion box
		assert.False(t, model.inputArea.HasSuggestions(), "Suggestions should be cleared after applying")
		suggestedValue := model.inputArea.GetValue()
		assert.Contains(t, suggestedValue, "/help", "Should contain help command")

		// Clean up
		mockProvider.Close()
	})

	t.Run("window_resize_with_suggestions", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Setup suggestions
		model.inputArea.SetValue("/h")
		model.inputArea.UpdateSuggestions("/h")
		assert.True(t, model.inputArea.HasSuggestions(), "Should have suggestions")

		// Simulate window resize
		resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
		updatedModel, _ := model.Update(resizeMsg)
		model = updatedModel.(Model)

		// Suggestions should persist after resize
		assert.True(t, model.inputArea.HasSuggestions(), "Suggestions should persist after resize")

		// Viewport height should be recalculated
		assert.GreaterOrEqual(t, model.messageList.GetHeight(), model.layout.GetMinViewportHeight(),
			"Viewport height should be at least minimum after resize")

		// Clean up
		mockProvider.Close()
	})
}

func TestModelToolCallIntegration(t *testing.T) {
	t.Run("complete_tool_call_lifecycle", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Start a tool call
		startMsg := toolStartMsg{
			toolCallID: "test-tool-123",
			toolName:   "test_search",
			params:     map[string]interface{}{"query": "test"},
		}
		updatedModel, _ := model.Update(startMsg)
		model = updatedModel.(Model)

		// Verify tool call started
		assert.Len(t, model.activeToolCalls, 1, "Should have one active tool call")
		assert.Contains(t, model.activeToolCalls, "test-tool-123", "Should contain the specific tool call")

		// Send progress update
		progressMsg := toolProgressMsg{
			toolCallID: "test-tool-123",
			toolName:   "test_search",
			progress:   0.5,
			status:     "Searching...",
			step:       2,
			totalSteps: 4,
		}
		updatedModel, _ = model.Update(progressMsg)
		model = updatedModel.(Model)

		// Verify progress updated
		state := model.activeToolCalls["test-tool-123"]
		assert.Equal(t, 0.5, state.progress, "Progress should be updated")
		assert.Equal(t, "Searching...", state.status, "Status should be updated")

		// Complete the tool call
		completeMsg := toolCompleteMsg{
			toolCallID: "test-tool-123",
			toolName:   "test_search",
			success:    true,
			result:     "Search completed successfully",
			duration:   time.Second * 2,
			error:      "",
		}
		updatedModel, _ = model.Update(completeMsg)
		model = updatedModel.(Model)

		// Verify tool call completed
		assert.Len(t, model.activeToolCalls, 0, "Should have no active tool calls")

		// Verify result message added
		messages := model.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.True(t, lastMessage.isToolResult, "Last message should be tool result")
		assert.True(t, lastMessage.toolSuccess, "Tool should be marked successful")
		assert.Equal(t, "Search completed successfully", lastMessage.text, "Result text should match")

		// Clean up
		mockProvider.Close()
	})

	t.Run("concurrent_tool_calls", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Start multiple tool calls
		startMsg1 := toolStartMsg{toolCallID: "tool-1", toolName: "search", params: map[string]interface{}{}}
		startMsg2 := toolStartMsg{toolCallID: "tool-2", toolName: "analyze", params: map[string]interface{}{}}

		updatedModel, _ := model.Update(startMsg1)
		model = updatedModel.(Model)
		updatedModel, _ = model.Update(startMsg2)
		model = updatedModel.(Model)

		// Verify both tools are active
		assert.Len(t, model.activeToolCalls, 2, "Should have two active tool calls")

		// Complete one tool
		completeMsg1 := toolCompleteMsg{
			toolCallID: "tool-1",
			toolName:   "search",
			success:    true,
			result:     "Search done",
			duration:   time.Second,
		}
		updatedModel, _ = model.Update(completeMsg1)
		model = updatedModel.(Model)

		// Verify only one tool remains
		assert.Len(t, model.activeToolCalls, 1, "Should have one active tool call")
		assert.Contains(t, model.activeToolCalls, "tool-2", "Second tool should still be active")
		assert.NotContains(t, model.activeToolCalls, "tool-1", "First tool should be completed")

		// Clean up
		mockProvider.Close()
	})
}

func TestModelLayoutCalculations(t *testing.T) {
	t.Run("viewport_height_recalculation", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Test different window sizes
		testSizes := []struct {
			width, height int
			description   string
		}{
			{80, 24, "small_terminal"},
			{120, 40, "medium_terminal"},
			{200, 60, "large_terminal"},
		}

		for _, size := range testSizes {
			t.Run(size.description, func(t *testing.T) {
				resizeMsg := tea.WindowSizeMsg{Width: size.width, Height: size.height}
				updatedModel, _ := model.Update(resizeMsg)
				model = updatedModel.(Model)

				// Verify viewport height is reasonable
				viewportHeight := model.messageList.GetHeight()
				assert.GreaterOrEqual(t, viewportHeight, model.layout.GetMinViewportHeight(),
					"Viewport should be at least minimum height")
				assert.LessOrEqual(t, viewportHeight, size.height,
					"Viewport should not exceed window height")
			})
		}

		// Clean up
		mockProvider.Close()
	})

	t.Run("suggestions_affect_viewport_height", func(t *testing.T) {
		// Create model with mock dependencies
		cfg := &config.AppConfig{}
		ctx := context.Background()

		mockProvider := NewMockMessageProvider()
		model := NewChatModel(
			WithParentContext(ctx),
			WithInitialConfig(cfg),
			WithMessageProvider(mockProvider),
			WithDelayProvider(&MockDelayProvider{}),
		)

		// Set initial window size
		resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
		updatedModel, _ := model.Update(resizeMsg)
		model = updatedModel.(Model)

		heightWithoutSuggestions := model.messageList.GetHeight()

		// Add suggestions
		model.inputArea.SetValue("/h")
		model.inputArea.UpdateSuggestions("/h")

		// Recalculate with suggestions
		updatedModel, _ = model.Update(resizeMsg)
		model = updatedModel.(Model)

		heightWithSuggestions := model.messageList.GetHeight()

		// Viewport should be smaller when suggestions are present
		assert.LessOrEqual(t, heightWithSuggestions, heightWithoutSuggestions,
			"Viewport should be smaller or equal when suggestions are present")

		// Clean up
		mockProvider.Close()
	})
}
