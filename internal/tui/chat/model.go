// internal/tui/chat/model.go
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type (
	errMsg            error
	ollamaResponseMsg string
	ollamaErrorMsg    error
)

type Model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	senderStyle lipgloss.Style
	errorStyle  lipgloss.Style
	cfg         *config.Config
	modelName   string
	err         error
	loading     bool
	// Store chat history as a slice of strings or a more structured format
	// For now, we'll just append to the viewport directly.
}

func InitialModel(cfg *config.Config, modelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280 // Example limit

	ta.SetWidth(50) // Example width
	ta.SetHeight(1) // Single line input

	// Remove borders for a cleaner look
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(50, 10) // Example dimensions
	// Use a glamour renderer for markdown
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(vp.Width),
	)
	vp.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	// Store the renderer in the viewport's UserData to access it later
	vp.SetContent("Welcome to Codex Lite Chat! Type your message and press Enter.")
	vp.UserData = renderer

	return Model{
		textarea:    ta,
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),  // Purple
		errorStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")),  // Red
		cfg:         cfg,
		modelName:   modelName,
		err:         nil,
		loading:     false,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// fetchOllamaResponse is a tea.Cmd that calls the Ollama API
func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Use a context with a timeout for the Ollama call
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Increased timeout
		defer cancel()

		// Pass the context to the Ollama query
		response, err := ollama.Query(ctx, m.cfg.OllamaHostURL, m.modelName, prompt)
		if err != nil {
			return ollamaErrorMsg(fmt.Errorf("Ollama query failed: %w", err))
		}
		return ollamaResponseMsg(response)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			if m.loading { // Don't send if already loading
				return m, nil
			}
			userInput := strings.TrimSpace(m.textarea.Value())
			if userInput == "" {
				return m, nil
			}

			// Append user message to viewport
			userMessage := m.senderStyle.Render("You: ") + userInput
			m.appendToViewport(userMessage)

			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.loading = true // Set loading state
			// Add a "Bot is thinking..." message
			m.appendToViewport("Bot: Thinking...")
			m.viewport.GotoBottom()

			return m, m.fetchOllamaResponse(userInput)
		}

	case ollamaResponseMsg:
		m.loading = false // Reset loading state
		// Remove the "Bot: Thinking..." message by replacing the last line
		// This is a simplistic way; a more robust way would be to manage messages in a slice.
		lines := strings.Split(m.viewport.View(), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "Bot: Thinking...") {
			// Reconstruct content without the last "Thinking..." line
			// This is still tricky because viewport content might not be raw lines.
			// For now, we'll just append, and the "Thinking" message will remain.
			// A better approach is needed for dynamic message replacement.
		}

		botResponse := "Bot: " + string(msg)
		m.appendToViewport(botResponse)
		m.viewport.GotoBottom()
		return m, nil

	case ollamaErrorMsg:
		m.loading = false // Reset loading state
		errorMsg := m.errorStyle.Render(fmt.Sprintf("Error: %v", msg))
		m.appendToViewport(errorMsg)
		m.viewport.GotoBottom()
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.textarea.Height() - 1 // Adjust for textarea and a potential status line
		m.textarea.SetWidth(msg.Width)
		// Re-render content with new width if using markdown
		if renderer, ok := m.viewport.UserData.(*glamour.TermRenderer); ok {
			// This assumes you have access to the original raw content.
			// For simplicity, we might need to re-fetch or re-format the whole chat history
			// if dynamic re-wrapping of markdown is crucial on resize.
			// For now, existing content might not perfectly re-wrap.
			_ = renderer // Avoid unused variable if not fully implemented
		}
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *Model) appendToViewport(content string) {
	currentContent := m.viewport.View()
	if currentContent != "" && !strings.HasSuffix(currentContent, "\n") {
		currentContent += "\n"
	}
	// Attempt to render with glamour if it's a bot response
	// This is a heuristic; you might need a more robust way to distinguish
	if strings.HasPrefix(content, "Bot: ") {
		if renderer, ok := m.viewport.UserData.(*glamour.TermRenderer); ok {
			rawBotMessage := strings.TrimPrefix(content, "Bot: ")
			renderedOutput, err := renderer.Render(rawBotMessage)
			if err == nil {
				content = "Bot: " + strings.TrimSpace(renderedOutput)
			} else {
				// Fallback to plain text if rendering fails
				content = "Bot: " + rawBotMessage
			}
		}
	}

	m.viewport.SetContent(currentContent + content)
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("An error occurred: %v\nPress Esc or Ctrl+C to quit.", m.err)
	}
	// Add a loading indicator if a response is being fetched
	loadingIndicator := ""
	if m.loading {
		loadingIndicator = " (loading...)"
	}

	return fmt.Sprintf(
		"%s\n\n%s%s", // Added an extra newline for spacing
		m.viewport.View(),
		m.textarea.View(),
		loadingIndicator,
	) + "\n" // Ensure a final newline for better prompt display
}