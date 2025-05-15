// internal/tui/chat/model.go
package chat

import (
	"context" // Will be used for Ollama calls
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea" // Or textinput
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// This import is now used
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama" // For making Ollama calls
)

type Model struct {
	cfg        config.AppConfig // Store the AppConfig
	modelName  string           // Store the model name for Ollama calls
	viewport   viewport.Model
	messages   []string
	textarea   textarea.Model // Using textarea for multi-line input
	spinner    spinner.Model
	isLoading  bool
	err        error
	// ... other fields like styles
}

// NewModel is the constructor for the chat TUI model
func NewModel(appConfig config.AppConfig, ollamaModelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0 // No limit by default
	ta.SetWidth(50)  // Example width
	ta.SetHeight(1) // Single line input initially, can grow

	// ... (viewport, spinner, styles initialization)

	return Model{
		cfg:       appConfig, // Store the config
		modelName: ollamaModelName,
		textarea:  ta,
		// ... initialize other fields
		isLoading: false,
	}
}

// Example of how you might use cfg and modelName in an Update function
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.isLoading {
				userPrompt := m.textarea.Value()
				m.messages = append(m.messages, "You: "+userPrompt)
				m.textarea.Reset()
				m.isLoading = true
				// Use m.cfg.OllamaHostURL and m.modelName here
				cmds = append(cmds, m.fetchOllamaResponse(userPrompt))
			}
		}
    // ... other key handling
	case ollamaResponseMsg: // Custom message for Ollama responses
		m.isLoading = false
		m.messages = append(m.messages, "AI: "+string(msg))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	case ollamaErrorMsg: // Custom message for Ollama errors
		m.isLoading = false
		m.err = error(msg) // Store the error to display it
		// Optionally append an error message to m.messages as well

		// ...
	}

	// ... (handle other messages, update components)
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	if m.isLoading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// Define a command to fetch Ollama response
type ollamaResponseMsg string
type ollamaErrorMsg error

func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Create a new context for this specific call, potentially with timeout
		// The FromContext is for extracting, NewContext is for adding.
		// Here, we already have m.cfg, so we don't need to extract it from ctx again.
		// However, if ollama.Query itself needs a context with the config (it doesn't currently),
		// then you'd pass config.NewContext(context.Background(), m.cfg)
		// For now, ollama.Query just needs host, model, prompt.
		response, err := ollama.Query(m.cfg.OllamaHostURL, m.modelName, prompt)
		if err != nil {
			return ollamaErrorMsg(err)
		}
		return ollamaResponseMsg(response)
	}
}

// View method would use m.err to display errors if any
func (m Model) View() string {
    // ... your view logic ...
    var errStr string
    if m.err != nil {
        errStr = "Error: " + m.err.Error()
    }
    // ... display errStr in your TUI ...
    return "..." // Placeholder for actual view
}

func (m Model) Init() tea.Cmd {
    return m.spinner.Tick // Start the spinner
}