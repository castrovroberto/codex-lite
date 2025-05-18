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
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/castrovroberto/codex-lite/internal/logger" // Import logger to get the global logger
	"github.com/castrovroberto/codex-lite/internal/ollama"
)

type (
	errMsg error
	// ollamaResponseMsg string // This line should be removed or commented out
	ollamaErrorMsg error
	// New message type to carry successful response and duration
	ollamaSuccessResponseMsg struct {
		response string
		duration time.Duration
	}
)

// chatMessage holds a single chat entry for re-rendering
type chatMessage struct {
	text         string
	isMarkdown   bool
	isCode       bool   // New: specifically for code blocks
	language     string // New: for syntax highlighting
	timestamp    time.Time
	sender       string
	placeholder  bool
	ThinkingTime time.Duration // New field for LLM thinking time
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
	senderStyle       lipgloss.Style
	errorStyle        lipgloss.Style
	codeStyle         lipgloss.Style // New: for code blocks
	timeStyle         lipgloss.Style // New: for timestamps
	statusStyle       lipgloss.Style // New: for status bar
	suggestionStyle   lipgloss.Style // New: for suggestions
	thinkingTimeStyle lipgloss.Style // New style for thinking time

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

	thinkingStartTime time.Time // To track when Ollama request started for live timer
	chatStartTime     time.Time // New: To track the actual start of the chat session

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

	thinkingTimeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")) // New style for thinking time

	// Setup loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	// Create model
	m := Model{
		headerStyle:       headerStyle,
		provider:          "Ollama",
		sessionID:         time.Now().Format("20060102150405"),
		modelName:         modelName,
		status:            "Connected",
		spin:              sp,
		textarea:          ta,
		viewport:          vp,
		senderStyle:       senderStyle,
		timeStyle:         timeStyle,
		codeStyle:         codeStyle,
		statusStyle:       statusStyle,
		suggestionStyle:   suggestionStyle,
		errorStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		thinkingTimeStyle: thinkingTimeStyle,
		cfg:               cfg,
		parentCtx:         ctx,
		renderer:          renderer,
		formatter:         formatter,
		messages:          nil,
		placeholderIndex:  -1,
		editingIndex:      -1,
		isEditing:         false,
		chatStartTime:     time.Now(),
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
	if err := registry.Register(agent.NewCodebaseAnalyzeTool(cfg.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register codebase analysis tool", "error", err)
	}
	if err := registry.Register(agent.NewGitTool(cfg.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register Git tool", "error", err)
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
		startTime := time.Now()

		baseCtx := m.parentCtx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctxWithValues := context.WithValue(baseCtx, contextkeys.ConfigKey, m.cfg)
		ctxWithValues = context.WithValue(ctxWithValues, contextkeys.LoggerKey, logger.Get())

		ctx, cancel := context.WithTimeout(ctxWithValues, m.cfg.OllamaRequestTimeout+5*time.Second)
		defer cancel()

		// Base system prompt from configuration (content loaded from file via getter)
		baseSystemPrompt := m.cfg.GetLoadedChatSystemPrompt() // Use the getter method

		// Tool descriptions (if any tools are registered)
		var toolDescriptions []map[string]interface{}
		toolSystemPromptSegment := ""
		if m.toolRegistry != nil && len(m.toolRegistry.List()) > 0 {
			for _, tool := range m.toolRegistry.List() {
				toolDescriptions = append(toolDescriptions, map[string]interface{}{
					"name":        tool.Name(),
					"description": tool.Description(),
					"parameters":  tool.Parameters(),
				})
			}
			toolSystemPromptSegment = fmt.Sprintf(`You have access to the following tools:
%s

To use a tool, respond ONLY with a JSON object in this exact format:
{
  "tool": "tool_name",
  "params": {
    // tool-specific parameters here
  }
}
If you do not need to use a tool, respond to the user directly without any JSON.`, formatToolDescriptions(toolDescriptions))
		}

		// Combine system prompts
		finalSystemPrompt := baseSystemPrompt
		if toolSystemPromptSegment != "" {
			if finalSystemPrompt != "" {
				finalSystemPrompt += "\n\n" + toolSystemPromptSegment
			} else {
				finalSystemPrompt = toolSystemPromptSegment
			}
		}

		// Prepend the final combined system prompt to the user prompt for Ollama
		fullPrompt := prompt
		if finalSystemPrompt != "" {
			fullPrompt = finalSystemPrompt + "\n\nUser: " + prompt
		}

		response, err := ollama.Query(ctx, m.cfg.OllamaHostURL, m.modelName, fullPrompt)
		duration := time.Since(startTime)

		if err != nil {
			return ollamaErrorMsg(fmt.Errorf("ollama query failed: %w", err))
		}
		return ollamaSuccessResponseMsg{response: response, duration: duration}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Adjust for viewport border/frame
		wFrame := m.viewport.Style.GetHorizontalFrameSize()
		hFrame := m.viewport.Style.GetVerticalFrameSize()
		m.viewport.Width = msg.Width - wFrame
		m.viewport.Height = msg.Height - m.textarea.Height() - 1 - hFrame // -1 for the status bar line
		m.textarea.SetWidth(msg.Width)

		if m.renderer != nil {
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
			if err != nil {
				logger.Get().Error("Failed to re-initialize glamour markdown renderer on resize", "error", err)
			} else {
				m.renderer = newRenderer
			}
			m.rebuildViewport() // Important to apply new width
		}
		// Bubbles also need to be updated with WindowSizeMsg
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save history before quitting
			if err := m.SaveHistory(); err != nil {
				// Log the error or display it. For now, let's use the model's error field.
				// This error won't be visible long as the app quits immediately.
				// Consider logging to a file or stderr for better visibility of save errors.
				log := logger.Get() // Assuming logger.Get() is accessible
				log.Error("Failed to save chat history on exit", "sessionID", m.sessionID, "error", err)
				m.err = fmt.Errorf("failed to save history: %w", err) // Sets error on model, though app quits
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.textarea.Value() != "" && !m.loading {
				m.thinkingStartTime = time.Now() // Set start time for live timer
				m.loading = true
				userPrompt := m.textarea.Value()
				m.addMessage(chatMessage{text: userPrompt, sender: "You", timestamp: time.Now()})
				m.textarea.Reset()
				m.viewport.GotoBottom()
				m.addMessage(chatMessage{text: "...", sender: "AI", timestamp: time.Now(), placeholder: true})
				m.placeholderIndex = len(m.messages) - 1
				cmds = append(cmds, m.spin.Tick, m.fetchOllamaResponse(userPrompt))
			} else {
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
		case tea.KeyCtrlE:
			m.startEditing()
			// After starting to edit, the textarea has the old message.
			// Pass the event to textarea in case it has specific Ctrl+E handling or for consistency.
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		case tea.KeyTab:
			if len(m.suggestions) > 0 {
				m.selected = (m.selected + 1) % len(m.suggestions)
				// Tab consumed for suggestion cycling, do not pass to textarea
			} else {
				// If no suggestions, let textarea handle Tab
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
		default: // Crucial for typing, backspace, arrows within textarea
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}

	case spinner.TickMsg:
		if m.loading {
			m.spin, cmd = m.spin.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ollamaSuccessResponseMsg:
		m.loading = false
		botResponseText := msg.response
		responseTime := msg.duration
		m.replacePlaceholder(chatMessage{
			text:         botResponseText,
			sender:       "AI",
			timestamp:    time.Now(),
			ThinkingTime: responseTime,
			isMarkdown:   true,
		})
		m.viewport.GotoBottom()

	case ollamaErrorMsg:
		m.loading = false
		errorMsg := m.errorStyle.Render(fmt.Sprintf("Error: %v", msg))
		m.replacePlaceholder(chatMessage{
			text:      errorMsg,
			sender:    "System",
			timestamp: time.Now(),
		})

	case errMsg:
		m.err = msg
		// Potentially update status or log

	// New: Check if the response is a tool invocation (assuming this was intended from previous structure)
	// This case might need to be reviewed if it's from an ollamaSuccessResponseMsg.text
	case json.RawMessage: // This case needs careful placement. Is it a primary msg type or derived from another?
		var toolInvocation struct {
			Tool   string          `json:"tool"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal([]byte(msg), &toolInvocation); err == nil && toolInvocation.Tool != "" {
			if tool, ok := m.toolRegistry.Get(toolInvocation.Tool); ok {
				result, toolErr := tool.Execute(m.parentCtx, toolInvocation.Params)
				if toolErr != nil {
					m.addMessage(chatMessage{
						text:      fmt.Sprintf("Error executing tool %s: %v", toolInvocation.Tool, toolErr),
						timestamp: time.Now(),
						sender:    "System",
					})
				} else {
					resultJSON, _ := json.MarshalIndent(result, "", "  ")
					m.addMessage(chatMessage{
						text:       fmt.Sprintf("Tool %s result:\n```json\n%s\n```", toolInvocation.Tool, resultJSON),
						timestamp:  time.Now(),
						sender:     "System",
						isMarkdown: true, // Render result as markdown with code block
					})
				}
			} else {
				m.addMessage(chatMessage{
					text:      fmt.Sprintf("Unknown tool requested: %s", toolInvocation.Tool),
					timestamp: time.Now(),
					sender:    "System",
				})
			}
		} else {
			// If json.RawMessage was not a tool invocation, it might be an error or unhandled. Consider logging.
			// Or, if this type of message is not expected directly, this case might be removed or refined.
		}
	}

	// If viewport needs to react to any other messages (e.g., mouse events not directly handled)
	// m.viewport, cmd = m.viewport.Update(msg) // Be careful not to double-update on WindowSizeMsg
	// cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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
	var view strings.Builder

	// Header
	header := m.headerStyle.Render(fmt.Sprintf("Chat with %s (%s) | Session: %s | Status: %s",
		m.provider, m.modelName, m.sessionID, m.status))
	view.WriteString(header)
	view.WriteString("\n")

	// Messages
	var renderedMessages []string
	for _, msg := range m.messages {
		var sender string
		var content string

		sender = m.senderStyle.Render(msg.sender + ":")
		timestamp := m.timeStyle.Render(msg.timestamp.Format("15:04:05"))

		// Add thinking time if available (for AI messages)
		thinkingTimeStr := ""
		if msg.sender == "AI" && msg.ThinkingTime > 0 {
			thinkingTimeStr = m.thinkingTimeStyle.Render(fmt.Sprintf(" (took %.2fs)", msg.ThinkingTime.Seconds()))
		}

		if msg.placeholder {
			content = msg.text // Placeholder text, no special rendering
		} else if msg.isMarkdown {
			// Process for code blocks first
			processedText := m.processCodeBlocks(msg.text)
			renderedMarkdown, err := m.renderer.Render(processedText)
			if err != nil {
				content = m.errorStyle.Render("Failed to render markdown: " + err.Error())
			} else {
				content = renderedMarkdown
			}
		} else if msg.isCode {
			highlightedCode := m.highlightCode(msg.text, msg.language)
			content = m.codeStyle.Render(highlightedCode)
		} else {
			content = msg.text // Plain text
		}

		// Assemble the message line
		// Check if content is multi-line (often true for markdown/code)
		if strings.Contains(content, "\n") {
			renderedMessages = append(renderedMessages, fmt.Sprintf("%s %s%s\n%s", sender, timestamp, thinkingTimeStr, content))
		} else {
			renderedMessages = append(renderedMessages, fmt.Sprintf("%s %s%s %s", sender, timestamp, thinkingTimeStr, content))
		}
	}

	m.viewport.SetContent(strings.Join(renderedMessages, "\n"))

	// Viewport and Textarea
	view.WriteString(m.viewport.View())
	view.WriteString("\n")
	view.WriteString(m.textarea.View())

	// Status bar (optional, can show loading state, suggestions, etc.)
	statusBar := ""
	if m.loading {
		elapsed := time.Since(m.thinkingStartTime)
		// Format elapsed time, e.g., to one decimal place for seconds
		elapsedStr := fmt.Sprintf("%.1fs", elapsed.Seconds())
		statusBar = m.statusStyle.Render(fmt.Sprintf("%s Thinking... (%s)", m.spin.View(), elapsedStr))
	} else if m.err != nil {
		statusBar = m.errorStyle.Render("Error: " + m.err.Error())
	} else {
		statusBar = m.statusStyle.Render("Ctrl+C to quit. Ctrl+E to edit last. Tab for suggestions.")
	}
	view.WriteString("\n")
	view.WriteString(statusBar)

	return view.String()
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
