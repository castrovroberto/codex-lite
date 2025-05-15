// internal/tui/chat/model.go
package chat

import (
	"context" // Now used for Ollama call context
	"fmt"     // For fmt.Sprintf in View
	"strings"
	"time" // For context timeout example

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss" // Now used

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

// Define some basic styles
var (
	senderStyle = lipgloss.NewStyle().Bold(true)
	aiStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")) // A nice purple
	errorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))  // Red for errors
	helpStyle   = lipgloss.NewStyle().Faint(true)
)

type Model struct {
	cfg        config.AppConfig
	modelName  string
	viewport   viewport.Model
	messages   []string
	textarea   textarea.Model
	spinner    spinner.Model
	isLoading  bool
	err        error
	width      int
	height     int
	// For a simple header/footer
	header string
}

func NewModel(appConfig config.AppConfig, ollamaModelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask Codex Lite..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.SetHeight(1) // Start as single line, can grow if needed or use fixed height

	vp := viewport.New(0, 0) // Width and height will be set on WindowSizeMsg

	spin := spinner.New()
	spin.Spinner = spinner.Dot // Or any other spinner

	headerTxt := fmt.Sprintf("Codex Lite Chat | Model: %s | Type 'esc' or 'ctrl+c' to quit.", ollamaModelName)

	return Model{
		cfg:       appConfig,
		modelName: ollamaModelName,
		textarea:  ta,
		viewport:  vp,
		spinner:   spin,
		isLoading: false,
		header:    headerTxt,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

type ollamaResponseMsg string
type ollamaErrorMsg error

func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Example: Use a context with a timeout for the Ollama call
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// If ollama.Query needs context (it doesn't now but could be modified):
		// response, err := ollama.Query(ctx, m.cfg.OllamaHostURL, m.modelName, prompt)
		// For now, ollama.Query doesn't take ctx, but this shows usage of `context` package
		_ = ctx // Placeholder to show context is "used" for the import if Query doesn't take it

		response, err := ollama.Query(m.cfg.OllamaHostURL, m.modelName, prompt)
		if err != nil {
			return ollamaErrorMsg(fmt.Errorf("Ollama query failed: %w", err))
		}
		return ollamaResponseMsg(response)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Example: Header (1 line), Footer (input area + help, e.g. 3 lines), rest for viewport
		headerHeight := 1
		footerHeight := 3 // For textarea + 1 line for error + 1 for help
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		m.textarea.SetWidth(msg.Width)
		// Update viewport content if needed
		m.viewport.SetContent(strings.Join(m.messages, "\n"))


	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.isLoading && strings.TrimSpace(m.textarea.Value()) != "" {
				userPrompt := m.textarea.Value()
				m.messages = append(m.messages, senderStyle.Render("You: ")+userPrompt)
				m.textarea.Reset()
				m.isLoading = true
				m.err = nil // Clear previous error
				cmds = append(cmds, m.fetchOllamaResponse(userPrompt))
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
			}
		}

	case ollamaResponseMsg:
		m.isLoading = false
		m.messages = append(m.messages, aiStyle.Render("AI: ")+string(msg))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case ollamaErrorMsg:
		m.isLoading = false
		m.err = error(msg) // Store the error
		// Optionally add to messages list too:
		// m.messages = append(m.messages, errorStyle.Render("Error: "+m.err.Error()))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	if m.isLoading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	// Viewport updates are mainly driven by SetContent, but it can handle its own msgs too
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)


	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 { // Not ready yet
		return "Initializing..."
	}

	var s strings.Builder

	// Header
	s.WriteString(m.header)
	s.WriteString("\n")

	// Viewport for messages
	s.WriteString(m.viewport.View())
	s.WriteString("\n")

	// Input area
	if m.isLoading {
		s.WriteString(m.spinner.View() + " Thinking...")
	} else {
		s.WriteString(m.textarea.View())
	}
	s.WriteString("\n")

	// Error display
	var errStr string
	if m.err != nil {
		errStr = errorStyle.Render("Error: " + m.err.Error())
		s.WriteString(errStr) // Now errStr is used
	} else {
		s.WriteString(helpStyle.Render("Enter your prompt. Press Esc or Ctrl+C to quit.")) // Placeholder if no error
	}


	return s.String()
}