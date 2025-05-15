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

	"github.com/castrovroberto/codex-lite/internal/config" // Ensure this path and package are correct
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
	cfg         *config.AppConfig // Check this type if 'undefined' error persists
	modelName   string
	err         error
	loading     bool
	renderer    *glamour.TermRenderer // Moved renderer here
	// Store chat history as a slice of strings or a more structured format
	// For now, we'll just append to the viewport directly.
}

func InitialModel(cfg *config.AppConfig, modelName string) Model { // Check cfg type if 'undefined' error persists
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(50)
	ta.SetHeight(1)

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(50, 10)
	// Use a glamour renderer for markdown
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(vp.Width), // Initial word wrap based on viewport width
	)
	vp.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	vp.SetContent("Welcome to Codex Lite Chat! Type your message and press Enter.")
	// vp.UserData = renderer // Removed: UserData is not a field of viewport.Model

	return Model{
		textarea:    ta,
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		errorStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		cfg:         cfg,
		modelName:   modelName,
		renderer:    renderer, // Initialize the renderer in our model
		err:         nil,
		loading:     false,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

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
			if m.loading {
				return m, nil
			}
			userInput := strings.TrimSpace(m.textarea.Value())
			if userInput == "" {
				return m, nil
			}

			userMessage := m.senderStyle.Render("You: ") + userInput
			m.appendToViewport(userMessage, false) // User messages are not markdown from bot

			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.loading = true
			m.appendToViewport("Bot: Thinking...", false) // Not markdown
			m.viewport.GotoBottom()

			return m, m.fetchOllamaResponse(userInput)
		}

	case ollamaResponseMsg:
		m.loading = false
		// Simplistic removal of "Thinking..." - consider better state management for messages
		// For now, we will just append. A robust solution would edit the message list.
		botResponse := "Bot: " + string(msg)
		m.appendToViewport(botResponse, true) // Bot messages are markdown
		m.viewport.GotoBottom()
		return m, nil

	case ollamaErrorMsg:
		m.loading = false
		errorMsg := m.errorStyle.Render(fmt.Sprintf("Error: %v", msg))
		m.appendToViewport(errorMsg, false) // Not markdown
		m.viewport.GotoBottom()
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - m.textarea.Height() - 1
		m.textarea.SetWidth(msg.Width)

		// Update glamour renderer's word wrap width
		if m.renderer != nil {
			// This recreates the renderer; glamour might not support dynamic width changes easily.
			// Or, you might need to re-render existing content if it stores raw markdown.
			newRenderer, _ := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width), // Use new viewport width
			)
			m.renderer = newRenderer
			// Note: Existing content in the viewport won't automatically re-wrap with just this.
			// You'd need to re-process and SetContent with the re-rendered markdown.
			// This is a complex UI feature usually requiring storing all raw messages.
		}
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// appendToViewport now takes a flag to indicate if content is markdown
func (m *Model) appendToViewport(content string, isMarkdown bool) {
	currentContent := m.viewport.View()
	if currentContent != "" && !strings.HasSuffix(currentContent, "\n") {
		currentContent += "\n"
	}

	finalContent := content
	if isMarkdown && m.renderer != nil && strings.HasPrefix(content, "Bot: ") {
		rawBotMessage := strings.TrimPrefix(content, "Bot: ")
		renderedOutput, err := m.renderer.Render(rawBotMessage)
		if err == nil {
			finalContent = "Bot: " + strings.TrimSpace(renderedOutput)
		} else {
			// Fallback to plain text if rendering fails
			finalContent = "Bot: " + rawBotMessage
		}
	}

	m.viewport.SetContent(currentContent + finalContent)
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("An error occurred: %v\nPress Esc or Ctrl+C to quit.", m.err)
	}
	loadingIndicator := ""
	if m.loading {
		loadingIndicator = " (loading...)"
	}

	return fmt.Sprintf(
		"%s\n\n%s%s",
		m.viewport.View(),
		m.textarea.View(),
		loadingIndicator,
	) + "\n"
}