// internal/tui/chat/model.go
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	// Bubbles components for TUI

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config" // Ensure this path and package are correct
	"github.com/castrovroberto/CGE/internal/llm"    // Import the new llm package
	"github.com/castrovroberto/CGE/internal/logger" // Import logger to get the global logger
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

// Add a new message type for main context cancellation
type mainContextCancelledMsg struct{}

// Progress tracking message types
type toolProgressMsg struct {
	toolCallID string
	toolName   string
	progress   float64 // 0.0 to 1.0
	status     string  // Current status description
	step       int     // Current step number
	totalSteps int     // Total number of steps
}

type toolStartMsg struct {
	toolCallID string
	toolName   string
	params     map[string]interface{}
}

type toolCompleteMsg struct {
	toolCallID string
	toolName   string
	success    bool
	result     string
	duration   time.Duration
	error      string
}

// toolProgressState tracks the progress of an active tool call
type toolProgressState struct {
	toolName     string
	startTime    time.Time
	progress     float64
	status       string
	step         int
	totalSteps   int
	messageIndex int // Index in messages array for updating
}

// ProgressRenderer handles rendering of progress bars and status
type ProgressRenderer struct {
	width int
}

// NewProgressRenderer creates a new progress renderer
func NewProgressRenderer(width int) *ProgressRenderer {
	return &ProgressRenderer{width: width}
}

// RenderProgress renders a progress bar with status
func (pr *ProgressRenderer) RenderProgress(state *toolProgressState) string {
	if state == nil {
		return ""
	}

	// Progress bar
	barWidth := pr.width - 20 // Leave space for text
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(state.progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Status text
	elapsed := time.Since(state.startTime)
	statusText := fmt.Sprintf("🔄 %s", state.toolName)

	if state.totalSteps > 0 {
		statusText += fmt.Sprintf(" (%d/%d)", state.step, state.totalSteps)
	}

	statusText += fmt.Sprintf(" %.1f%% - %s", state.progress*100, state.status)
	statusText += fmt.Sprintf(" (%.1fs)", elapsed.Seconds())

	return fmt.Sprintf("%s\n[%s]", statusText, bar)
}

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

	// Enhanced tool call support
	isToolCall   bool                   // New: indicates this is a tool call message
	isToolResult bool                   // New: indicates this is a tool result message
	toolName     string                 // New: name of the tool being called/result from
	toolCallID   string                 // New: unique ID for tool call correlation
	toolSuccess  bool                   // New: whether tool execution was successful
	toolDuration time.Duration          // New: how long the tool took to execute
	toolParams   map[string]interface{} // New: tool parameters for display
}

// Model defines the state of the chat TUI
type Model struct {
	// Component models
	theme       *Theme
	layout      *LayoutDimensions
	header      *HeaderModel
	messageList *MessageListModel
	inputArea   *InputAreaModel
	statusBar   *StatusBarModel

	// Context and config
	cfg       *config.AppConfig
	parentCtx context.Context

	// Business logic components
	toolRegistry *agent.Registry
	llmClient    llm.Client

	// State management
	loading           bool
	thinkingStartTime time.Time
	chatStartTime     time.Time

	// Available slash commands for suggestions
	availableCommands []string

	// Progress tracking
	activeToolCalls map[string]*toolProgressState
}

var defaultSlashCommands = []string{
	"/help",
	"/model ", // Suggest space for model name
	"/clear",
	"/session ", // Suggest space for session id or action
	"/status",   // Show current status and statistics
	"/tools",    // List available tools
	"/quit",
}

func InitialModel(ctx context.Context, cfg *config.AppConfig, modelName string) Model {
	// Initialize theme and layout
	theme := NewDefaultTheme()
	layout := NewLayoutDimensions(theme)

	// Initialize chat start time
	chatStartTime := time.Now()
	sessionID := chatStartTime.Format("20060102150405")

	// Initialize component models
	header := NewHeaderModel(theme, "Ollama", modelName, sessionID, "Connected")
	statusBar := NewStatusBarModel(theme, chatStartTime)
	inputArea := NewInputAreaModel(theme, defaultSlashCommands)
	messageList := NewMessageListModel(theme, 50, 10)

	// Create model
	m := Model{
		theme:             theme,
		layout:            layout,
		header:            header,
		messageList:       messageList,
		inputArea:         inputArea,
		statusBar:         statusBar,
		cfg:               cfg,
		parentCtx:         ctx,
		loading:           false,
		chatStartTime:     chatStartTime,
		availableCommands: defaultSlashCommands,
		activeToolCalls:   make(map[string]*toolProgressState),
	}

	// Add welcome message
	welcomeMsg := chatMessage{
		text:       "Welcome to CGE Chat! Type your message below or use '/' for commands.",
		sender:     "System",
		timestamp:  time.Now(),
		isMarkdown: false,
	}
	m.messageList.AddMessage(welcomeMsg)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.statusBar.GetSpinnerTickCmd(),
	)
}

func (m Model) fetchOllamaResponse(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Simulate LLM response for now
		time.Sleep(1 * time.Second)
		return ollamaSuccessResponseMsg{
			response: "This is a simulated response from the LLM.",
			duration: 1 * time.Second,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update all components
	var cmd tea.Cmd
	m.header, cmd = m.header.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.statusBar, cmd = m.statusBar.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.messageList, cmd = m.messageList.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Handle main model logic
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Calculate viewport height using layout dimensions
		textareaHeight := m.inputArea.GetHeight()
		suggestionAreaHeight := m.inputArea.GetSuggestionAreaHeight()
		viewportFrameHeight := 2 // Border frame

		viewportHeight := m.layout.CalculateViewportHeight(
			msg.Height,
			textareaHeight,
			suggestionAreaHeight,
			viewportFrameHeight,
		)

		m.messageList.SetHeight(viewportHeight)

		// Update input area
		m.inputArea, cmd = m.inputArea.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		// Handle key messages
		switch msg.String() {
		case "ctrl+c":
			logger.Get().Info("Ctrl+C pressed, attempting to save chat history and quit TUI.")
			if err := m.SaveHistory(); err != nil {
				m.statusBar.SetError(fmt.Errorf("error saving history on Ctrl+C: %w", err))
				logger.Get().Error("Failed to save chat history on Ctrl+C", "error", err)
			} else {
				logger.Get().Info("Chat history saved successfully on Ctrl+C.")
			}
			return m, tea.Quit

		case "tab":
			if m.inputArea.ApplySelectedSuggestion() {
				// Suggestion was applied, don't pass to input area
				return m, nil
			}
			// No suggestion to apply, let input area handle it
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "escape":
			if m.inputArea.HasSuggestions() {
				m.inputArea.ClearSuggestions()
				return m, nil
			}
			// No suggestions to clear, handle as normal escape
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "up":
			if m.inputArea.HandleSuggestionNavigation("up") {
				// Navigation handled by input area
				return m, nil
			}
			// No suggestions, let input area handle normally
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "down":
			if m.inputArea.HandleSuggestionNavigation("down") {
				// Navigation handled by input area
				return m, nil
			}
			// No suggestions, let input area handle normally
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "enter":
			// If suggestions are active and one is selected, apply it first
			if m.inputArea.ApplySelectedSuggestion() {
				// Suggestion was applied, don't send message yet
				return m, nil
			}

			if m.inputArea.GetValue() != "" && !m.loading {
				m.thinkingStartTime = time.Now()
				m.loading = true
				m.statusBar.SetLoading(true)

				userPrompt := m.inputArea.GetValue()
				m.messageList.AddMessage(chatMessage{
					text:      userPrompt,
					sender:    "You",
					timestamp: time.Now(),
				})

				m.inputArea.Reset()
				return m, tea.Batch(m.fetchOllamaResponse(userPrompt), m.statusBar.GetSpinnerTickCmd())
			}

		default:
			// Pass other keys to input area
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case ollamaSuccessResponseMsg:
		m.loading = false
		m.statusBar.SetLoading(false)
		m.statusBar.ClearError()

		responseMsg := chatMessage{
			text:         msg.response,
			sender:       "Assistant",
			timestamp:    time.Now(),
			isMarkdown:   true,
			ThinkingTime: msg.duration,
		}
		m.messageList.ReplacePlaceholder(responseMsg)

	case ollamaErrorMsg:
		m.loading = false
		m.statusBar.SetLoading(false)
		m.statusBar.SetError(msg)

		errorMsg := chatMessage{
			text:      fmt.Sprintf("Error: %v", msg),
			sender:    "System",
			timestamp: time.Now(),
		}
		m.messageList.ReplacePlaceholder(errorMsg)

	case errMsg:
		m.statusBar.SetError(msg)

	// Tool call message handlers
	case toolStartMsg:
		logger.Get().Info("Tool call started", "toolCallID", msg.toolCallID, "toolName", msg.toolName)

		// Create new tool progress state
		m.activeToolCalls[msg.toolCallID] = &toolProgressState{
			toolName:     msg.toolName,
			startTime:    time.Now(),
			progress:     0.0,
			status:       "Starting...",
			step:         1,
			totalSteps:   1,
			messageIndex: -1, // Will be set when message is added
		}

		// Add tool call message to chat
		toolCallMsg := chatMessage{
			text:       fmt.Sprintf("Executing %s...", msg.toolName),
			sender:     "System",
			timestamp:  time.Now(),
			isToolCall: true,
			toolName:   msg.toolName,
			toolCallID: msg.toolCallID,
			toolParams: msg.params,
		}
		m.messageList.AddMessage(toolCallMsg)

		// Update components with active tool calls
		m.messageList.SetActiveToolCalls(m.activeToolCalls)
		m.statusBar.SetActiveToolCalls(len(m.activeToolCalls))

	case toolProgressMsg:
		logger.Get().Debug("Tool call progress update", "toolCallID", msg.toolCallID, "progress", msg.progress, "status", msg.status)

		// Update existing tool progress state
		if state, exists := m.activeToolCalls[msg.toolCallID]; exists {
			state.progress = msg.progress
			state.status = msg.status
			state.step = msg.step
			state.totalSteps = msg.totalSteps

			// Update components with latest progress
			m.messageList.SetActiveToolCalls(m.activeToolCalls)
		} else {
			logger.Get().Warn("Received progress for unknown tool call", "toolCallID", msg.toolCallID)
		}

	case toolCompleteMsg:
		logger.Get().Info("Tool call completed", "toolCallID", msg.toolCallID, "success", msg.success, "duration", msg.duration)

		// Remove from active tool calls
		delete(m.activeToolCalls, msg.toolCallID)

		// Add tool result message to chat
		var resultText string
		if msg.success {
			resultText = msg.result
		} else {
			resultText = fmt.Sprintf("Tool execution failed: %s", msg.error)
		}

		toolResultMsg := chatMessage{
			text:         resultText,
			sender:       "System",
			timestamp:    time.Now(),
			isToolResult: true,
			toolName:     msg.toolName,
			toolCallID:   msg.toolCallID,
			toolSuccess:  msg.success,
			toolDuration: msg.duration,
		}
		m.messageList.AddMessage(toolResultMsg)

		// Update components with updated active tool calls
		m.messageList.SetActiveToolCalls(m.activeToolCalls)
		m.statusBar.SetActiveToolCalls(len(m.activeToolCalls))

	default:
		// Update input area for other messages
		m.inputArea, cmd = m.inputArea.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var view strings.Builder

	// Header
	view.WriteString(m.header.View())
	view.WriteString("\n")

	// Message List (viewport)
	view.WriteString(m.messageList.View())
	view.WriteString("\n")

	// Input Area (textarea + suggestions)
	view.WriteString(m.inputArea.View())
	view.WriteString("\n")

	// Status Bar
	view.WriteString(m.statusBar.View())

	return view.String()
}

// LoadHistory loads a previous chat history into the model
func (m *Model) LoadHistory(history *ChatHistory) {
	m.header.SetSessionID(history.SessionID)
	m.header.SetModelName(history.ModelName)
	m.messageList.LoadHistory(history.Messages)
}

func formatToolDescriptions(tools []map[string]interface{}) string {
	var sb strings.Builder
	for _, tool := range tools {
		fmt.Fprintf(&sb, "\n%s: %s\nParameters: %s\n",
			tool["name"], tool["description"], tool["parameters"])
	}
	return sb.String()
}
