package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/logger"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// StatusBarState represents the complete state of the status bar for atomic updates
type StatusBarState struct {
	ActiveToolCalls  int
	SessionStartTime time.Time
	Loading          bool
	Err              error
	LastUpdateTime   time.Time
}

// StatusBarModel manages the status bar display
type StatusBarModel struct {
	theme             *Theme
	spinner           spinner.Model
	loading           bool
	err               error
	thinkingStartTime time.Time
	chatStartTime     time.Time
	activeToolCalls   int
	width             int
	lastState         *StatusBarState // Track last known good state
}

// NewStatusBarModel creates a new status bar model
func NewStatusBarModel(theme *Theme, chatStartTime time.Time) *StatusBarModel {
	// Setup loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Header // Use theme colors

	return &StatusBarModel{
		theme:         theme,
		spinner:       sp,
		loading:       false,
		chatStartTime: chatStartTime,
		width:         50, // Default width
	}
}

// Update handles status bar updates
func (s *StatusBarModel) Update(msg tea.Msg) (*StatusBarModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
	case spinner.TickMsg:
		if s.loading {
			s.spinner, cmd = s.spinner.Update(msg)
		}
	}

	return s, cmd
}

// View renders the status bar
func (s *StatusBarModel) View() string {
	// Validate state before rendering to prevent inconsistencies
	if !s.ValidateState() {
		logger.Get().Warn("Status bar state validation failed, using safe defaults")
		// Use safe defaults
		s.activeToolCalls = 0
		if s.chatStartTime.IsZero() {
			s.chatStartTime = time.Now()
		}
	}

	var statusBar string

	if s.loading {
		elapsed := time.Since(s.thinkingStartTime)
		elapsedStr := fmt.Sprintf("%.1fs", elapsed.Seconds())
		statusBar = s.theme.StatusBar.Render(fmt.Sprintf("%s Thinking... (%s)", s.spinner.View(), elapsedStr))
	} else if s.err != nil {
		statusBar = s.theme.Error.Render("Error: " + s.err.Error())
	} else {
		// Enhanced status bar with more information
		var statusParts []string

		// Basic controls
		statusParts = append(statusParts, "Ctrl+C: quit")
		statusParts = append(statusParts, "Ctrl+E: edit last")
		statusParts = append(statusParts, "Tab: suggestions")

		// Active operations count - always include if > 0
		if s.activeToolCalls > 0 {
			statusParts = append(statusParts, fmt.Sprintf("Active: %d", s.activeToolCalls))
		}

		// Session info - use consistent time source
		sessionDuration := time.Since(s.chatStartTime)
		statusParts = append(statusParts, fmt.Sprintf("Session: %.0fm", sessionDuration.Minutes()))

		// Create full status bar content
		fullStatusContent := strings.Join(statusParts, " | ")

		// Apply theme styling
		statusBar = s.theme.StatusBar.Render(fullStatusContent)

		// Handle truncation only if absolutely necessary and we have a width constraint
		if s.width > 0 && len(fullStatusContent) > s.width {
			// Create minimal version that preserves active tool calls
			var minimalParts []string
			minimalParts = append(minimalParts, "Ctrl+C: quit")

			// Always preserve active tool calls if present
			if s.activeToolCalls > 0 {
				minimalParts = append(minimalParts, fmt.Sprintf("Active: %d", s.activeToolCalls))
			}

			// Add session time
			minimalParts = append(minimalParts, fmt.Sprintf("Session: %.0fm", sessionDuration.Minutes()))

			minimalContent := strings.Join(minimalParts, " | ")
			statusBar = s.theme.StatusBar.Render(minimalContent)
		}
	}

	return statusBar
}

// SetLoading sets the loading state
func (s *StatusBarModel) SetLoading(loading bool) {
	s.loading = loading
	if loading {
		s.thinkingStartTime = time.Now()
	}
}

// SetError sets the error state
func (s *StatusBarModel) SetError(err error) {
	s.err = err
}

// ClearError clears the error state
func (s *StatusBarModel) ClearError() {
	s.err = nil
}

// SetActiveToolCalls sets the number of active tool calls
func (s *StatusBarModel) SetActiveToolCalls(count int) {
	s.activeToolCalls = count
}

// GetHeight returns the status bar height
func (s *StatusBarModel) GetHeight() int {
	return s.theme.StatusBarHeight
}

// GetSpinnerTickCmd returns the spinner tick command if loading
func (s *StatusBarModel) GetSpinnerTickCmd() tea.Cmd {
	if s.loading {
		return s.spinner.Tick
	}
	return nil
}

// UpdateState performs an atomic update of all status bar state
func (s *StatusBarModel) UpdateState(state StatusBarState) {
	s.activeToolCalls = state.ActiveToolCalls
	s.chatStartTime = state.SessionStartTime
	s.loading = state.Loading
	s.err = state.Err
	s.lastState = &state
}

// ValidateState checks for state consistency before rendering
func (s *StatusBarModel) ValidateState() bool {
	// Check for reasonable values
	if s.activeToolCalls < 0 {
		return false
	}
	if s.chatStartTime.IsZero() {
		return false
	}
	return true
}
