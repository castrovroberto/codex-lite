// internal/tui/chat/model.go
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	// Import slog for logging errors
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/castrovroberto/codex-lite/internal/config" // Ensure this path and package are correct
	"github.com/castrovroberto/codex-lite/internal/logger" // Import logger to get the global logger
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type (
	errMsg            error
	ollamaResponseMsg string
	ollamaErrorMsg    error
)

// chatMessage holds a single chat entry for re-rendering (with optional markdown)
type chatMessage struct {
	text        string
	isMarkdown  bool
	placeholder bool
}

type Model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	senderStyle lipgloss.Style
	errorStyle  lipgloss.Style
	cfg         *config.AppConfig // Check this type if 'undefined' error persists
	modelName   string
	parentCtx   context.Context // Store the parent context
	err         error
	loading     bool
	renderer    *glamour.TermRenderer // Glamour markdown renderer
	// Chat history as structured messages for re-rendering and resize support
	messages         []chatMessage
	placeholderIndex int // index of the current placeholder message, or -1 if none
}

func InitialModel(ctx context.Context, cfg *config.AppConfig, modelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(50)
	// Allow multi-line input for longer prompts
	ta.SetHeight(3)

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	// Initial dimensions: width matches textarea, height arbitrary
	vp := viewport.New(50, 10)
	// Use a glamour renderer for markdown
	// Handle potential error during renderer creation
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(vp.Width), // Initial word wrap based on viewport width
	)
	vp.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	if err != nil {
		// Log the error and proceed without markdown rendering
		logger.Get().Error("Failed to initialize glamour markdown renderer", "error", err)
		renderer = nil // Ensure renderer is nil if creation failed
	}

	// Prepare the model with initial message history
	m := Model{
		textarea:         ta,
		viewport:         vp,
		senderStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		cfg:              cfg,
		modelName:        modelName,
		parentCtx:        ctx,      // Store the provided context
		renderer:         renderer, // Initialize the renderer in our model
		err:              nil,
		loading:          false,
		messages:         nil,
		placeholderIndex: -1,
	}
	// Seed welcome message
	m.messages = []chatMessage{{text: "Welcome to Codex Lite Chat! Type your message and press Enter.", isMarkdown: false, placeholder: false}}
	// Render initial viewport content
	m.rebuildViewport()
	return m
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Use the parentCtx from the model, which should have AppConfig and Logger
		// If parentCtx is nil, default to context.Background(), but this indicates a setup issue.
		baseCtx := m.parentCtx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(baseCtx, 60*time.Second)
		defer cancel()

		response, err := ollama.Query(ctx, m.cfg.OllamaHostURL, m.modelName, prompt)
		if err != nil {
			return ollamaErrorMsg(fmt.Errorf("Ollama query failed: %w", err))
		}
		return ollamaResponseMsg(response)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
   // Intercept scroll keys to allow scrolling the chat viewport even when the textarea is focused
   if keyMsg, ok := msg.(tea.KeyMsg); ok {
       switch keyMsg.Type {
       case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
           // Scroll the viewport
           var vpCmd tea.Cmd
           m.viewport, vpCmd = m.viewport.Update(keyMsg)
           return m, vpCmd
       }
   }
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
			m.addMessage(userMessage, false, false)

			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.loading = true
			m.addMessage("Bot: Thinking...", false, true) // placeholder for bot response
			m.viewport.GotoBottom()

			return m, m.fetchOllamaResponse(userInput)
		}

	case ollamaResponseMsg:
		m.loading = false
		// Simplistic removal of "Thinking..." - consider better state management for messages
		// For now, we will just append. A robust solution would edit the message list.
		botResponse := "Bot: " + string(msg)
		m.loading = false
		m.replacePlaceholder(botResponse, true)
		return m, nil

	case ollamaErrorMsg:
		m.loading = false
		errorMsg := m.errorStyle.Render(fmt.Sprintf("Error: %v", msg))
		m.loading = false
		m.replacePlaceholder(errorMsg, false)
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		// Adjust for viewport border/frame
		wFrame := m.viewport.Style.GetHorizontalFrameSize()
		hFrame := m.viewport.Style.GetVerticalFrameSize()
		m.viewport.Width = msg.Width - wFrame
		m.viewport.Height = msg.Height - m.textarea.Height() - 1 - hFrame
		m.textarea.SetWidth(msg.Width)

		// Update glamour renderer's word wrap width
		if m.renderer != nil {
			// This recreates the renderer; glamour might not support dynamic width changes easily.
			// Or, you might need to re-render existing content if it stores raw markdown.
			// Handle potential error during renderer recreation on resize
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width), // Use new viewport width
			)
			if err != nil {
				logger.Get().Error("Failed to re-initialize glamour markdown renderer on resize", "error", err)
				// Keep the old renderer or keep it nil if it was already nil
			} else {
				m.renderer = newRenderer
			}
			// Re-render all messages to wrap to new width
			m.rebuildViewport()
		}
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// rebuildViewport re-renders all stored messages into the viewport, applying markdown and wrapping on resize.
func (m *Model) rebuildViewport() {
	var b strings.Builder
	for _, cm := range m.messages {
		if cm.isMarkdown && m.renderer != nil && strings.HasPrefix(cm.text, "Bot: ") {
			raw := strings.TrimPrefix(cm.text, "Bot: ")
			rendered, err := m.renderer.Render(raw)
			if err != nil {
				logger.Get().Warn("Markdown rendering failed in rebuildViewport", "error", err)
				b.WriteString(cm.text)
			} else {
				b.WriteString("Bot: " + strings.TrimSpace(rendered))
			}
		} else {
			b.WriteString(cm.text)
		}
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

// addMessage appends a new message to history; if placeholder is true, tracks its index for replacement.
func (m *Model) addMessage(text string, isMarkdown, placeholder bool) {
	if placeholder {
		m.placeholderIndex = len(m.messages)
	}
	m.messages = append(m.messages, chatMessage{text: text, isMarkdown: isMarkdown, placeholder: placeholder})
	m.rebuildViewport()
}

// replacePlaceholder replaces the current placeholder message (if any) with real content; otherwise appends.
func (m *Model) replacePlaceholder(text string, isMarkdown bool) {
	if m.placeholderIndex >= 0 && m.placeholderIndex < len(m.messages) {
		m.messages[m.placeholderIndex] = chatMessage{text: text, isMarkdown: isMarkdown, placeholder: false}
		m.placeholderIndex = -1
	} else {
		m.messages = append(m.messages, chatMessage{text: text, isMarkdown: isMarkdown, placeholder: false})
	}
	m.rebuildViewport()
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("An error occurred: %v\nPress Esc or Ctrl+C to quit.", m.err)
	}
	loadingIndicator := ""
	if m.loading {
		loadingIndicator = " (loading...)"
	}

   // Help hint for scrolling, mouse wheel, and quitting
   help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
       "↑/↓ scroll  PgUp/PgDn page scroll  Mouse wheel scroll  Esc/Ctrl+C quit",
   )
   // Render viewport, help line, and input area
   return fmt.Sprintf(
       "%s\n%s\n\n%s%s",
       m.viewport.View(),
       help,
       m.textarea.View(),
       loadingIndicator,
   ) + "\n"
}
