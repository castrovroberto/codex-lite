package chat

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInputAreaModel(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *InputAreaModel
		action   func(*InputAreaModel) interface{}
		expected interface{}
	}{
		{
			name: "new_input_area_has_default_height",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				return NewInputAreaModel(theme, []string{"/help", "/clear"})
			},
			action: func(model *InputAreaModel) interface{} {
				return model.GetHeight()
			},
			expected: 3, // Default textarea height
		},
		{
			name: "set_and_get_value",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				return NewInputAreaModel(theme, []string{"/help", "/clear"})
			},
			action: func(model *InputAreaModel) interface{} {
				model.SetValue("test message")
				return model.GetValue()
			},
			expected: "test message",
		},
		{
			name: "reset_clears_value",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				model := NewInputAreaModel(theme, []string{"/help", "/clear"})
				model.SetValue("test message")
				return model
			},
			action: func(model *InputAreaModel) interface{} {
				model.Reset()
				return model.GetValue()
			},
			expected: "",
		},
		{
			name: "slash_command_triggers_suggestions",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				return NewInputAreaModel(theme, []string{"/help", "/clear"})
			},
			action: func(model *InputAreaModel) interface{} {
				model.SetValue("/h")
				model.UpdateSuggestions("/h")
				return model.HasSuggestions()
			},
			expected: true,
		},
		{
			name: "non_slash_input_no_suggestions",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				return NewInputAreaModel(theme, []string{"/help", "/clear"})
			},
			action: func(model *InputAreaModel) interface{} {
				model.SetValue("hello")
				model.UpdateSuggestions("hello")
				return model.HasSuggestions()
			},
			expected: false,
		},
		{
			name: "suggestion_navigation_cycles",
			setup: func() *InputAreaModel {
				theme := NewDefaultTheme()
				model := NewInputAreaModel(theme, []string{"/help", "/clear", "/status"})
				model.SetValue("/")
				model.UpdateSuggestions("/")
				return model
			},
			action: func(model *InputAreaModel) interface{} {
				initial := model.selected
				model.HandleSuggestionNavigation("down")
				after_down := model.selected
				model.HandleSuggestionNavigation("up")
				after_up := model.selected
				return []int{initial, after_down, after_up}
			},
			expected: []int{0, 1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setup()
			result := tt.action(model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInputAreaSuggestionFiltering(t *testing.T) {
	tests := []struct {
		name        string
		commands    []string
		input       string
		expectedLen int
		firstMatch  string
	}{
		{
			name:        "exact_prefix_match",
			commands:    []string{"/help", "/history", "/clear"},
			input:       "/h",
			expectedLen: 2,
			firstMatch:  "/help",
		},
		{
			name:        "single_match",
			commands:    []string{"/help", "/history", "/clear"},
			input:       "/c",
			expectedLen: 1,
			firstMatch:  "/clear",
		},
		{
			name:        "no_matches",
			commands:    []string{"/help", "/history", "/clear"},
			input:       "/x",
			expectedLen: 0,
			firstMatch:  "",
		},
		{
			name:        "empty_slash_shows_all",
			commands:    []string{"/help", "/clear"},
			input:       "/",
			expectedLen: 2,
			firstMatch:  "/help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := NewDefaultTheme()
			model := NewInputAreaModel(theme, tt.commands)
			model.SetValue(tt.input)
			model.UpdateSuggestions(tt.input)

			assert.Equal(t, tt.expectedLen, len(model.suggestions), "Suggestion count mismatch")
			if tt.expectedLen > 0 {
				assert.Equal(t, tt.firstMatch, model.suggestions[0], "First suggestion mismatch")
			}
		})
	}
}

func TestInputAreaSuggestionApplication(t *testing.T) {
	theme := NewDefaultTheme()
	model := NewInputAreaModel(theme, []string{"/help", "/clear"})

	// Setup suggestions
	model.SetValue("/h")
	model.UpdateSuggestions("/h")

	// Apply first suggestion
	applied := model.ApplySelectedSuggestion()
	assert.True(t, applied, "Should apply suggestion")
	assert.Equal(t, "/help", model.GetValue(), "Should set value to selected suggestion")
	assert.False(t, model.HasSuggestions(), "Should clear suggestions after applying")
}

func TestInputAreaWindowResize(t *testing.T) {
	theme := NewDefaultTheme()
	model := NewInputAreaModel(theme, []string{"/help"})

	// Simulate window resize
	resizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, cmd := model.Update(resizeMsg)

	// Should handle resize without errors
	assert.Nil(t, cmd, "Resize should not produce commands")
}

func TestInputAreaSuggestionPersistence(t *testing.T) {
	theme := NewDefaultTheme()
	model := NewInputAreaModel(theme, []string{"/help", "/clear"})

	// Setup suggestions
	model.SetValue("/h")
	model.UpdateSuggestions("/h")
	assert.True(t, model.HasSuggestions(), "Should have suggestions")

	initialCount := len(model.suggestions)
	initialSelected := model.selected

	// Simulate non-input event (window resize)
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	model.Update(resizeMsg)

	// Suggestions should persist
	assert.True(t, model.HasSuggestions(), "Suggestions should persist after resize")
	assert.Equal(t, initialCount, len(model.suggestions), "Suggestion count should remain same")
	assert.Equal(t, initialSelected, model.selected, "Selected index should remain same")
}
