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

	agentStatusStyle = lipgloss.NewStyle().PaddingLeft(2)
)

// agentProgressMsg is a tea.Msg to send agent progress updates to the TUI model.
// type agentProgressMsg orchestrator.AgentProgressUpdate // Removed

// New messages for overall content update and error reporting
type ContentUpdateMsg string
type ErrorMsg error

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
	program     *tea.Program // Program allows sending messages from outside the TUI
	// filesAgentProgress  map[string]map[string]orchestrator.AgentProgressUpdate // Removed
	// filesAgentOrder     map[string][]string // Removed
	// currentFileProgress string // Removed

	// Used to display progress bar
	progressFilesProcessed int
	progressTotalFiles     int
	progressFileName       string

	processingDone bool // Indicates if all processing has finished
	finalContent   string
	timestamp      string
	appName        string
	modelName      string
}

// HeaderModel represents the header component
type HeaderModel struct {
	provider    string
	model       string
	sessionID   string
	status      string
	startTime   time.Time
	elapsedTime time.Duration
	width       int
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
		// filesAgentProgress: make(map[string]map[string]orchestrator.AgentProgressUpdate), // Removed
		// filesAgentOrder:    make(map[string][]string), // Removed
	}
}

// SetProgram stores the tea.Program instance on the model.
// This is crucial for sending messages to the TUI from external goroutines.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// ProcessAgentUpdate is called by external goroutines to send agent progress updates to the TUI.
// It sends a message that the TUI's Update function will handle.
/* // Removed ProcessAgentUpdate function
func (m *Model) ProcessAgentUpdate(update orchestrator.AgentProgressUpdate) {
	if m.program != nil {
		m.program.Send(agentProgressMsg(update))
	}
}
*/

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

// Update handles model updates
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			if m.ready {
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.header.width = msg.Width
		// Update viewport size
		// Subtract space for header, progress, and agent progress view
		// Adjust this based on your layout
		const headerHeight = 3   // Approximate height for header
		const progressHeight = 3 // Approximate height for overall progress bar
		// const agentViewMinHeight = 5 // Min height for agent view area // Removed

		viewportHeight := m.height - headerHeight - progressHeight
		/* // Removed agent progress view height calculation
		if m.processing && m.currentFileProgress != "" {
			// If showing agent progress, allocate space for it
			agentLines := 0
			if order, ok := m.filesAgentOrder[m.currentFileProgress]; ok {
				agentLines = len(order)
			}
			agentViewHeight := agentLines + 2 // +2 for padding/title
			if agentViewHeight < agentViewMinHeight {
				agentViewHeight = agentViewMinHeight
			}
			if agentViewHeight > viewportHeight/2 {
				agentViewHeight = viewportHeight / 2
			} // Cap agent view
			viewportHeight -= agentViewHeight
		}
		*/
		if viewportHeight < 1 {
			viewportHeight = 1
		} // Ensure viewport height is at least 1

		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
		return m, nil

	case spinner.TickMsg:
		if m.processing {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, cmd

	/* // Removed agentProgressMsg case
	case agentProgressMsg:
		update := orchestrator.AgentProgressUpdate(msg)
		m.currentFileProgress = update.FilePath // Keep track of the latest file having agent progress

		if _, ok := m.filesAgentProgress[update.FilePath]; !ok {
			m.filesAgentProgress[update.FilePath] = make(map[string]orchestrator.AgentProgressUpdate)
			m.filesAgentOrder[update.FilePath] = []string{}
		}

		m.filesAgentProgress[update.FilePath][update.AgentName] = update

		// Add to order if it's a new agent for this file
		found := false
		for _, name := range m.filesAgentOrder[update.FilePath] {
			if name == update.AgentName {
				found = true
				break
			}
		}
		if !found {
			m.filesAgentOrder[update.FilePath] = append(m.filesAgentOrder[update.FilePath], update.AgentName)
		}
	*/

	case ContentUpdateMsg:
		m.finalContent = string(msg)
		m.viewport.SetContent(m.finalContent)
		// Optionally scroll to bottom if new content always means new lines added
		// m.viewport.GotoBottom()
		return m, nil

	case ErrorMsg:
		m.err = msg
		m.processing = false
		m.processingDone = true
		// The View method will display the error.
		// We want to quit, but let the view show the error first.
		// The quit is typically handled by the main analyzeCmd upon p.Run() returning.
		// However, if we want to force quit from here, we can.
		return m, tea.Quit // This will cause p.Run() in analyze.go to unblock.

	case tea.QuitMsg:
		return m, tea.Quit

	default:
		if !m.ready {
			m.viewport, cmd = m.viewport.Update(msg)
			m.ready = true // Assuming viewport readiness sets the model ready
			cmds = append(cmds, cmd)
		}
	}

	// If spinner is active, generate a tick command
	if m.processing && !m.processingDone {
		cmds = append(cmds, m.spinner.Tick)
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

	// Render overall file progress
	if m.processing {
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), statusStyle.Render("Processing Files...")))
		b.WriteString(m.progress.View())
		b.WriteString("\n")
	}

	// Render content viewport
	b.WriteString(m.viewport.View())

	// Render footer with error or help
	if m.err != nil {
		b.WriteString(fmt.Sprintf("\nError: %v", m.err))
	} else {
		b.WriteString(fmt.Sprintf("\n%s", statusStyle.Render("Ctrl+C or q to quit.")))
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
	m.finalContent = content // Assuming finalContent is the field for the main viewport
	if m.program != nil {
		// Send a message to ensure the viewport is updated within the TUI loop
		m.program.Send(ContentUpdateMsg(content))
	} else {
		// If program is not set yet, just update the field directly.
		// This might happen if SetContent is called before p.Run()
		m.viewport.SetContent(content)
	}
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
	if m.program != nil {
		m.program.Send(ErrorMsg(err))
	}
}

// SetProgress updates the progress state (overall file progress)
func (m *Model) SetProgress(current, total int, currentFile string) {
	m.progress.SetProgress(current, total, currentFile)
	// m.currentFileProgress = currentFile // Also update the file for which agent progress is shown // Removed

	// When a new file starts processing for overall progress, clear old agent progress for other files
	// to avoid showing stale agent data if a file completes very quickly before agent updates arrive.
	// This is a simple cleanup. A more robust approach might be needed for complex scenarios.
	/* // Removed agent progress cleanup
	for f := range m.filesAgentProgress {
		if f != currentFile {
			delete(m.filesAgentProgress, f)
			delete(m.filesAgentOrder, f)
		}
	}
	*/
}

// tickMsg is used for updating elapsed time
type tickMsg time.Time

// Err returns any error that was set on the model.
func (m *Model) Err() error {
	return m.err
}
