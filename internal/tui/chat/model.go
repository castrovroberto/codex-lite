package chat

import (
	"fmt"
	"strings"

	"github.com/castrovroberto/codex-lite/internal/config" // Added
	"github.com/castrovroberto/codex-lite/internal/ollama"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	errMsg            error
	ollamaResponseMsg string
)

type Message struct {
	Sender string
	Text   string
}

var (
	aiStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))  // Purple
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))  // Blue
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true) // Red
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(lipgloss.Color("237"))
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 0) // Adjusted padding
)

type Model struct {
	TextInput       textinput.Model
	Viewport        viewport.Model
	Messages        []Message
	Spinner         spinner.Model
	Error           error
	Cwd             string
	OllamaModelName string
	OllamaHostURL   string // Added
	IsLoading       bool
	width           int
	height          int
}

func NewInitialModel(cwd, ollamaModelName, ollamaHostURL string) Model {
	ti := textinput.New()
	ti.Placeholder = "Type your prompt... (Ctrl+C to quit)"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 50 // Will be adjusted on WindowSizeMsg

	vp := viewport.New(50, 10) // Will be adjusted
	vp.SetContent("Welcome to Codex Lite Chat!\nType your prompt below and press Enter. Ctrl+C to quit.")

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		TextInput:       ti,
		Viewport:        vp,
		Messages:        []Message{},
		Spinner:         s,
		Cwd:             cwd,
		OllamaModelName: ollamaModelName,
		OllamaHostURL:   ollamaHostURL, // Added
		IsLoading:       false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

func (m Model) makeOllamaQueryCmd(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Phase 3: Enhance this to include more context from m.Messages
		// For now, just sending the current prompt with minimal history
		var history strings.Builder
		// Include last 2 messages (1 user, 1 AI) for basic context
		numHistory := len(m.Messages)
		startIdx := 0
		if numHistory > 2 {
			startIdx = numHistory - 2
		}
		for _, msg := range m.Messages[startIdx:] {
			prefix := "User: "
			if msg.Sender == "AI" {
				prefix = "AI: "
			}
			history.WriteString(prefix + msg.Text + "\n")
		}
		
		fullPrompt := history.String() + "User: " + prompt

		response, err := ollama.Query(m.OllamaHostURL, m.OllamaModelName, fullPrompt)
		if err != nil {
			return errMsg(err)
		}
		return ollamaResponseMsg(response)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.IsLoading {
				return m, nil
			}
			userInput := strings.TrimSpace(m.TextInput.Value())
			if userInput == "" {
				return m, nil
			}

			m.Messages = append(m.Messages, Message{Sender: "You", Text: userInput})
			m.TextInput.Reset()
			m.IsLoading = true
			cmds = append(cmds, m.Spinner.Tick, m.makeOllamaQueryCmd(userInput))
			m.updateViewportContent()
			m.Viewport.GotoBottom()
			return m, tea.Batch(cmds...)
		}

	case ollamaResponseMsg:
		m.IsLoading = false
		m.Messages = append(m.Messages, Message{Sender: "AI", Text: strings.TrimSpace(string(msg))})
		m.Error = nil
		m.updateViewportContent()
		m.Viewport.GotoBottom()
		return m, nil

	case errMsg:
		m.IsLoading = false
		m.Error = msg
		m.updateViewportContent() // To show error if it's part of the view
		m.Viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if m.IsLoading {
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		inputHeight := 3 // textinput.View() is 1 line, border is 2 lines = 3
		errorHeight := 0
		if m.Error != nil {
			errorHeight = 1
		}

		m.Viewport.Width = msg.Width
		m.Viewport.Height = msg.Height - headerHeight - inputHeight - errorHeight - 1 // -1 for a small bottom margin
		m.TextInput.Width = msg.Width - 4 // Account for input box border (2) and padding (2)

		m.updateViewportContent()
		m.Viewport.GotoBottom()
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	cmds = append(cmds, cmd)

	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewportContent() {
	var sb strings.Builder
	for _, msg := range m.Messages {
		var styledSender, styledText string
		if msg.Sender == "You" {
			styledSender = userStyle.Render(msg.Sender + ": ")
			styledText = msg.Text
		} else if msg.Sender == "AI" {
			styledSender = aiStyle.Render(msg.Sender + ": ")
			styledText = msg.Text
		}
		sb.WriteString(styledSender + styledText + "\n")
	}
	m.Viewport.SetContent(sb.String())
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var view strings.Builder
	headerTop := fmt.Sprintf("Codex Lite - Interactive Session | Workdir: %s", m.Cwd)
	headerBottom := fmt.Sprintf("Model: %s", m.OllamaModelName)
	view.WriteString(headerStyle.Width(m.width).Render(headerTop) + "\n")
	view.WriteString(headerStyle.Width(m.width).Render(headerBottom) + "\n")
	view.WriteString(m.Viewport.View() + "\n")

	inputArea := m.TextInput.View()
	if m.IsLoading {
		inputArea = m.Spinner.View() + " Thinking..."
	}
	view.WriteString(inputStyle.Width(m.width - 2).Render(inputArea)) // -2 for border

	if m.Error != nil {
		view.WriteString("\n" + errorStyle.Render("Error: "+m.Error.Error()))
	}
	return view.String()
}