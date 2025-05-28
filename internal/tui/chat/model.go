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
	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config" // Ensure this path and package are correct
	"github.com/castrovroberto/CGE/internal/contextkeys"
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

	// Tool call specific styles
	toolCallStyle    lipgloss.Style // New: for tool call messages
	toolResultStyle  lipgloss.Style // New: for tool result messages
	toolSuccessStyle lipgloss.Style // New: for successful tool results
	toolErrorStyle   lipgloss.Style // New: for failed tool results
	toolParamsStyle  lipgloss.Style // New: for tool parameters display

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

	thinkingStartTime time.Time // To track when Ollama request started for live timer
	chatStartTime     time.Time // New: To track the actual start of the chat session

	// New: Add tool registry
	toolRegistry *agent.Registry

	// LLM Client
	llmClient llm.Client // New field for the LLM client

	// Available slash commands for suggestions
	availableCommands []string

	// Progress tracking
	activeToolCalls  map[string]*toolProgressState // Track active tool calls
	progressRenderer *ProgressRenderer             // Custom progress renderer
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

	// Tool call styles
	toolCallStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("25")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Margin(0, 0, 1, 0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33"))

	toolResultStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	toolSuccessStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("22")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("28"))

	toolErrorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("52")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196"))

	toolParamsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

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
		toolCallStyle:     toolCallStyle,
		toolResultStyle:   toolResultStyle,
		toolSuccessStyle:  toolSuccessStyle,
		toolErrorStyle:    toolErrorStyle,
		toolParamsStyle:   toolParamsStyle,
		cfg:               cfg,
		parentCtx:         ctx,
		renderer:          renderer,
		formatter:         formatter,
		messages:          nil,
		placeholderIndex:  -1,
		editingIndex:      -1,
		isEditing:         false,
		chatStartTime:     time.Now(),
		availableCommands: defaultSlashCommands, // Initialize with default commands
		activeToolCalls:   make(map[string]*toolProgressState),
		progressRenderer:  NewProgressRenderer(50), // Default width, will be updated on resize
	}

	// Initialize and set the LLM client
	// For now, we assume Ollama if cfg.LLM.Provider is "ollama" or default
	if cfg.LLM.Provider == "ollama" || cfg.LLM.Provider == "" { // Default to ollama
		m.llmClient = llm.NewOllamaClient()
		m.provider = "Ollama" // Update provider string in model
	} else {
		// Placeholder for other providers like OpenAI
		// m.llmClient = llm.NewOpenAIClient(cfg) // Example
		// For now, if not ollama, we might log an error or use a nil client
		logger.Get().Error("Unsupported LLM provider in chat TUI", "provider", cfg.LLM.Provider)
		// m.llmClient could be nil, fetchOllamaResponse needs to handle this
		m.provider = cfg.LLM.Provider // Set provider string
	}

	// Initialize tool registry
	registry := agent.NewRegistry()

	// Register code analysis tools
	if err := registry.Register(agent.NewCodeSearchTool(cfg.Project.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register code search tool", "error", err)
	}
	if err := registry.Register(agent.NewFileReadTool(cfg.Project.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register file read tool", "error", err)
	}
	if err := registry.Register(agent.NewCodebaseAnalyzeTool(cfg.Project.WorkspaceRoot)); err != nil {
		logger.Get().Error("Failed to register codebase analysis tool", "error", err)
	}
	if err := registry.Register(agent.NewGitTool(cfg.Project.WorkspaceRoot)); err != nil {
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
	cmds := []tea.Cmd{m.spin.Tick, textarea.Blink}
	cmds = append(cmds, func() tea.Msg {
		if m.parentCtx != nil {
			<-m.parentCtx.Done()
			return mainContextCancelledMsg{}
		}
		return nil
	})
	return tea.Batch(cmds...)
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

		ctx, cancel := context.WithTimeout(ctxWithValues, m.cfg.LLM.RequestTimeoutSeconds+5*time.Second)
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

		// Ensure llmClient is not nil before using
		if m.llmClient == nil {
			return ollamaErrorMsg(fmt.Errorf("LLM client not initialized for provider: %s", m.provider))
		}

		response, err := m.llmClient.Generate(ctx, m.modelName, prompt, finalSystemPrompt, toolDescriptions)
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
		// Update component widths
		wFrame := m.viewport.Style.GetHorizontalFrameSize()
		m.viewport.Width = msg.Width - wFrame
		m.textarea.SetWidth(msg.Width)

		// Calculate proper viewport height using our new helper method
		m.viewport.Height = m.calculateViewportHeight(msg.Height)

		// Update progress renderer width
		if m.progressRenderer != nil {
			m.progressRenderer.width = msg.Width
		}

		// Update glamour renderer for new width
		if m.renderer != nil {
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
			if err != nil {
				logger.Get().Error("Failed to re-initialize glamour renderer on resize", "error", err)
			} else {
				m.renderer = newRenderer
				m.rebuildViewport() // Rebuild with new width
			}
		}

		// Update child components
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case toolProgressMsg:
		// Update progress for active tool call
		if state, exists := m.activeToolCalls[msg.toolCallID]; exists {
			state.progress = msg.progress
			state.status = msg.status
			state.step = msg.step
			state.totalSteps = msg.totalSteps

			// Update the message in place if it exists
			if state.messageIndex >= 0 && state.messageIndex < len(m.messages) {
				// Update the progress display in the existing message
				m.rebuildViewport()
			}
		}

	case toolStartMsg:
		// Start tracking a new tool call
		m.activeToolCalls[msg.toolCallID] = &toolProgressState{
			toolName:     msg.toolName,
			startTime:    time.Now(),
			progress:     0.0,
			status:       "Starting...",
			step:         0,
			totalSteps:   0,
			messageIndex: len(m.messages), // Will be the next message
		}

	case toolCompleteMsg:
		// Remove from active tracking and update final result
		if state, exists := m.activeToolCalls[msg.toolCallID]; exists {
			delete(m.activeToolCalls, msg.toolCallID)

			// Update the final message with completion status
			if state.messageIndex >= 0 && state.messageIndex < len(m.messages) {
				m.messages[state.messageIndex].toolSuccess = msg.success
				m.messages[state.messageIndex].toolDuration = msg.duration
				if !msg.success && msg.error != "" {
					m.messages[state.messageIndex].text = msg.error
				} else if msg.result != "" {
					m.messages[state.messageIndex].text = msg.result
				}
				m.rebuildViewport()
			}
		}

	case mainContextCancelledMsg:
		logger.Get().Info("Main context canceled, attempting to save chat history and quit TUI.")
		if err := m.SaveHistory(); err != nil {
			m.err = fmt.Errorf("error saving history on context cancellation: %w", err)
			logger.Get().Error("Failed to save chat history on context cancellation", "error", err, "sessionID", m.sessionID)
		} else {
			logger.Get().Info("Chat history saved successfully on context cancellation.", "sessionID", m.sessionID)
		}
		return m, tea.Quit

	case tea.KeyMsg:
		// Handle Ctrl+C and Esc (when not editing) first as they cause an exit.
		switch msg.Type {
		case tea.KeyCtrlC:
			logger.Get().Info("Ctrl+C pressed, attempting to save chat history and quit TUI.")
			if err := m.SaveHistory(); err != nil {
				m.err = fmt.Errorf("error saving history on Ctrl+C: %w", err)
				logger.Get().Error("Failed to save chat history on Ctrl+C", "error", err, "sessionID", m.sessionID)
			} else {
				logger.Get().Info("Chat history saved successfully on Ctrl+C.", "sessionID", m.sessionID)
			}
			return m, tea.Quit
		default:
			// For keys not handled by the switch above (Ctrl+C),
			// use the string representation for other specific keys or pass to textarea.
			switch keyStr := msg.String(); keyStr {
			case "enter":
				// If suggestions are active and one is selected, apply it first
				if len(m.suggestions) > 0 && m.selected >= 0 && m.selected < len(m.suggestions) {
					m.textarea.SetValue(m.suggestions[m.selected])
					m.textarea.CursorEnd() // Move cursor to end after setting value
					m.suggestions = nil    // Clear suggestions
					m.selected = -1        // Reset selection
					// Don't send the message yet, just apply the suggestion
					return m, nil
				}

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
					// If enter is pressed on empty textarea or while loading, let textarea handle it (might do nothing or add newline)
					m.textarea, cmd = m.textarea.Update(msg)
					cmds = append(cmds, cmd)
				}
			case "ctrl+e":
				m.startEditing()
				m.textarea, cmd = m.textarea.Update(msg) // Pass to textarea for consistency or specific handling
				cmds = append(cmds, cmd)
			case "tab":
				if len(m.suggestions) > 0 && m.selected >= 0 && m.selected < len(m.suggestions) {
					// Tab inserts the selected suggestion
					m.textarea.SetValue(m.suggestions[m.selected])
					m.textarea.CursorEnd() // Move cursor to end after setting value
					m.suggestions = nil    // Clear suggestions
					m.selected = -1        // Reset selection
					// Don't pass to textarea when suggestion is applied
				} else {
					m.textarea, cmd = m.textarea.Update(msg) // If no suggestions, let textarea handle Tab
					cmds = append(cmds, cmd)
				}
				// After handling tab, update suggestions in case the input text could trigger new ones
				m.updateSuggestions(m.textarea.Value())
			case "escape":
				if len(m.suggestions) > 0 {
					// Escape clears suggestions
					m.suggestions = nil
					m.selected = -1
					// Don't pass to textarea when clearing suggestions
				} else {
					// If no suggestions, let normal Esc handling take over (quit/cancel editing)
					if !m.isEditing { // If not editing, Esc quits and saves
						logger.Get().Info("Escape pressed (not editing), attempting to save chat history and quit TUI.")
						if err := m.SaveHistory(); err != nil {
							m.err = fmt.Errorf("error saving history on Escape: %w", err)
							logger.Get().Error("Failed to save chat history on Escape", "error", err, "sessionID", m.sessionID)
						} else {
							logger.Get().Info("Chat history saved successfully on Escape.", "sessionID", m.sessionID)
						}
						return m, tea.Quit
					} else { // If editing, Esc cancels editing mode
						m.isEditing = false
						m.editingIndex = -1
						m.textarea.Blur()
						m.textarea.Reset()
						m.textarea.Placeholder = "Type your message... (Ctrl+E to edit last, Tab for completion)"
						// Let the textarea also process the Esc key (e.g., to clear its internal state if any)
						m.textarea, cmd = m.textarea.Update(msg)
						cmds = append(cmds, cmd)
					}
				}
			case "up":
				if len(m.suggestions) > 0 {
					// Navigate suggestions with arrow keys
					m.selected = (m.selected - 1 + len(m.suggestions)) % len(m.suggestions)
					// Don't pass to textarea when suggestions are active
				} else {
					m.textarea, cmd = m.textarea.Update(msg)
					cmds = append(cmds, cmd)
				}
			case "down":
				if len(m.suggestions) > 0 {
					// Navigate suggestions with arrow keys
					m.selected = (m.selected + 1) % len(m.suggestions)
					// Don't pass to textarea when suggestions are active
				} else {
					m.textarea, cmd = m.textarea.Update(msg)
					cmds = append(cmds, cmd)
				}
			default: // Crucial for typing, backspace, arrows within textarea
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
				// After any key that modifies the textarea, update suggestions
				m.updateSuggestions(m.textarea.Value())
			}
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
		// Check for tool invocation
		var toolInvoke struct {
			Tool   string          `json:"tool"`
			Params json.RawMessage `json:"params"`
		}
		trimmed := strings.TrimSpace(botResponseText)
		if strings.HasPrefix(trimmed, "{") {
			if err := json.Unmarshal([]byte(botResponseText), &toolInvoke); err == nil && toolInvoke.Tool != "" {
				// Parse tool parameters for display
				var toolParams map[string]interface{}
				json.Unmarshal(toolInvoke.Params, &toolParams)

				// Add tool call message
				toolCallID := fmt.Sprintf("call_%d", time.Now().UnixNano())
				m.replacePlaceholder(chatMessage{
					text:       "",
					sender:     "Assistant",
					timestamp:  time.Now(),
					isToolCall: true,
					toolName:   toolInvoke.Tool,
					toolCallID: toolCallID,
					toolParams: toolParams,
				})

				// Execute tool and add result
				if tool, ok := m.toolRegistry.Get(toolInvoke.Tool); ok {
					toolStartTime := time.Now()
					result, toolErr := tool.Execute(m.parentCtx, toolInvoke.Params)
					toolDuration := time.Since(toolStartTime)

					var resultText string
					var success bool

					if toolErr != nil {
						resultText = fmt.Sprintf("Error executing tool %s: %v", toolInvoke.Tool, toolErr)
						success = false
					} else {
						resultJSON, _ := json.MarshalIndent(result, "", "  ")
						resultText = string(resultJSON)
						success = true
					}

					m.addMessage(chatMessage{
						text:         resultText,
						sender:       "System",
						timestamp:    time.Now(),
						isToolResult: true,
						toolName:     toolInvoke.Tool,
						toolCallID:   toolCallID,
						toolSuccess:  success,
						toolDuration: toolDuration,
						isMarkdown:   success, // Format as markdown if successful
					})
				} else {
					m.addMessage(chatMessage{
						text:         fmt.Sprintf("Unknown tool requested: %s", toolInvoke.Tool),
						sender:       "System",
						timestamp:    time.Now(),
						isToolResult: true,
						toolName:     toolInvoke.Tool,
						toolCallID:   toolCallID,
						toolSuccess:  false,
					})
				}
				m.viewport.GotoBottom()
				return m, nil
			}
		}
		// Regular AI response
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
		errorMsgString := fmt.Sprintf("Error: %v", error(msg)) // Convert ollamaErrorMsg to error then string
		m.replacePlaceholder(chatMessage{
			text:      m.errorStyle.Render(errorMsgString), // Ensure errorStyle is applied
			sender:    "System",
			timestamp: time.Now(),
		})

	case errMsg:
		m.err = msg

	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// New: Check if the response is a tool invocation (assuming this was intended from previous structure)
		// This case might need to be reviewed if it's from an ollamaSuccessResponseMsg.text
	}

	// If viewport needs to react to any other messages (e.g., mouse events not directly handled)
	// Check if viewport update is needed, but avoid double-updating on WindowSizeMsg.
	// This was commented out, let's keep it that way unless a specific need arises.
	// m.viewport, cmd = m.viewport.Update(msg)
	// cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// rebuildViewport re-renders all stored messages into the viewport, applying markdown and wrapping on resize.
func (m *Model) rebuildViewport() {
	defer func() {
		if r := recover(); r != nil {
			logger.Get().Error("Panic in rebuildViewport", "panic", r)
			// Set fallback content
			m.viewport.SetContent("Error rendering messages. Please restart the chat.")
		}
	}()

	var b strings.Builder
	for i, cm := range m.messages {
		// Add index validation
		if i < 0 || i >= len(m.messages) {
			logger.Get().Warn("Invalid message index in rebuildViewport", "index", i, "length", len(m.messages))
			continue
		}

		// Handle tool call messages specially
		if cm.isToolCall {
			b.WriteString(m.formatToolCall(cm))
		} else if cm.isToolResult {
			b.WriteString(m.formatToolResult(cm))
		} else if cm.isMarkdown && m.renderer != nil {
			// Handle all markdown consistently
			rendered, err := m.renderer.Render(cm.text)
			if err != nil {
				logger.Get().Warn("Markdown rendering failed in rebuildViewport", "error", err)
				// Fall back to regular message formatting
				if cm.sender != "" && cm.text != "" {
					senderPrefix := m.senderStyle.Render(cm.sender + ": ")
					timestamp := m.timeStyle.Render(cm.timestamp.Format("15:04:05"))

					if cm.ThinkingTime > 0 {
						thinkingTime := m.thinkingTimeStyle.Render(fmt.Sprintf(" (%.2fs)", cm.ThinkingTime.Seconds()))
						b.WriteString(fmt.Sprintf("%s %s%s\n%s", senderPrefix, timestamp, thinkingTime, cm.text))
					} else {
						b.WriteString(fmt.Sprintf("%s %s\n%s", senderPrefix, timestamp, cm.text))
					}
				} else {
					b.WriteString(cm.text)
				}
			} else {
				// Format with sender and timestamp
				senderPrefix := m.senderStyle.Render(cm.sender + ": ")
				timestamp := m.timeStyle.Render(cm.timestamp.Format("15:04:05"))

				if cm.ThinkingTime > 0 {
					thinkingTime := m.thinkingTimeStyle.Render(fmt.Sprintf(" (%.2fs)", cm.ThinkingTime.Seconds()))
					b.WriteString(fmt.Sprintf("%s %s%s\n%s", senderPrefix, timestamp, thinkingTime, strings.TrimSpace(rendered)))
				} else {
					b.WriteString(fmt.Sprintf("%s %s\n%s", senderPrefix, timestamp, strings.TrimSpace(rendered)))
				}
			}
		} else {
			// Regular message formatting
			if cm.sender != "" && cm.text != "" {
				senderPrefix := m.senderStyle.Render(cm.sender + ": ")
				timestamp := m.timeStyle.Render(cm.timestamp.Format("15:04:05"))

				if cm.ThinkingTime > 0 {
					thinkingTime := m.thinkingTimeStyle.Render(fmt.Sprintf(" (%.2fs)", cm.ThinkingTime.Seconds()))
					b.WriteString(fmt.Sprintf("%s %s%s\n%s", senderPrefix, timestamp, thinkingTime, cm.text))
				} else {
					b.WriteString(fmt.Sprintf("%s %s\n%s", senderPrefix, timestamp, cm.text))
				}
			} else {
				b.WriteString(cm.text)
			}
		}
		b.WriteString("\n\n") // Add spacing between messages
	}

	// Add active progress bars at the bottom
	if len(m.activeToolCalls) > 0 {
		b.WriteString("\n" + strings.Repeat("─", m.progressRenderer.width) + "\n")
		b.WriteString("🔄 Active Operations:\n\n")

		for _, state := range m.activeToolCalls {
			progressDisplay := m.progressRenderer.RenderProgress(state)
			b.WriteString(progressDisplay + "\n\n")
		}
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

	// Update placeholder index if this is a placeholder
	if msg.placeholder {
		m.placeholderIndex = len(m.messages) - 1
		logger.Get().Debug("Added placeholder message", "index", m.placeholderIndex)
	}

	m.rebuildViewport()
}

// replacePlaceholder replaces the current placeholder message (if any) with real content; otherwise appends.
func (m *Model) replacePlaceholder(msg chatMessage) {
	if m.placeholderIndex >= 0 && m.placeholderIndex < len(m.messages) {
		logger.Get().Debug("Replacing placeholder", "index", m.placeholderIndex, "sender", msg.sender)
		m.messages[m.placeholderIndex] = msg
		m.placeholderIndex = -1
	} else {
		logger.Get().Debug("No valid placeholder to replace, appending message", "placeholderIndex", m.placeholderIndex, "messagesLength", len(m.messages))
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

// updateSuggestions populates the suggestions list based on the input.
// For now, it suggests slash commands if the input starts with "/".
func (m *Model) updateSuggestions(input string) {
	previousSuggestionCount := len(m.suggestions)

	if strings.HasPrefix(input, "/") {
		m.suggestions = nil
		m.selected = 0
		for _, cmd := range m.availableCommands {
			if strings.HasPrefix(cmd, input) {
				m.suggestions = append(m.suggestions, cmd)
			}
		}
		// Ensure selected index is valid
		if len(m.suggestions) == 0 {
			m.selected = -1
		} else if m.selected >= len(m.suggestions) {
			m.selected = 0
		}

		// Log suggestion updates for debugging
		if len(m.suggestions) > 0 {
			logger.Get().Debug("Updated suggestions", "input", input, "count", len(m.suggestions), "selected", m.selected)
		}
	} else {
		m.suggestions = nil
		m.selected = -1
	}

	// If suggestion count changed, recalculate viewport height
	if len(m.suggestions) != previousSuggestionCount {
		// Estimate current window height from current viewport and component heights
		currentHeight := m.viewport.Height + m.getHeaderHeight() + m.getStatusBarHeight() +
			m.textarea.Height() + previousSuggestionCount + m.viewport.Style.GetVerticalFrameSize()

		// Recalculate viewport height with new suggestion count
		newHeight := m.calculateViewportHeight(currentHeight)
		logger.Get().Debug("Recalculating viewport height due to suggestion change",
			"previousSuggestions", previousSuggestionCount,
			"newSuggestions", len(m.suggestions),
			"oldHeight", m.viewport.Height,
			"newHeight", newHeight)
		m.viewport.Height = newHeight
	}
}

func (m Model) View() string {
	var view strings.Builder

	// Header
	header := m.headerStyle.Render(fmt.Sprintf("Chat with %s (%s) | Session: %s | Status: %s",
		m.provider, m.modelName, m.sessionID, m.status))
	view.WriteString(header)
	view.WriteString("\n")

	// Viewport (content is managed by rebuildViewport())
	view.WriteString(m.viewport.View())
	view.WriteString("\n")
	view.WriteString(m.textarea.View())

	// Render suggestions if any
	if len(m.suggestions) > 0 {
		view.WriteString("\n") // Add a line break before suggestions
		suggestionLines := make([]string, len(m.suggestions))
		for i, sug := range m.suggestions {
			if i == m.selected {
				suggestionLines[i] = m.suggestionStyle.Copy().Reverse(true).Render("> " + sug)
			} else {
				suggestionLines[i] = m.suggestionStyle.Render("  " + sug)
			}
		}
		view.WriteString(strings.Join(suggestionLines, "\n"))
	}

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
		// Enhanced status bar with more information
		var statusParts []string

		// Basic controls
		statusParts = append(statusParts, "Ctrl+C: quit")
		statusParts = append(statusParts, "Ctrl+E: edit last")
		statusParts = append(statusParts, "Tab: suggestions")

		// Active operations count
		if len(m.activeToolCalls) > 0 {
			statusParts = append(statusParts, fmt.Sprintf("Active: %d", len(m.activeToolCalls)))
		}

		// Session info
		sessionDuration := time.Since(m.chatStartTime)
		statusParts = append(statusParts, fmt.Sprintf("Session: %.0fm", sessionDuration.Minutes()))

		statusBar = m.statusStyle.Render(strings.Join(statusParts, " | "))
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
// formatToolCall formats a tool call message for display
func (m *Model) formatToolCall(msg chatMessage) string {
	var parts []string

	// Tool call header with icon
	header := fmt.Sprintf("🔧 Tool Call: %s", msg.toolName)
	if msg.toolCallID != "" {
		header += fmt.Sprintf(" (ID: %s)", msg.toolCallID[:8]) // Show first 8 chars of ID
	}
	parts = append(parts, m.toolCallStyle.Render(header))

	// Tool parameters (if any)
	if len(msg.toolParams) > 0 {
		paramsJSON, _ := json.MarshalIndent(msg.toolParams, "", "  ")
		paramText := fmt.Sprintf("Parameters:\n%s", string(paramsJSON))
		parts = append(parts, m.toolParamsStyle.Render(paramText))
	}

	return strings.Join(parts, "\n")
}

// formatToolResult formats a tool result message for display
func (m *Model) formatToolResult(msg chatMessage) string {
	var parts []string

	// Result header with status icon
	var icon, header string
	var style lipgloss.Style

	if msg.toolSuccess {
		icon = "✅"
		header = fmt.Sprintf("%s Tool Result: %s", icon, msg.toolName)
		style = m.toolSuccessStyle
	} else {
		icon = "❌"
		header = fmt.Sprintf("%s Tool Error: %s", icon, msg.toolName)
		style = m.toolErrorStyle
	}

	if msg.toolDuration > 0 {
		header += fmt.Sprintf(" (%.2fs)", msg.toolDuration.Seconds())
	}

	parts = append(parts, style.Render(header))

	// Result content
	if msg.text != "" {
		resultContent := m.toolResultStyle.Render(msg.text)
		parts = append(parts, resultContent)
	}

	return strings.Join(parts, "\n")
}

func formatToolDescriptions(tools []map[string]interface{}) string {
	var sb strings.Builder
	for _, tool := range tools {
		fmt.Fprintf(&sb, "\n%s: %s\nParameters: %s\n",
			tool["name"], tool["description"], tool["parameters"])
	}
	return sb.String()
}

// Layout dimension helper methods
func (m *Model) getHeaderHeight() int {
	return 2 // Header + newline
}

func (m *Model) getStatusBarHeight() int {
	return 2 // Status bar + newline
}

func (m *Model) getSuggestionAreaHeight() int {
	if len(m.suggestions) > 0 {
		return len(m.suggestions) + 1 // suggestions + newline
	}
	return 0
}

func (m *Model) calculateViewportHeight(windowHeight int) int {
	headerHeight := m.getHeaderHeight()
	statusBarHeight := m.getStatusBarHeight()
	textareaHeight := m.textarea.Height()
	suggestionAreaHeight := m.getSuggestionAreaHeight()

	// Account for viewport frame (border)
	viewportFrameHeight := m.viewport.Style.GetVerticalFrameSize()

	// Calculate available height for viewport content
	availableHeight := windowHeight - headerHeight - statusBarHeight - textareaHeight - suggestionAreaHeight - viewportFrameHeight

	// Ensure minimum viewport height
	minHeight := 3
	if availableHeight < minHeight {
		return minHeight
	}

	return availableHeight
}
