package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/castrovroberto/CGE/internal/logger"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// MessageListModel manages the message list and viewport
type MessageListModel struct {
	theme            *Theme
	viewport         viewport.Model
	messages         []chatMessage
	placeholderIndex int
	renderer         *glamour.TermRenderer
	formatter        chroma.Formatter
	progressRenderer *ProgressRenderer
	activeToolCalls  map[string]*toolProgressState
	width            int
	height           int
}

// NewMessageListModel creates a new message list model
func NewMessageListModel(theme *Theme, width, height int) *MessageListModel {
	// Initialize viewport with theme styling
	vp := viewport.New(width, height)
	vp.Style = theme.ViewportBorder

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

	return &MessageListModel{
		theme:            theme,
		viewport:         vp,
		messages:         nil,
		placeholderIndex: -1,
		renderer:         renderer,
		formatter:        formatter,
		progressRenderer: NewProgressRenderer(width),
		activeToolCalls:  make(map[string]*toolProgressState),
		width:            width,
		height:           height,
	}
}

// Update handles message list updates
func (ml *MessageListModel) Update(msg tea.Msg) (*MessageListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ml.width = msg.Width
		ml.height = msg.Height

		// Update viewport dimensions
		wFrame := ml.viewport.Style.GetHorizontalFrameSize()
		ml.viewport.Width = msg.Width - wFrame
		ml.viewport.Height = msg.Height

		// Update progress renderer width
		if ml.progressRenderer != nil {
			ml.progressRenderer.width = msg.Width
		}

		// Update glamour renderer for new width
		if ml.renderer != nil {
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(ml.viewport.Width),
			)
			if err != nil {
				logger.Get().Error("Failed to re-initialize glamour renderer on resize", "error", err)
			} else {
				ml.renderer = newRenderer
				ml.rebuildViewport() // Rebuild with new width
			}
		}

	case tea.MouseMsg:
		ml.viewport, cmd = ml.viewport.Update(msg)
	}

	return ml, cmd
}

// View renders the message list
func (ml *MessageListModel) View() string {
	return ml.viewport.View()
}

// AddMessage appends a new message to history
func (ml *MessageListModel) AddMessage(msg chatMessage) {
	if msg.timestamp.IsZero() {
		msg.timestamp = time.Now()
	}

	// Process code blocks in markdown
	if msg.isMarkdown {
		msg.text = ml.processCodeBlocks(msg.text)
	}

	ml.messages = append(ml.messages, msg)

	// Update placeholder index if this is a placeholder
	if msg.placeholder {
		ml.placeholderIndex = len(ml.messages) - 1
		logger.Get().Debug("Added placeholder message", "index", ml.placeholderIndex)
	}

	ml.rebuildViewport()
}

// ReplacePlaceholder replaces the current placeholder message with real content
func (ml *MessageListModel) ReplacePlaceholder(msg chatMessage) {
	if ml.placeholderIndex >= 0 && ml.placeholderIndex < len(ml.messages) {
		logger.Get().Debug("Replacing placeholder", "index", ml.placeholderIndex, "sender", msg.sender)
		ml.messages[ml.placeholderIndex] = msg
		ml.placeholderIndex = -1
	} else {
		logger.Get().Debug("No valid placeholder to replace, appending message", "placeholderIndex", ml.placeholderIndex, "messagesLength", len(ml.messages))
		ml.messages = append(ml.messages, msg)
	}
	ml.rebuildViewport()
}

// rebuildViewport re-renders all stored messages into the viewport
func (ml *MessageListModel) rebuildViewport() {
	defer func() {
		if r := recover(); r != nil {
			logger.Get().Error("Panic in rebuildViewport", "panic", r)
			// Set fallback content
			ml.viewport.SetContent("Error rendering messages. Please restart the chat.")
		}
	}()

	var b strings.Builder
	for i, cm := range ml.messages {
		// Add index validation
		if i < 0 || i >= len(ml.messages) {
			logger.Get().Warn("Invalid message index in rebuildViewport", "index", i, "length", len(ml.messages))
			continue
		}

		// Handle tool call messages specially
		if cm.isToolCall {
			b.WriteString(ml.formatToolCall(cm))
		} else if cm.isToolResult {
			b.WriteString(ml.formatToolResult(cm))
		} else if cm.isMarkdown && ml.renderer != nil {
			// Handle all markdown consistently
			rendered, err := ml.renderer.Render(cm.text)
			if err != nil {
				logger.Get().Warn("Markdown rendering failed in rebuildViewport", "error", err)
				// Fall back to regular message formatting
				b.WriteString(ml.formatRegularMessage(cm))
			} else {
				// Format with sender and timestamp
				senderPrefix := ml.theme.Sender.Render(cm.sender + ": ")
				timestamp := ml.theme.Time.Render(cm.timestamp.Format("15:04:05"))

				if cm.ThinkingTime > 0 {
					thinkingTime := ml.theme.ThinkingTime.Render(fmt.Sprintf(" (%.2fs)", cm.ThinkingTime.Seconds()))
					b.WriteString(fmt.Sprintf("%s %s%s\n%s", senderPrefix, timestamp, thinkingTime, strings.TrimSpace(rendered)))
				} else {
					b.WriteString(fmt.Sprintf("%s %s\n%s", senderPrefix, timestamp, strings.TrimSpace(rendered)))
				}
			}
		} else {
			// Regular message formatting
			b.WriteString(ml.formatRegularMessage(cm))
		}
		b.WriteString("\n\n") // Add spacing between messages
	}

	// Add active progress bars at the bottom
	if len(ml.activeToolCalls) > 0 {
		b.WriteString("\n" + strings.Repeat("─", ml.progressRenderer.width) + "\n")
		b.WriteString("🔄 Active Operations:\n\n")

		for _, state := range ml.activeToolCalls {
			progressDisplay := ml.progressRenderer.RenderProgress(state)
			b.WriteString(progressDisplay + "\n\n")
		}
	}

	ml.viewport.SetContent(b.String())
	ml.viewport.GotoBottom()
}

// formatRegularMessage formats a regular message
func (ml *MessageListModel) formatRegularMessage(cm chatMessage) string {
	if cm.sender != "" && cm.text != "" {
		senderPrefix := ml.theme.Sender.Render(cm.sender + ": ")
		timestamp := ml.theme.Time.Render(cm.timestamp.Format("15:04:05"))

		if cm.ThinkingTime > 0 {
			thinkingTime := ml.theme.ThinkingTime.Render(fmt.Sprintf(" (%.2fs)", cm.ThinkingTime.Seconds()))
			return fmt.Sprintf("%s %s%s\n%s", senderPrefix, timestamp, thinkingTime, cm.text)
		} else {
			return fmt.Sprintf("%s %s\n%s", senderPrefix, timestamp, cm.text)
		}
	}
	return cm.text
}

// formatToolCall formats a tool call message for display
func (ml *MessageListModel) formatToolCall(msg chatMessage) string {
	var parts []string

	// Tool call header with icon
	header := fmt.Sprintf("🔧 Tool Call: %s", msg.toolName)
	if msg.toolCallID != "" {
		header += fmt.Sprintf(" (ID: %s)", msg.toolCallID[:8]) // Show first 8 chars of ID
	}
	parts = append(parts, ml.theme.ToolCall.Render(header))

	// Tool parameters (if any)
	if len(msg.toolParams) > 0 {
		paramsJSON, _ := json.MarshalIndent(msg.toolParams, "", "  ")
		paramText := fmt.Sprintf("Parameters:\n%s", string(paramsJSON))
		parts = append(parts, ml.theme.ToolParams.Render(paramText))
	}

	return strings.Join(parts, "\n")
}

// formatToolResult formats a tool result message for display
func (ml *MessageListModel) formatToolResult(msg chatMessage) string {
	var parts []string

	// Result header with status icon
	var icon, header string
	var style = ml.theme.ToolResult

	if msg.toolSuccess {
		icon = "✅"
		header = fmt.Sprintf("%s Tool Result: %s", icon, msg.toolName)
		style = ml.theme.ToolSuccess
	} else {
		icon = "❌"
		header = fmt.Sprintf("%s Tool Error: %s", icon, msg.toolName)
		style = ml.theme.ToolError
	}

	if msg.toolDuration > 0 {
		header += fmt.Sprintf(" (%.2fs)", msg.toolDuration.Seconds())
	}

	parts = append(parts, style.Render(header))

	// Result content
	if msg.text != "" {
		resultContent := ml.theme.ToolResult.Render(msg.text)
		parts = append(parts, resultContent)
	}

	return strings.Join(parts, "\n")
}

// processCodeBlocks handles syntax highlighting of code blocks in markdown
func (ml *MessageListModel) processCodeBlocks(text string) string {
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
				highlighted := ml.highlightCode(code, language)
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
func (ml *MessageListModel) highlightCode(code, language string) string {
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
	err = ml.formatter.Format(&buf, style, iterator)
	if err != nil {
		return code
	}

	return ml.theme.Code.Render(buf.String())
}

// SetHeight sets the viewport height
func (ml *MessageListModel) SetHeight(height int) {
	ml.height = height
	ml.viewport.Height = height
}

// GetHeight returns the current viewport height
func (ml *MessageListModel) GetHeight() int {
	return ml.viewport.Height
}

// GotoBottom scrolls to the bottom
func (ml *MessageListModel) GotoBottom() {
	ml.viewport.GotoBottom()
}

// LoadHistory loads messages from chat history
func (ml *MessageListModel) LoadHistory(messages []chatMessage) {
	ml.messages = messages
	ml.rebuildViewport()
}

// GetMessages returns the current messages
func (ml *MessageListModel) GetMessages() []chatMessage {
	return ml.messages
}

// SetActiveToolCalls sets the active tool calls for progress tracking
func (ml *MessageListModel) SetActiveToolCalls(activeToolCalls map[string]*toolProgressState) {
	ml.activeToolCalls = activeToolCalls
	ml.rebuildViewport() // Rebuild to show progress
}
