package chat

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// HeaderModel manages the header display
type HeaderModel struct {
	theme     *Theme
	provider  string
	modelName string
	sessionID string
	status    string
	width     int
}

// NewHeaderModel creates a new header model
func NewHeaderModel(theme *Theme, provider, modelName, sessionID, status string) *HeaderModel {
	return &HeaderModel{
		theme:     theme,
		provider:  provider,
		modelName: modelName,
		sessionID: sessionID,
		status:    status,
		width:     50, // Default width
	}
}

// Update handles header-specific updates
func (h *HeaderModel) Update(msg tea.Msg) (*HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
	}
	return h, nil
}

// View renders the header
func (h *HeaderModel) View() string {
	headerText := fmt.Sprintf("Chat with %s (%s) | Session: %s | Status: %s",
		h.provider, h.modelName, h.sessionID, h.status)
	return h.theme.Header.Render(headerText)
}

// SetProvider updates the provider name
func (h *HeaderModel) SetProvider(provider string) {
	h.provider = provider
}

// SetModelName updates the model name
func (h *HeaderModel) SetModelName(modelName string) {
	h.modelName = modelName
}

// SetSessionID updates the session ID
func (h *HeaderModel) SetSessionID(sessionID string) {
	h.sessionID = sessionID
}

// SetStatus updates the status
func (h *HeaderModel) SetStatus(status string) {
	h.status = status
}

// GetHeight returns the header height
func (h *HeaderModel) GetHeight() int {
	return h.theme.HeaderHeight
}

// GetSessionID returns the session ID
func (h *HeaderModel) GetSessionID() string {
	return h.sessionID
}

// GetModelName returns the model name
func (h *HeaderModel) GetModelName() string {
	return h.modelName
}
