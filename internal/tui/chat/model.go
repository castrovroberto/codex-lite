// internal/tui/chat/model.go
package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	// Bubbles components for TUI

	tea "github.com/charmbracelet/bubbletea"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config" // Ensure this path and package are correct
	"github.com/castrovroberto/CGE/internal/llm"

	// Import the new llm package

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

// Add near the top after other type definitions
type chatMsgWrapper struct {
	ChatMessage
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

	// Business logic interfaces (injectable for testing)
	messageProvider MessageProvider
	delayProvider   DelayProvider
	historyService  HistoryService

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

// NewChatModel creates a new ChatModel using functional options
func NewChatModel(opts ...ChatModelOption) Model {
	// Initialize with default values
	m := Model{
		theme:             NewDefaultTheme(),
		availableCommands: defaultSlashCommands,
		activeToolCalls:   make(map[string]*toolProgressState),
		chatStartTime:     time.Now(),
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(&m)
	}

	// Ensure essential components are initialized if not provided by options
	if m.theme == nil {
		m.theme = NewDefaultTheme()
	}
	if m.layout == nil {
		m.layout = NewLayoutDimensions(m.theme)
	}

	sessionID := m.chatStartTime.Format("20060102150405")
	if m.header == nil {
		m.header = NewHeaderModel(m.theme, "Unknown", "default", sessionID, "Initializing")
	}
	if m.statusBar == nil {
		m.statusBar = NewStatusBarModel(m.theme, m.chatStartTime)
	}
	if m.inputArea == nil {
		m.inputArea = NewInputAreaModel(m.theme, m.availableCommands)
	}
	if m.messageList == nil {
		m.messageList = NewMessageListModel(m.theme, 50, 10)
	}

	// Ensure essential providers are set
	if m.messageProvider == nil {
		panic("MessageProvider is required for ChatModel")
	}
	if m.delayProvider == nil {
		m.delayProvider = &RealDelayProvider{}
	}

	// Add welcome message
	welcomeMsg := chatMessage{
		text:       "Welcome to CGE Chat! Type your message or use '/' for commands.",
		sender:     "System",
		timestamp:  time.Now(),
		isMarkdown: false,
	}
	m.messageList.AddMessage(welcomeMsg)

	return m
}

// Legacy InitialModel function for compatibility - creates ChatPresenter automatically
func InitialModel(ctx context.Context, cfg *config.AppConfig, modelName string) Model {
	// Create LLM client based on configuration
	var llmClient llm.Client
	switch cfg.LLM.Provider {
	case "ollama":
		ollamaConfig := cfg.GetOllamaConfig()
		llmClient = llm.NewOllamaClient(ollamaConfig)
	case "openai":
		openaiConfig := cfg.GetOpenAIConfig()
		llmClient = llm.NewOpenAIClient(openaiConfig)
	default:
		// Fallback to ollama if provider is unknown
		ollamaConfig := cfg.GetOllamaConfig()
		llmClient = llm.NewOllamaClient(ollamaConfig)
	}

	// Create tool registry with chat tools
	workspaceRoot := cfg.Project.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = "." // Fallback to current directory
	}

	// Convert workspace root to absolute path
	absWorkspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absWorkspaceRoot = workspaceRoot
	}

	toolFactory := agent.NewToolFactory(absWorkspaceRoot)
	toolRegistry := toolFactory.CreateGenerationRegistry()

	// Create chat presenter
	systemPrompt := cfg.GetLoadedChatSystemPrompt()
	presenter := NewChatPresenter(ctx, llmClient, toolRegistry, systemPrompt, modelName)

	// Create model with options
	return NewChatModel(
		WithParentContext(ctx),
		WithInitialConfig(cfg),
		WithMessageProvider(presenter),
		WithDelayProvider(&RealDelayProvider{}),
	)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.statusBar.GetSpinnerTickCmd(),
		m.listenForMessages(), // Start listening for messages from the provider
	)
}

func (m Model) sendMessage(prompt string) tea.Cmd {
	// Send the message through the message provider
	if err := m.messageProvider.Send(m.parentCtx, prompt); err != nil {
		return func() tea.Msg {
			return errMsg(err)
		}
	}
	return nil
}

// listenForMessages creates a command that listens to the message provider's channel
func (m Model) listenForMessages() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.messageProvider.Messages():
			if !ok {
				return errMsg(fmt.Errorf("message provider channel closed"))
			}
			return chatMsgWrapper{ChatMessage: msg}
		case <-m.parentCtx.Done():
			return mainContextCancelledMsg{}
		}
	}
}

// convertToTuiMessage converts a ChatMessage to the internal chatMessage format
func convertToTuiMessage(msg ChatMessage) chatMessage {
	tuiMsg := chatMessage{
		text:      msg.Text,
		sender:    msg.Sender,
		timestamp: msg.Timestamp,
	}

	// Map message types to appropriate display properties
	switch msg.Type {
	case AssistantMessage:
		tuiMsg.isMarkdown = true
	case ToolCallMessage:
		tuiMsg.isToolCall = true
		if name, ok := msg.Metadata["tool_name"].(string); ok {
			tuiMsg.toolName = name
		}
		if id, ok := msg.Metadata["tool_call_id"].(string); ok {
			tuiMsg.toolCallID = id
		}
		if params, ok := msg.Metadata["params"].(map[string]interface{}); ok {
			tuiMsg.toolParams = params
		}
	case ToolResultMessage:
		tuiMsg.isToolResult = true
		if name, ok := msg.Metadata["tool_name"].(string); ok {
			tuiMsg.toolName = name
		}
		if id, ok := msg.Metadata["tool_call_id"].(string); ok {
			tuiMsg.toolCallID = id
		}
		if success, ok := msg.Metadata["success"].(bool); ok {
			tuiMsg.toolSuccess = success
		}
		if duration, ok := msg.Metadata["duration"].(time.Duration); ok {
			tuiMsg.toolDuration = duration
		}
	case ErrorMessage:
		// Error messages are displayed as regular text, but could be styled differently
	}

	return tuiMsg
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
		// First, update all components that need width/height information
		var headerCmd, inputCmd tea.Cmd

		// Update header first to ensure height calculation is current
		m.header, headerCmd = m.header.Update(msg)
		if headerCmd != nil {
			cmds = append(cmds, headerCmd)
		}

		// Calculate viewport height using centralized layout dimensions with dynamic header
		textareaHeight := m.inputArea.GetHeight()
		suggestionAreaHeight := m.inputArea.GetSuggestionAreaHeight()

		// Use the new header-aware layout validation
		err := m.layout.ValidateLayoutWithHeader(
			msg.Height,
			textareaHeight,
			suggestionAreaHeight,
			m.layout.GetViewportFrameHeight(),
			m.header,
		)
		if err != nil {
			logger.Get().Warn("Layout validation failed", "error", err)
			// In case of layout validation failure, try to use a safe fallback
			logger.Get().Debug("Layout validation details",
				"windowHeight", msg.Height,
				"headerHeight", m.header.GetHeight(),
				"textareaHeight", textareaHeight,
				"suggestionAreaHeight", suggestionAreaHeight,
				"statusBarHeight", m.layout.GetStatusBarHeight(),
				"viewportFrameHeight", m.layout.GetViewportFrameHeight())
		}

		// Add debug layout information with dynamic header height
		m.debugLayoutInfoWithHeader(msg.Width, msg.Height, textareaHeight, suggestionAreaHeight)

		viewportHeight := m.layout.CalculateViewportHeightWithHeader(
			msg.Height,
			textareaHeight,
			suggestionAreaHeight,
			m.layout.GetViewportFrameHeight(),
			m.header,
		)

		// Ensure viewport height is reasonable
		if viewportHeight < m.layout.GetMinViewportHeight() {
			logger.Get().Warn("Calculated viewport height is too small, using minimum",
				"calculated", viewportHeight,
				"minimum", m.layout.GetMinViewportHeight())
			viewportHeight = m.layout.GetMinViewportHeight()
		}

		m.messageList.SetHeight(viewportHeight)

		// Update input area after header calculations are complete
		m.inputArea, inputCmd = m.inputArea.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
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
				// Start loading state with proper coordination
				m.setLoading(true)

				userPrompt := m.inputArea.GetValue()
				m.messageList.AddMessage(chatMessage{
					text:      userPrompt,
					sender:    "You",
					timestamp: time.Now(),
				})

				// Add a placeholder for the assistant response
				m.messageList.AddMessage(chatMessage{
					text:        "Thinking...",
					sender:      "Assistant",
					timestamp:   time.Now(),
					placeholder: true,
				})

				m.inputArea.Reset()
				return m, tea.Batch(m.sendMessage(userPrompt), m.statusBar.GetSpinnerTickCmd())
			}

		default:
			// Pass other keys to input area
			m.inputArea, cmd = m.inputArea.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case ollamaSuccessResponseMsg:
		// End loading state with proper coordination and cleanup
		m.setLoading(false)
		m.statusBar.ClearError()

		// Calculate thinking time from the start time
		var thinkingTime time.Duration
		if !m.thinkingStartTime.IsZero() {
			thinkingTime = msg.duration
		}

		responseMsg := chatMessage{
			text:         msg.response,
			sender:       "Assistant",
			timestamp:    time.Now(),
			isMarkdown:   true,
			ThinkingTime: thinkingTime,
		}
		m.messageList.ReplacePlaceholder(responseMsg)

		// Reset thinking start time
		m.thinkingStartTime = time.Time{}

	case ollamaErrorMsg:
		// End loading state and handle error
		m.setError(msg)

		errorMsg := chatMessage{
			text:      fmt.Sprintf("Error: %v", msg),
			sender:    "System",
			timestamp: time.Now(),
		}
		m.messageList.ReplacePlaceholder(errorMsg)

		// Reset thinking start time
		m.thinkingStartTime = time.Time{}

	case errMsg:
		m.setError(msg)

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

		// Update components with active tool calls using centralized method
		m.updateToolCallState()

	case toolProgressMsg:
		logger.Get().Debug("Tool call progress update", "toolCallID", msg.toolCallID, "progress", msg.progress, "status", msg.status)

		// Update existing tool progress state
		if state, exists := m.activeToolCalls[msg.toolCallID]; exists {
			state.progress = msg.progress
			state.status = msg.status
			state.step = msg.step
			state.totalSteps = msg.totalSteps

			// Update components with latest progress using centralized method
			m.updateToolCallState()
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

		// Update components with updated active tool calls using centralized method
		m.updateToolCallState()

	case chatMsgWrapper:
		// Handle new messages from the MessageProvider
		chatMessage := msg.ChatMessage
		switch chatMessage.Type {
		case UserMessage:
			// User messages are typically added when sending, but could be echoed back
			m.messageList.AddMessage(convertToTuiMessage(chatMessage))
		case AssistantMessage:
			m.setLoading(false) // Stop loading when we receive assistant response
			m.messageList.ReplacePlaceholder(convertToTuiMessage(chatMessage))
		case ErrorMessage:
			m.setError(fmt.Errorf("%s", chatMessage.Text))
			m.messageList.ReplacePlaceholder(convertToTuiMessage(chatMessage))
		case ToolCallMessage:
			// Display tool call attempt
			m.messageList.AddMessage(convertToTuiMessage(chatMessage))
			if _, ok := chatMessage.Metadata["tool_call_id"].(string); ok {
				m.updateToolCallState()
			}
		case ToolResultMessage:
			// Display tool result
			m.messageList.AddMessage(convertToTuiMessage(chatMessage))
			m.updateToolCallState()
		case SystemMessage:
			// Display system messages
			m.messageList.AddMessage(convertToTuiMessage(chatMessage))
		}
		m.messageList.GotoBottom()
		// Return a new command to continue listening
		return m, m.listenForMessages()

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

// State management helper methods

// setLoading sets the loading state consistently across components
func (m *Model) setLoading(loading bool) {
	m.loading = loading
	if loading {
		m.thinkingStartTime = time.Now()
	}
	m.statusBar.SetLoading(loading)
}

// setError sets error state and clears loading
func (m *Model) setError(err error) {
	m.loading = false
	m.statusBar.SetLoading(false)
	m.statusBar.SetError(err)
}

// updateToolCallState updates tool call state consistently across components
func (m *Model) updateToolCallState() {
	m.statusBar.SetActiveToolCalls(len(m.activeToolCalls))
	m.messageList.SetActiveToolCalls(m.activeToolCalls)

	// Use centralized status bar state update
	m.updateStatusBarState()
}

// updateStatusBarState performs centralized, atomic status bar state updates
func (m *Model) updateStatusBarState() {
	state := StatusBarState{
		ActiveToolCalls:  len(m.activeToolCalls),
		SessionStartTime: m.chatStartTime,
		Loading:          m.loading,
		Err:              nil, // Errors are set separately via SetError
		LastUpdateTime:   time.Now(),
	}
	m.statusBar.UpdateState(state)
}

// validateState performs state consistency checks (for debugging)
func (m *Model) validateState() bool {
	// Check tool call state consistency
	if len(m.activeToolCalls) < 0 {
		logger.Get().Warn("Negative active tool call count")
		return false
	}

	return true
}

// debugLayoutInfo logs detailed layout information for troubleshooting
func (m *Model) debugLayoutInfo(windowWidth, windowHeight, textareaHeight, suggestionAreaHeight int) {
	logger.Get().Debug("Layout debug info",
		"windowWidth", windowWidth,
		"windowHeight", windowHeight,
		"headerHeight", m.layout.GetHeaderHeight(),
		"statusBarHeight", m.layout.GetStatusBarHeight(),
		"inputAreaHeight", textareaHeight,
		"suggestionAreaHeight", suggestionAreaHeight,
		"viewportFrameHeight", m.layout.GetViewportFrameHeight(),
		"calculatedViewportHeight", m.layout.CalculateViewportHeight(windowHeight, textareaHeight, suggestionAreaHeight, m.layout.GetViewportFrameHeight()),
		"activeToolCalls", len(m.activeToolCalls),
	)
}

// debugLayoutInfoWithHeader logs detailed layout information for troubleshooting with dynamic header height
func (m *Model) debugLayoutInfoWithHeader(windowWidth, windowHeight, textareaHeight, suggestionAreaHeight int) {
	logger.Get().Debug("Layout debug info with dynamic header height",
		"windowWidth", windowWidth,
		"windowHeight", windowHeight,
		"headerHeight", m.header.GetHeight(),
		"statusBarHeight", m.layout.GetStatusBarHeight(),
		"inputAreaHeight", textareaHeight,
		"suggestionAreaHeight", suggestionAreaHeight,
		"viewportFrameHeight", m.layout.GetViewportFrameHeight(),
		"calculatedViewportHeight", m.layout.CalculateViewportHeightWithHeader(windowHeight, textareaHeight, suggestionAreaHeight, m.layout.GetViewportFrameHeight(), m.header),
		"activeToolCalls", len(m.activeToolCalls),
	)
}

// Getter methods for accessing model components

// Header returns the header model
func (m *Model) Header() *HeaderModel {
	return m.header
}

// MessageList returns the message list model
func (m *Model) MessageList() *MessageListModel {
	return m.messageList
}

// InputArea returns the input area model
func (m *Model) InputArea() *InputAreaModel {
	return m.inputArea
}

// StatusBar returns the status bar model
func (m *Model) StatusBar() *StatusBarModel {
	return m.statusBar
}

// Theme returns the theme
func (m *Model) Theme() *Theme {
	return m.theme
}
