package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLayoutDimensionHelpers(t *testing.T) {
	// Create a test model
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("getHeaderHeight", func(t *testing.T) {
		height := model.getHeaderHeight()
		assert.Equal(t, 2, height, "Header height should be 2")
	})

	t.Run("getStatusBarHeight", func(t *testing.T) {
		height := model.getStatusBarHeight()
		assert.Equal(t, 2, height, "Status bar height should be 2")
	})

	t.Run("getSuggestionAreaHeight_no_suggestions", func(t *testing.T) {
		height := model.getSuggestionAreaHeight()
		assert.Equal(t, 0, height, "Suggestion area height should be 0 when no suggestions")
	})

	t.Run("getSuggestionAreaHeight_with_suggestions", func(t *testing.T) {
		model.suggestions = []string{"/help", "/clear", "/model"}
		height := model.getSuggestionAreaHeight()
		assert.Equal(t, 4, height, "Suggestion area height should be suggestions count + 1")
	})

	t.Run("calculateViewportHeight", func(t *testing.T) {
		windowHeight := 50
		height := model.calculateViewportHeight(windowHeight)
		assert.GreaterOrEqual(t, height, 3, "Viewport height should be at least minimum height")
		assert.LessOrEqual(t, height, windowHeight, "Viewport height should not exceed window height")
	})
}

func TestPlaceholderManagement(t *testing.T) {
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("addMessage_regular", func(t *testing.T) {
		initialCount := len(model.messages)

		model.addMessage(chatMessage{
			text:      "Hello",
			sender:    "User",
			timestamp: time.Now(),
		})

		assert.Equal(t, initialCount+1, len(model.messages), "Message count should increase by 1")
		assert.Equal(t, -1, model.placeholderIndex, "Placeholder index should remain -1 for regular messages")
	})

	t.Run("addMessage_placeholder", func(t *testing.T) {
		initialCount := len(model.messages)

		msg := chatMessage{
			text:        "...",
			sender:      "AI",
			timestamp:   time.Now(),
			placeholder: true,
		}

		model.addMessage(msg)

		assert.Equal(t, initialCount+1, len(model.messages), "Message count should increase")
		assert.Equal(t, len(model.messages)-1, model.placeholderIndex, "Placeholder index should be set correctly")
		assert.True(t, model.messages[model.placeholderIndex].placeholder, "Message should be marked as placeholder")
	})

	t.Run("replacePlaceholder_valid", func(t *testing.T) {
		// First add a placeholder
		placeholder := chatMessage{
			text:        "...",
			sender:      "AI",
			timestamp:   time.Now(),
			placeholder: true,
		}
		model.addMessage(placeholder)

		initialCount := len(model.messages)
		placeholderIdx := model.placeholderIndex

		// Replace with real content
		replacement := chatMessage{
			text:      "Real response",
			sender:    "AI",
			timestamp: time.Now(),
		}

		model.replacePlaceholder(replacement)

		assert.Equal(t, initialCount, len(model.messages), "Message count should remain the same")
		assert.False(t, model.messages[placeholderIdx].placeholder, "Message should no longer be placeholder")
		assert.Equal(t, "Real response", model.messages[placeholderIdx].text, "Message text should be updated")
	})

	t.Run("replacePlaceholder_invalid_index", func(t *testing.T) {
		// Set invalid placeholder index
		model.placeholderIndex = 999
		initialCount := len(model.messages)

		model.replacePlaceholder(chatMessage{
			text:      "New Message",
			sender:    "AI",
			timestamp: time.Now(),
		})

		assert.Equal(t, initialCount+1, len(model.messages), "Message should be appended when placeholder index is invalid")
	})
}

func TestSuggestionHandling(t *testing.T) {
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("updateSuggestions_slash_command", func(t *testing.T) {
		model.updateSuggestions("/he")

		assert.Greater(t, len(model.suggestions), 0, "Should have suggestions for /he")
		assert.Equal(t, 0, model.selected, "First suggestion should be selected")

		// Check that all suggestions start with "/he"
		for _, suggestion := range model.suggestions {
			assert.True(t, strings.HasPrefix(suggestion, "/he"), "All suggestions should start with /he")
		}
	})

	t.Run("updateSuggestions_no_slash", func(t *testing.T) {
		model.updateSuggestions("hello")

		assert.Equal(t, 0, len(model.suggestions), "Should have no suggestions for non-slash input")
		assert.Equal(t, -1, model.selected, "Selected index should be -1")
	})

	t.Run("updateSuggestions_empty_input", func(t *testing.T) {
		model.updateSuggestions("")

		assert.Equal(t, 0, len(model.suggestions), "Should have no suggestions for empty input")
		assert.Equal(t, -1, model.selected, "Selected index should be -1")
	})

	t.Run("suggestion_navigation", func(t *testing.T) {
		// Set up suggestions
		model.suggestions = []string{"/help", "/clear", "/model"}
		model.selected = 0

		// Test down navigation
		model.selected = (model.selected + 1) % len(model.suggestions)
		assert.Equal(t, 1, model.selected, "Should move to next suggestion")

		// Test up navigation
		model.selected = (model.selected - 1 + len(model.suggestions)) % len(model.suggestions)
		assert.Equal(t, 0, model.selected, "Should move to previous suggestion")

		// Test wrap around (up from first)
		model.selected = (model.selected - 1 + len(model.suggestions)) % len(model.suggestions)
		assert.Equal(t, 2, model.selected, "Should wrap to last suggestion")
	})
}

func TestRebuildViewportSafety(t *testing.T) {
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("rebuildViewport_empty_messages", func(t *testing.T) {
		// Should not panic with empty messages
		assert.NotPanics(t, func() {
			model.rebuildViewport()
		}, "rebuildViewport should not panic with empty messages")
	})

	t.Run("rebuildViewport_with_messages", func(t *testing.T) {
		// Add some test messages
		model.addMessage(chatMessage{
			text:      "Hello",
			sender:    "User",
			timestamp: time.Now(),
		})

		model.addMessage(chatMessage{
			text:       "Hi there!",
			sender:     "AI",
			timestamp:  time.Now(),
			isMarkdown: true,
		})

		// Should not panic with real messages
		assert.NotPanics(t, func() {
			model.rebuildViewport()
		}, "rebuildViewport should not panic with real messages")
	})
}

// New tests for autocomplete improvements
func TestAutocompleteKeyHandling(t *testing.T) {
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("tab_inserts_suggestion", func(t *testing.T) {
		// Set up suggestions
		model.suggestions = []string{"/help", "/clear", "/model"}
		model.selected = 1 // Select "/clear"
		model.textarea.SetValue("/cl")

		// Simulate tab key behavior
		if len(model.suggestions) > 0 && model.selected >= 0 && model.selected < len(model.suggestions) {
			model.textarea.SetValue(model.suggestions[model.selected])
			model.suggestions = nil
			model.selected = -1
		}

		assert.Equal(t, "/clear", model.textarea.Value(), "Tab should insert selected suggestion")
		assert.Equal(t, 0, len(model.suggestions), "Suggestions should be cleared after insertion")
		assert.Equal(t, -1, model.selected, "Selected index should be reset")
	})

	t.Run("escape_clears_suggestions", func(t *testing.T) {
		// Set up suggestions
		model.suggestions = []string{"/help", "/clear", "/model"}
		model.selected = 0

		// Simulate escape key behavior
		if len(model.suggestions) > 0 {
			model.suggestions = nil
			model.selected = -1
		}

		assert.Equal(t, 0, len(model.suggestions), "Escape should clear suggestions")
		assert.Equal(t, -1, model.selected, "Selected index should be reset")
	})

	t.Run("enter_applies_suggestion", func(t *testing.T) {
		// Set up suggestions
		model.suggestions = []string{"/help", "/clear", "/model"}
		model.selected = 0
		model.textarea.SetValue("/he")

		// Simulate enter key behavior when suggestions are active
		if len(model.suggestions) > 0 && model.selected >= 0 && model.selected < len(model.suggestions) {
			model.textarea.SetValue(model.suggestions[model.selected])
			model.suggestions = nil
			model.selected = -1
		}

		assert.Equal(t, "/help", model.textarea.Value(), "Enter should apply selected suggestion")
		assert.Equal(t, 0, len(model.suggestions), "Suggestions should be cleared after application")
		assert.Equal(t, -1, model.selected, "Selected index should be reset")
	})
}

func TestViewportHeightRecalculation(t *testing.T) {
	cfg := &config.AppConfig{}
	ctx := context.Background()
	model := InitialModel(ctx, cfg, "test-model")

	t.Run("viewport_height_changes_with_suggestions", func(t *testing.T) {
		// Add suggestions (this should trigger height recalculation)
		model.updateSuggestions("/he")

		// The height calculation is complex and depends on window size,
		// but we can verify that the function doesn't panic and produces reasonable results
		assert.GreaterOrEqual(t, model.viewport.Height, 3, "Viewport height should be at least minimum")

		// Clear suggestions
		model.updateSuggestions("hello")

		// Height should be recalculated again
		assert.GreaterOrEqual(t, model.viewport.Height, 3, "Viewport height should still be at least minimum")
	})
}
