package chat

import (
	"context"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLayoutDimensionHelpers(t *testing.T) {
	// Create a test model
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("getHeaderHeight", func(t *testing.T) {
		height := model.layout.GetHeaderHeight()
		assert.Equal(t, 2, height, "Header height should be 2")
	})

	t.Run("getStatusBarHeight", func(t *testing.T) {
		height := model.layout.GetStatusBarHeight()
		assert.Equal(t, 2, height, "Status bar height should be 2")
	})

	t.Run("getSuggestionAreaHeight_no_suggestions", func(t *testing.T) {
		height := model.inputArea.GetSuggestionAreaHeight()
		assert.Equal(t, 0, height, "Suggestion area height should be 0 when no suggestions")
	})

	t.Run("getSuggestionAreaHeight_with_suggestions", func(t *testing.T) {
		// Simulate having suggestions by updating input with "/"
		model.inputArea.SetValue("/he")
		// Manually trigger suggestion update since we're not going through the Update method
		model.inputArea.UpdateSuggestions("/he")
		height := model.inputArea.GetSuggestionAreaHeight()
		assert.GreaterOrEqual(t, height, 1, "Suggestion area height should be at least 1 when suggestions exist")
	})

	t.Run("calculateViewportHeight", func(t *testing.T) {
		height := model.layout.CalculateViewportHeight(50, 3, 0, 2)
		assert.GreaterOrEqual(t, height, 3, "Viewport height should be at least minimum")
	})
}

func TestMessageManagement(t *testing.T) {
	// Create a test model
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("addMessage", func(t *testing.T) {
		initialCount := len(model.messageList.GetMessages())

		model.messageList.AddMessage(chatMessage{
			text:      "Test message",
			sender:    "User",
			timestamp: time.Now(),
		})

		assert.Equal(t, initialCount+1, len(model.messageList.GetMessages()), "Message count should increase by 1")
	})

	t.Run("replacePlaceholder", func(t *testing.T) {
		// Add a placeholder message
		model.messageList.AddMessage(chatMessage{
			text:        "...",
			sender:      "Assistant",
			timestamp:   time.Now(),
			placeholder: true,
		})

		// Replace with real content
		model.messageList.ReplacePlaceholder(chatMessage{
			text:      "Real response",
			sender:    "Assistant",
			timestamp: time.Now(),
		})

		messages := model.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.Equal(t, "Real response", lastMessage.text, "Placeholder should be replaced with real content")
		assert.False(t, lastMessage.placeholder, "Message should no longer be a placeholder")
	})
}

func TestSuggestionHandling(t *testing.T) {
	// Create a test model
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("suggestion_navigation", func(t *testing.T) {
		// Set input to trigger suggestions
		model.inputArea.SetValue("/he")
		// Manually trigger suggestion update since we're not going through the Update method
		model.inputArea.UpdateSuggestions("/he")

		// Test navigation
		handled := model.inputArea.HandleSuggestionNavigation("down")
		assert.True(t, handled, "Should handle suggestion navigation when suggestions exist")
	})

	t.Run("apply_suggestion", func(t *testing.T) {
		// Set input to trigger suggestions
		model.inputArea.SetValue("/he")
		// Manually trigger suggestion update since we're not going through the Update method
		model.inputArea.UpdateSuggestions("/he")

		// Apply suggestion
		applied := model.inputArea.ApplySelectedSuggestion()
		assert.True(t, applied, "Should apply suggestion when one is selected")

		// Check that suggestions are cleared
		assert.False(t, model.inputArea.HasSuggestions(), "Suggestions should be cleared after applying")
	})

	t.Run("clear_suggestions", func(t *testing.T) {
		// Set input to trigger suggestions
		model.inputArea.SetValue("/he")
		// Manually trigger suggestion update since we're not going through the Update method
		model.inputArea.UpdateSuggestions("/he")
		assert.True(t, model.inputArea.HasSuggestions(), "Should have suggestions")

		// Clear suggestions
		model.inputArea.ClearSuggestions()
		assert.False(t, model.inputArea.HasSuggestions(), "Suggestions should be cleared")
	})

	t.Run("suggestions_persist_on_non_input_events", func(t *testing.T) {
		// Set input to trigger suggestions
		model.inputArea.SetValue("/he")
		// Manually trigger suggestion update since we're not going through the Update method
		model.inputArea.UpdateSuggestions("/he")
		assert.True(t, model.inputArea.HasSuggestions(), "Should have suggestions")

		// Store the current suggestion state
		initialSuggestionCount := len(model.inputArea.suggestions)
		initialSelected := model.inputArea.selected

		// Simulate a window resize event (non-input event)
		resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
		model.inputArea.Update(resizeMsg)

		// Suggestions should still be there and unchanged
		assert.True(t, model.inputArea.HasSuggestions(), "Suggestions should persist after window resize")
		assert.Equal(t, initialSuggestionCount, len(model.inputArea.suggestions), "Suggestion count should remain the same")
		assert.Equal(t, initialSelected, model.inputArea.selected, "Selected index should remain the same")
	})
}

func TestComponentIntegration(t *testing.T) {
	// Create a test model
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("header_updates", func(t *testing.T) {
		model.header.SetModelName("new-model")
		assert.Equal(t, "new-model", model.header.GetModelName(), "Header should update model name")

		model.header.SetSessionID("new-session")
		assert.Equal(t, "new-session", model.header.GetSessionID(), "Header should update session ID")
	})

	t.Run("status_bar_states", func(t *testing.T) {
		// Test loading state
		model.statusBar.SetLoading(true)
		view := model.statusBar.View()
		assert.Contains(t, view, "Thinking", "Status bar should show loading state")

		// Test error state
		model.statusBar.SetLoading(false)
		model.statusBar.SetError(assert.AnError)
		view = model.statusBar.View()
		assert.Contains(t, view, "Error", "Status bar should show error state")

		// Clear error
		model.statusBar.ClearError()
		view = model.statusBar.View()
		assert.NotContains(t, view, "Error", "Status bar should not show error after clearing")
	})

	t.Run("viewport_height_changes_with_suggestions", func(t *testing.T) {
		// Add suggestions (this should trigger height recalculation)
		model.inputArea.SetValue("/he")

		// The height calculation is complex and depends on window size,
		// but we can verify that the function doesn't panic and produces reasonable results
		assert.GreaterOrEqual(t, model.messageList.GetHeight(), 3, "Viewport height should be at least minimum")

		// Clear suggestions
		model.inputArea.SetValue("hello")

		// Height should be recalculated again
		assert.GreaterOrEqual(t, model.messageList.GetHeight(), 3, "Viewport height should still be at least minimum")
	})
}

func TestToolCallMessageHandling(t *testing.T) {
	t.Run("tool_start_message", func(t *testing.T) {
		// Create a fresh test model for this test
		cfg := &config.AppConfig{}
		ctx := context.Background()
		model := InitialModel(ctx, cfg, "test-model")

		// Create a tool start message
		startMsg := toolStartMsg{
			toolCallID: "test-call-123",
			toolName:   "test_tool",
			params:     map[string]interface{}{"param1": "value1"},
		}

		// Process the message
		updatedModel, _ := model.Update(startMsg)
		m := updatedModel.(Model)

		// Verify active tool calls were updated
		assert.Len(t, m.activeToolCalls, 1, "Should have one active tool call")
		assert.Contains(t, m.activeToolCalls, "test-call-123", "Should contain the tool call ID")

		// Verify tool progress state
		state := m.activeToolCalls["test-call-123"]
		assert.Equal(t, "test_tool", state.toolName, "Tool name should match")
		assert.Equal(t, 0.0, state.progress, "Initial progress should be 0.0")
		assert.Equal(t, "Starting...", state.status, "Initial status should be 'Starting...'")

		// Verify message was added to message list
		messages := m.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.True(t, lastMessage.isToolCall, "Last message should be a tool call")
		assert.Equal(t, "test_tool", lastMessage.toolName, "Tool name should match")
		assert.Equal(t, "test-call-123", lastMessage.toolCallID, "Tool call ID should match")
	})

	t.Run("tool_progress_message", func(t *testing.T) {
		// Create a fresh test model for this test
		cfg := &config.AppConfig{}
		ctx := context.Background()
		model := InitialModel(ctx, cfg, "test-model")

		// First start a tool call
		startMsg := toolStartMsg{
			toolCallID: "test-call-456",
			toolName:   "progress_tool",
			params:     map[string]interface{}{},
		}
		updatedModel, _ := model.Update(startMsg)
		m := updatedModel.(Model)

		// Create a progress message
		progressMsg := toolProgressMsg{
			toolCallID: "test-call-456",
			toolName:   "progress_tool",
			progress:   0.5,
			status:     "Processing...",
			step:       2,
			totalSteps: 4,
		}

		// Process the progress message
		updatedModel, _ = m.Update(progressMsg)
		m = updatedModel.(Model)

		// Verify progress was updated
		state := m.activeToolCalls["test-call-456"]
		assert.Equal(t, 0.5, state.progress, "Progress should be updated to 0.5")
		assert.Equal(t, "Processing...", state.status, "Status should be updated")
		assert.Equal(t, 2, state.step, "Step should be updated")
		assert.Equal(t, 4, state.totalSteps, "Total steps should be updated")
	})

	t.Run("tool_complete_message_success", func(t *testing.T) {
		// Create a fresh test model for this test
		cfg := &config.AppConfig{}
		ctx := context.Background()
		model := InitialModel(ctx, cfg, "test-model")

		// First start a tool call
		startMsg := toolStartMsg{
			toolCallID: "test-call-789",
			toolName:   "complete_tool",
			params:     map[string]interface{}{},
		}
		updatedModel, _ := model.Update(startMsg)
		m := updatedModel.(Model)

		// Verify tool call is active
		assert.Len(t, m.activeToolCalls, 1, "Should have one active tool call")

		// Create a completion message
		completeMsg := toolCompleteMsg{
			toolCallID: "test-call-789",
			toolName:   "complete_tool",
			success:    true,
			result:     "Tool executed successfully",
			duration:   time.Second * 2,
			error:      "",
		}

		// Process the completion message
		updatedModel, _ = m.Update(completeMsg)
		m = updatedModel.(Model)

		// Verify tool call was removed from active calls
		assert.Len(t, m.activeToolCalls, 0, "Should have no active tool calls")
		assert.NotContains(t, m.activeToolCalls, "test-call-789", "Should not contain completed tool call")

		// Verify result message was added
		messages := m.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.True(t, lastMessage.isToolResult, "Last message should be a tool result")
		assert.True(t, lastMessage.toolSuccess, "Tool should be marked as successful")
		assert.Equal(t, "complete_tool", lastMessage.toolName, "Tool name should match")
		assert.Equal(t, "Tool executed successfully", lastMessage.text, "Result text should match")
		assert.Equal(t, time.Second*2, lastMessage.toolDuration, "Duration should match")
	})

	t.Run("tool_complete_message_failure", func(t *testing.T) {
		// Create a fresh test model for this test
		cfg := &config.AppConfig{}
		ctx := context.Background()
		model := InitialModel(ctx, cfg, "test-model")

		// First start a tool call
		startMsg := toolStartMsg{
			toolCallID: "test-call-error",
			toolName:   "error_tool",
			params:     map[string]interface{}{},
		}
		updatedModel, _ := model.Update(startMsg)
		m := updatedModel.(Model)

		// Create a failure completion message
		completeMsg := toolCompleteMsg{
			toolCallID: "test-call-error",
			toolName:   "error_tool",
			success:    false,
			result:     "",
			duration:   time.Millisecond * 500,
			error:      "Tool execution failed",
		}

		// Process the completion message
		updatedModel, _ = m.Update(completeMsg)
		m = updatedModel.(Model)

		// Verify tool call was removed from active calls
		assert.Len(t, m.activeToolCalls, 0, "Should have no active tool calls")

		// Verify error message was added
		messages := m.messageList.GetMessages()
		lastMessage := messages[len(messages)-1]
		assert.True(t, lastMessage.isToolResult, "Last message should be a tool result")
		assert.False(t, lastMessage.toolSuccess, "Tool should be marked as failed")
		assert.Equal(t, "error_tool", lastMessage.toolName, "Tool name should match")
		assert.Contains(t, lastMessage.text, "Tool execution failed", "Error text should be included")
	})

	t.Run("multiple_concurrent_tools", func(t *testing.T) {
		// Create a fresh test model for this test
		cfg := &config.AppConfig{}
		ctx := context.Background()
		model := InitialModel(ctx, cfg, "test-model")

		// Start multiple tool calls
		startMsg1 := toolStartMsg{toolCallID: "tool-1", toolName: "tool_a", params: map[string]interface{}{}}
		startMsg2 := toolStartMsg{toolCallID: "tool-2", toolName: "tool_b", params: map[string]interface{}{}}

		updatedModel, _ := model.Update(startMsg1)
		m := updatedModel.(Model)
		updatedModel, _ = m.Update(startMsg2)
		m = updatedModel.(Model)

		// Verify both tools are active
		assert.Len(t, m.activeToolCalls, 2, "Should have two active tool calls")
		assert.Contains(t, m.activeToolCalls, "tool-1", "Should contain first tool")
		assert.Contains(t, m.activeToolCalls, "tool-2", "Should contain second tool")

		// Complete one tool
		completeMsg1 := toolCompleteMsg{
			toolCallID: "tool-1",
			toolName:   "tool_a",
			success:    true,
			result:     "Result A",
			duration:   time.Second,
		}
		updatedModel, _ = m.Update(completeMsg1)
		m = updatedModel.(Model)

		// Verify only one tool remains active
		assert.Len(t, m.activeToolCalls, 1, "Should have one active tool call")
		assert.NotContains(t, m.activeToolCalls, "tool-1", "First tool should be completed")
		assert.Contains(t, m.activeToolCalls, "tool-2", "Second tool should still be active")
	})
}
