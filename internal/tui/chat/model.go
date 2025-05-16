// internal/tui/chat/model.go
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Bubbles components for TUI
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/castrovroberto/codex-lite/internal/agent"
	"github.com/castrovroberto/codex-lite/internal/config" // Ensure this path and package are correct
	"github.com/castrovroberto/codex-lite/internal/logger" // Import logger to get the global logger
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type (
	errMsg            error
	ollamaResponseMsg string
	ollamaErrorMsg    error
)

// chatMessage holds a single chat entry for re-rendering
type chatMessage struct {
	text        string
	isMarkdown  bool
	isCode      bool   // New: specifically for code blocks
	language    string // New: for syntax highlighting
	timestamp   time.Time
	sender      string
	placeholder bool
}

// Model defines the state of the chat TUI
type Model struct {
	// Header information
	headerStyle lipgloss.Style
	provider    string
	sessionID   string
	modelName   string
	status      string // New: connection status

	// Loading spinner
	spin spinner.Model

	// Chat window components
	viewport    viewport.Model
	textarea    textarea.Model
	suggestions []string // New: for auto-completion
	selected    int      // New: selected suggestion

	// Styles
	senderStyle     lipgloss.Style
	errorStyle      lipgloss.Style
	codeStyle       lipgloss.Style // New: for code blocks
	timeStyle       lipgloss.Style // New: for timestamps
	statusStyle     lipgloss.Style // New: for status bar
	suggestionStyle lipgloss.Style // New: for suggestions

	// Context and config
	cfg       *config.AppConfig
	parentCtx context.Context

	// Error and loading state
	err     error
	loading bool

	// Markdown and syntax highlighting
	renderer  *glamour.TermRenderer
	formatter chroma.Formatter // Updated: correct type for syntax highlighting

	// Chat history
	messages         []chatMessage
	placeholderIndex int
	editingIndex     int  // New: for message editing
	isEditing        bool // New: editing state

	// Window dimensions
	width  int
	height int

	// New: Add tool registry
	toolRegistry *agent.Registry
}

func InitialModel(ctx context.Context, cfg *config.AppConfig, modelName string) Model {
	// Initialize textarea with better styling
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+E to edit last, Tab for completion)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 2000 // Increased limit
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	// Initialize viewport with better styling
	vp := viewport.New(50, 10)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	// Initialize glamour renderer for markdown
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(vp.Width),
	)
	if err != nil {
		logger.Get().Error("Failed to initialize glamour markdown renderer", "error", err)
		renderer = nil
	}

	// Initialize syntax highlighting formatter
	formatter := formatters.TTY256

	// Initialize styles
	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("63")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	senderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("63")).
		Bold(true)

	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	codeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("237")).
		Foreground(lipgloss.Color("252"))

	suggestionStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("237")).
		Foreground(lipgloss.Color("252"))

	// Setup loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	// Create model
	m := Model{
		headerStyle:      headerStyle,
		provider:         "Ollama",
		sessionID:        time.Now().Format("2006-01-02 15:04:05"),
		modelName:        modelName,
		status:           "Connected",
		spin:             sp,
		textarea:         ta,
		viewport:         vp,
		senderStyle:      senderStyle,
		timeStyle:        timeStyle,
		codeStyle:        codeStyle,
		statusStyle:      statusStyle,
		suggestionStyle:  suggestionStyle,
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		cfg:              cfg,
		parentCtx:        ctx,
		renderer:         renderer,
		formatter:        formatter,
		messages:         nil,
		placeholderIndex: -1,
		editingIndex:     -1,
		isEditing:        false,
	}

	// Initialize tool registry
	registry := agent.NewRegistry()

	// Register code analysis tools
	if err := registry.Register(agent.NewCodeSearchTool(cfg.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register code search tool", "error", err)
	}
	if err := registry.Register(agent.NewFileReadTool(cfg.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register file read tool", "error", err)
	}

	// Add registry to model
	m.toolRegistry = registry

	// Add welcome message
	m.addMessage(chatMessage{
		text:      "Welcome to Codex Lite Chat! Type your message and press Enter. Use Ctrl+E to edit your last message, Tab for completion.",
		timestamp: time.Now(),
		sender:    "System",
	})

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

		// Prepare tool descriptions for the agent
		var toolDescriptions []map[string]interface{}
		for _, tool := range m.toolRegistry.List() {
			toolDescriptions = append(toolDescriptions, map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Parameters(),
			})
		}

		// Add tool descriptions to the prompt
		systemPrompt := fmt.Sprintf(`You are an AI assistant with access to the following tools:
%s

To use a tool, respond with a JSON object in this format:
{
  "tool": "tool_name",
  "params": {
    // tool-specific parameters
  }
}

Only respond with valid JSON when you want to use a tool. Otherwise, respond normally.`, formatToolDescriptions(toolDescriptions))

		// Add system prompt to the conversation context
		response, err := ollama.Query(ctx, m.cfg.OllamaHostURL, m.modelName, fmt.Sprintf("%s\n\nUser: %s", systemPrompt, prompt))
		if err != nil {
			return ollamaErrorMsg(fmt.Errorf("ollama query failed: %w", err))
		}
		return ollamaResponseMsg(response)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle spinner ticks for loading state
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	}
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

			m.addMessage(chatMessage{
				text:      userInput,
				sender:    "You",
				timestamp: time.Now(),
			})

			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.loading = true

			// Add thinking message
			m.addMessage(chatMessage{
				text:        "Thinking...",
				sender:      "Bot",
				timestamp:   time.Now(),
				placeholder: true,
			})
			m.viewport.GotoBottom()

			// Start the spinner and fetch the response concurrently
			return m, tea.Batch(
				m.fetchOllamaResponse(userInput),
				spinner.Tick,
			)
		case tea.KeyCtrlE:
			m.startEditing()
			return m, nil
		case tea.KeyTab:
			if len(m.suggestions) > 0 {
				m.selected = (m.selected + 1) % len(m.suggestions)
				return m, nil
			}
		}

	case ollamaResponseMsg:
		m.loading = false
		botResponse := string(msg)
		m.replacePlaceholder(chatMessage{
			text:       botResponse,
			sender:     "Bot",
			timestamp:  time.Now(),
			isMarkdown: true,
		})
		return m, nil

	case ollamaErrorMsg:
		m.loading = false
		errorMsg := m.errorStyle.Render(fmt.Sprintf("Error: %v", msg))
		m.replacePlaceholder(chatMessage{
			text:      errorMsg,
			sender:    "System",
			timestamp: time.Now(),
		})
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

	// New: Check if the response is a tool invocation
	case json.RawMessage:
		var toolInvocation struct {
			Tool   string          `json:"tool"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal([]byte(msg), &toolInvocation); err == nil && toolInvocation.Tool != "" {
			// Execute tool
			if tool, ok := m.toolRegistry.Get(toolInvocation.Tool); ok {
				result, err := tool.Execute(m.parentCtx, toolInvocation.Params)
				if err != nil {
					// Handle error
					m.addMessage(chatMessage{
						text:      fmt.Sprintf("Error executing tool: %v", err),
						timestamp: time.Now(),
						sender:    "System",
					})
				} else {
					// Format and display result
					resultJSON, _ := json.MarshalIndent(result, "", "  ")
					m.addMessage(chatMessage{
						text:      fmt.Sprintf("Tool result:\n```json\n%s\n```", resultJSON),
						timestamp: time.Now(),
						sender:    "System",
					})
				}
			}
		}
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
func (m *Model) addMessage(msg chatMessage) {
	if msg.timestamp.IsZero() {
		msg.timestamp = time.Now()
	}

	// Process code blocks in markdown
	if msg.isMarkdown {
		msg.text = m.processCodeBlocks(msg.text)
	}

	m.messages = append(m.messages, msg)
	m.rebuildViewport()
}

// replacePlaceholder replaces the current placeholder message (if any) with real content; otherwise appends.
func (m *Model) replacePlaceholder(msg chatMessage) {
	if m.placeholderIndex >= 0 && m.placeholderIndex < len(m.messages) {
		m.messages[m.placeholderIndex] = msg
		m.placeholderIndex = -1
	} else {
		m.messages = append(m.messages, msg)
	}
	m.rebuildViewport()
}

// processCodeBlocks handles syntax highlighting of code blocks in markdown
func (m *Model) processCodeBlocks(text string) string {
	// Split the text into lines
	lines := strings.Split(text, "\n")
	var result strings.Builder
	var codeBlock strings.Builder
	inCodeBlock := false
	var language string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				// Start of code block
				inCodeBlock = true
				language = strings.TrimPrefix(line, "```")
				continue
			} else {
				// End of code block
				inCodeBlock = false
				// Process the collected code block
				code := codeBlock.String()
				highlighted := m.highlightCode(code, language)
				result.WriteString(highlighted)
				result.WriteString("\n")
				codeBlock.Reset()
				continue
			}
		}

		if inCodeBlock {
			codeBlock.WriteString(line)
			codeBlock.WriteString("\n")
		} else {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// highlightCode applies syntax highlighting to a code block
func (m *Model) highlightCode(code, language string) string {
	// Get lexer for the language
	l := lexers.Get(language)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	// Tokenize the code
	iterator, err := l.Tokenise(nil, code)
	if err != nil {
		return code // Return original code if highlighting fails
	}

	// Apply highlighting
	var buf strings.Builder
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	err = m.formatter.Format(&buf, style, iterator)
	if err != nil {
		return code
	}

	return m.codeStyle.Render(buf.String())
}

// New: Handle message editing
func (m *Model) startEditing() {
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].sender == "You" {
		m.isEditing = true
		m.editingIndex = len(m.messages) - 1
		m.textarea.SetValue(m.messages[m.editingIndex].text)
	}
}

// New: Handle auto-completion
func (m *Model) updateSuggestions(input string) {
	// Implementation for context-aware suggestions
	// This could include command completion, code completion, etc.
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("An error occurred: %v\nPress Esc or Ctrl+C to quit.", m.err)
	}

	// Header with model, provider, and session info
	header := fmt.Sprintf(" Model: %s | Provider: %s | Session: %s | Status: %s ",
		m.modelName, m.provider, m.sessionID, m.status)
	headerView := m.headerStyle.Render(header)

	// Rebuild viewport content with improved message formatting
	var content strings.Builder
	for _, msg := range m.messages {
		// Format timestamp
		timestamp := m.timeStyle.Render(msg.timestamp.Format("15:04:05"))

		// Format sender
		var senderText string
		switch msg.sender {
		case "You":
			senderText = m.senderStyle.Render("You")
		case "Bot":
			senderText = m.senderStyle.Render("Bot")
		case "System":
			senderText = m.senderStyle.Render("System")
		}

		// Add message header
		content.WriteString(fmt.Sprintf("%s %s:\n", timestamp, senderText))

		// Process message content
		if msg.isMarkdown {
			// For markdown content (including code blocks)
			rendered, err := m.renderer.Render(msg.text)
			if err != nil {
				content.WriteString(msg.text)
			} else {
				content.WriteString(strings.TrimSpace(rendered))
			}
		} else {
			// For plain text
			content.WriteString(msg.text)
		}
		content.WriteString("\n\n")
	}

	// Set viewport content
	m.viewport.SetContent(content.String())

	// Help text
	var help strings.Builder
	help.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"↑/↓: scroll • ",
	))
	help.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Ctrl+C: quit • ",
	))
	help.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Ctrl+E: edit last • ",
	))
	help.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Tab: completion",
	))

	// Input area with spinner if loading
	input := m.textarea.View()
	if m.loading {
		input = lipgloss.JoinHorizontal(lipgloss.Center,
			input,
			" ",
			m.spin.View(),
			m.statusStyle.Render(" Thinking..."),
		)
	}

	// Show suggestions if any
	var suggestions string
	if len(m.suggestions) > 0 && !m.loading {
		var suggList []string
		for i, s := range m.suggestions {
			if i == m.selected {
				suggList = append(suggList, m.suggestionStyle.Copy().Bold(true).Render(s))
			} else {
				suggList = append(suggList, m.suggestionStyle.Render(s))
			}
		}
		suggestions = "\n" + strings.Join(suggList, " ")
	}

	// Combine all components
	return lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		"",
		m.viewport.View(),
		"",
		help.String(),
		input,
		suggestions,
	)
}

// LoadHistory loads a previous chat history into the model
func (m *Model) LoadHistory(history *ChatHistory) {
	m.sessionID = history.SessionID
	m.modelName = history.ModelName
	m.messages = history.Messages

	// Rebuild the viewport with the loaded messages
	m.rebuildViewport()
}

// New: Helper function to format tool descriptions
func formatToolDescriptions(tools []map[string]interface{}) string {
	var sb strings.Builder
	for _, tool := range tools {
		fmt.Fprintf(&sb, "\n%s: %s\nParameters: %s\n",
			tool["name"], tool["description"], tool["parameters"])
	}
	return sb.String()
}
