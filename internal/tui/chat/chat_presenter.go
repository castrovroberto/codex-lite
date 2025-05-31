package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/llm"
	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// ChatPresenter implements MessageProvider and acts as the presenter layer
// between the TUI and the business logic (orchestrator)
type ChatPresenter struct {
	llmClient    llm.Client
	toolRegistry *agent.Registry
	agentRunner  *orchestrator.AgentRunner
	messagesChan chan ChatMessage
	ctx          context.Context
	cancelCtx    context.CancelFunc
	systemPrompt string
	modelName    string
}

// NewChatPresenter creates a new ChatPresenter
func NewChatPresenter(ctx context.Context, llmClient llm.Client, toolRegistry *agent.Registry, systemPrompt, modelName string) *ChatPresenter {
	pCtx, pCancel := context.WithCancel(ctx)
	presenter := &ChatPresenter{
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		messagesChan: make(chan ChatMessage, 10), // Buffered channel
		ctx:          pCtx,
		cancelCtx:    pCancel,
		systemPrompt: systemPrompt,
		modelName:    modelName,
	}

	// Initialize AgentRunner
	presenter.agentRunner = orchestrator.NewAgentRunner(llmClient, toolRegistry, systemPrompt, modelName)

	return presenter
}

// Send implements MessageProvider.Send
func (p *ChatPresenter) Send(ctx context.Context, prompt string) error {
	// Start processing asynchronously
	go p.processPromptAsync(ctx, prompt)
	return nil
}

// Messages implements MessageProvider.Messages
func (p *ChatPresenter) Messages() <-chan ChatMessage {
	return p.messagesChan
}

// Close implements MessageProvider.Close
func (p *ChatPresenter) Close() error {
	p.cancelCtx()
	close(p.messagesChan)
	return nil
}

// processPromptAsync handles the actual agent interaction asynchronously
func (p *ChatPresenter) processPromptAsync(ctx context.Context, prompt string) {
	// Generate unique ID for this conversation turn
	turnID := p.generateID()

	// Send user message first
	p.sendMessage(ChatMessage{
		ID:        p.generateID(),
		Type:      UserMessage,
		Sender:    "User",
		Text:      prompt,
		Timestamp: time.Now(),
	})

	// Run the agent
	result, err := p.agentRunner.Run(ctx, prompt)
	if err != nil {
		p.sendMessage(ChatMessage{
			ID:        p.generateID(),
			Type:      ErrorMessage,
			Sender:    "System",
			Text:      fmt.Sprintf("Error: %v", err),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"turn_id": turnID,
				"error":   err.Error(),
			},
		})
		return
	}

	// Convert orchestrator result to chat messages
	p.convertRunResultToMessages(result, turnID)
}

// convertRunResultToMessages converts an orchestrator.RunResult to ChatMessage(s)
func (p *ChatPresenter) convertRunResultToMessages(result *orchestrator.RunResult, turnID string) {
	if !result.Success && result.Error != "" {
		p.sendMessage(ChatMessage{
			ID:        p.generateID(),
			Type:      ErrorMessage,
			Sender:    "System",
			Text:      fmt.Sprintf("Error: %s", result.Error),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"turn_id":    turnID,
				"iterations": result.Iterations,
				"tool_calls": result.ToolCalls,
			},
		})
		return
	}

	// Send tool call and result messages if there were tool interactions
	if result.ToolCalls > 0 {
		// For now, send a summary of tool calls
		// In a more sophisticated implementation, you might track individual tool calls
		// by integrating more deeply with the AgentRunner's iteration process
		p.sendMessage(ChatMessage{
			ID:        p.generateID(),
			Type:      ToolCallMessage,
			Sender:    "System",
			Text:      fmt.Sprintf("Executed %d tool calls in %d iterations", result.ToolCalls, result.Iterations),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"turn_id":    turnID,
				"tool_calls": result.ToolCalls,
				"iterations": result.Iterations,
			},
		})
	}

	// Send the final response
	if result.FinalResponse != "" {
		p.sendMessage(ChatMessage{
			ID:        p.generateID(),
			Type:      AssistantMessage,
			Sender:    "Assistant",
			Text:      result.FinalResponse,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"turn_id":    turnID,
				"iterations": result.Iterations,
				"tool_calls": result.ToolCalls,
			},
		})
	}
}

// sendMessage safely sends a message to the channel
func (p *ChatPresenter) sendMessage(msg ChatMessage) {
	select {
	case p.messagesChan <- msg:
		// Message sent successfully
	case <-p.ctx.Done():
		// Context cancelled, stop sending
	default:
		// Channel full, could handle this differently (e.g., drop oldest, log warning)
	}
}

// generateID generates a unique ID for messages
func (p *ChatPresenter) generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
