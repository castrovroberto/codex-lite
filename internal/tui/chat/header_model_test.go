package chat

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestHeaderModel(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *HeaderModel
		action   func(*HeaderModel) interface{}
		expected interface{}
	}{
		{
			name: "new_header_has_correct_values",
			setup: func() *HeaderModel {
				theme := NewDefaultTheme()
				return NewHeaderModel(theme, "TestProvider", "test-model", "session123", "Connected")
			},
			action: func(model *HeaderModel) interface{} {
				return []string{model.GetProvider(), model.GetModelName(), model.GetSessionID(), model.GetStatus()}
			},
			expected: []string{"TestProvider", "test-model", "session123", "Connected"},
		},
		{
			name: "set_provider_updates_value",
			setup: func() *HeaderModel {
				theme := NewDefaultTheme()
				return NewHeaderModel(theme, "OldProvider", "model", "session", "status")
			},
			action: func(model *HeaderModel) interface{} {
				model.SetProvider("NewProvider")
				return model.GetProvider()
			},
			expected: "NewProvider",
		},
		{
			name: "set_model_name_updates_value",
			setup: func() *HeaderModel {
				theme := NewDefaultTheme()
				return NewHeaderModel(theme, "provider", "old-model", "session", "status")
			},
			action: func(model *HeaderModel) interface{} {
				model.SetModelName("new-model")
				return model.GetModelName()
			},
			expected: "new-model",
		},
		{
			name: "set_session_id_updates_value",
			setup: func() *HeaderModel {
				theme := NewDefaultTheme()
				return NewHeaderModel(theme, "provider", "model", "old-session", "status")
			},
			action: func(model *HeaderModel) interface{} {
				model.SetSessionID("new-session")
				return model.GetSessionID()
			},
			expected: "new-session",
		},
		{
			name: "set_status_updates_value",
			setup: func() *HeaderModel {
				theme := NewDefaultTheme()
				return NewHeaderModel(theme, "provider", "model", "session", "Disconnected")
			},
			action: func(model *HeaderModel) interface{} {
				model.SetStatus("Connected")
				return model.GetStatus()
			},
			expected: "Connected",
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

func TestHeaderModelView(t *testing.T) {
	theme := NewDefaultTheme()
	header := NewHeaderModel(theme, "Ollama", "llama2", "20240101123000", "Connected")

	view := header.View()

	// Verify the view contains key information
	assert.Contains(t, view, "Ollama", "Should show provider")
	assert.Contains(t, view, "llama2", "Should show model name")
	assert.Contains(t, view, "20240101123000", "Should show session ID")
	assert.Contains(t, view, "Connected", "Should show status")
	assert.NotEmpty(t, view, "View should not be empty")
}

func TestHeaderModelUpdate(t *testing.T) {
	theme := NewDefaultTheme()
	header := NewHeaderModel(theme, "Ollama", "model", "session", "Connected")

	// Test window resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedHeader, cmd := header.Update(resizeMsg)

	// Should update without error
	assert.NotNil(t, updatedHeader, "Updated header should not be nil")
	assert.Nil(t, cmd, "Window resize should not produce commands")

	// Test other message types (should be ignored)
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedHeader, cmd = header.Update(keyMsg)
	assert.NotNil(t, updatedHeader, "Updated header should not be nil")
	assert.Nil(t, cmd, "Key messages should not produce commands")
}

func TestHeaderModelGetters(t *testing.T) {
	theme := NewDefaultTheme()
	header := NewHeaderModel(theme, "TestProvider", "test-model", "test-session", "TestStatus")

	// Test all getters
	assert.Equal(t, "TestProvider", header.GetProvider())
	assert.Equal(t, "test-model", header.GetModelName())
	assert.Equal(t, "test-session", header.GetSessionID())
	assert.Equal(t, "TestStatus", header.GetStatus())
}

func TestHeaderModelSetters(t *testing.T) {
	theme := NewDefaultTheme()
	header := NewHeaderModel(theme, "Initial", "initial", "initial", "Initial")

	// Test all setters
	header.SetProvider("Updated Provider")
	header.SetModelName("updated-model")
	header.SetSessionID("updated-session")
	header.SetStatus("Updated Status")

	// Verify updates
	assert.Equal(t, "Updated Provider", header.GetProvider())
	assert.Equal(t, "updated-model", header.GetModelName())
	assert.Equal(t, "updated-session", header.GetSessionID())
	assert.Equal(t, "Updated Status", header.GetStatus())
}
