package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	progressStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2)

	progressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5A189A"))

	progressTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C77DFF"))
)

// ProgressModel represents the progress component
type ProgressModel struct {
	progress    progress.Model
	total       int
	current     int
	currentFile string
	width       int
}

// NewProgressModel creates a new progress model
func NewProgressModel() ProgressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	p.Full = '█'
	p.Empty = '░'

	return ProgressModel{
		progress: p,
	}
}

// Init initializes the progress model
func (m ProgressModel) Init() tea.Cmd {
	return nil
}

// Update handles progress model updates
func (m ProgressModel) Update(msg tea.Msg) (ProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.progress.Width = msg.Width - 20
	}

	progressModel, cmd := m.progress.Update(msg)
	m.progress = progressModel.(progress.Model)
	return m, cmd
}

// View renders the progress model
func (m ProgressModel) View() string {
	if m.total == 0 {
		return ""
	}

	var b strings.Builder

	// Progress bar
	percent := float64(m.current) / float64(m.total)
	bar := m.progress.ViewAs(percent)

	// Progress text
	text := fmt.Sprintf("%d/%d files", m.current, m.total)
	if m.currentFile != "" {
		text = fmt.Sprintf("%s • Current: %s", text, m.currentFile)
	}

	b.WriteString(progressStyle.Render(bar))
	b.WriteString("\n")
	b.WriteString(progressTextStyle.Render(text))

	return b.String()
}

// SetProgress updates the progress state
func (m *ProgressModel) SetProgress(current, total int, currentFile string) {
	m.current = current
	m.total = total
	m.currentFile = currentFile
}
