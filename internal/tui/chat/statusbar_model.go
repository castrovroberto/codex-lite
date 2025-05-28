package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

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

		// Active operations count
		if s.activeToolCalls > 0 {
			statusParts = append(statusParts, fmt.Sprintf("Active: %d", s.activeToolCalls))
		}

		// Session info
		sessionDuration := time.Since(s.chatStartTime)
		statusParts = append(statusParts, fmt.Sprintf("Session: %.0fm", sessionDuration.Minutes()))

		statusBar = s.theme.StatusBar.Render(strings.Join(statusParts, " | "))
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
