package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7B2CBF")).
			PaddingLeft(2)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9D4EDD")).
			PaddingLeft(2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C77DFF"))

	headerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#5A189A"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8FA3"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8FA3"))
)

// Model represents the main TUI model
type Model struct {
	viewport    viewport.Model
	spinner     spinner.Model
	progress    ProgressModel
	header      HeaderModel
	content     string
	ready       bool
	err         error
	width       int
	height      int
	processing  bool
	startTime   time.Time
	elapsedTime time.Duration
}

// HeaderModel represents the header component
type HeaderModel struct {
	provider    string
	model       string
	sessionID   string
	status      string
	startTime   time.Time
	elapsedTime time.Duration
}

// NewModel creates a new TUI model
func NewModel(provider, model, sessionID string) Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot

	return Model{
		header: HeaderModel{
			provider:  provider,
			model:     model,
			sessionID: sessionID,
			status:    "Ready",
		},
		spinner:  s,
		progress: NewProgressModel(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

// Update handles model updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-8) // Subtract header and progress height
			m.viewport.Style = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5A189A"))
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 8
		}
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Update progress
	var progressCmd tea.Cmd
	m.progress, progressCmd = m.progress.Update(msg)
	cmds = append(cmds, progressCmd)

	// Update elapsed time if processing
	if m.processing {
		m.elapsedTime = time.Since(m.startTime)
		m.header.elapsedTime = m.elapsedTime
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
	}

	return m, tea.Batch(cmds...)
}

// View renders the model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Render header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Render progress
	if m.processing {
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), statusStyle.Render("Processing...")))
		b.WriteString(m.progress.View())
		b.WriteString("\n")
	}

	// Render content
	b.WriteString(m.viewport.View())

	// Render footer
	if m.err != nil {
		b.WriteString(fmt.Sprintf("\nError: %v", m.err))
	}

	return b.String()
}

// renderHeader renders the header component
func (m Model) renderHeader() string {
	var b strings.Builder

	// Title and subtitle
	b.WriteString(titleStyle.Render("Codex Lite"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("AI-Powered Code Analysis"))
	b.WriteString("\n")

	// Info line
	info := fmt.Sprintf(
		"%s • %s • Session: %s • %s • Elapsed: %s",
		m.header.provider,
		m.header.model,
		m.header.sessionID,
		m.header.status,
		m.header.elapsedTime.Round(time.Second),
	)
	b.WriteString(infoStyle.Render(info))

	return headerStyle.Width(m.width).Render(b.String())
}

// SetContent updates the viewport content
func (m *Model) SetContent(content string) {
	m.viewport.SetContent(content)
}

// StartProcessing starts the processing state
func (m *Model) StartProcessing() {
	m.processing = true
	m.startTime = time.Now()
	m.header.startTime = m.startTime
	m.header.status = "Processing"
}

// StopProcessing stops the processing state
func (m *Model) StopProcessing() {
	m.processing = false
	m.header.status = "Ready"
}

// SetError sets an error state
func (m *Model) SetError(err error) {
	m.err = err
	m.header.status = "Error"
}

// SetProgress updates the progress state
func (m *Model) SetProgress(current, total int, currentFile string) {
	m.progress.SetProgress(current, total, currentFile)
}

// tickMsg is used for updating elapsed time
type tickMsg time.Time
