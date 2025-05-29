package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	"github.com/castrovroberto/CGE/internal/contextkeys"
	"github.com/castrovroberto/CGE/internal/llm"
)

// DeliberationPhase represents different phases of deliberation
type DeliberationPhase string

const (
	PhaseThought    DeliberationPhase = "thought"    // Internal reasoning
	PhaseAction     DeliberationPhase = "action"     // Tool selection/execution
	PhaseReflect    DeliberationPhase = "reflect"    // Post-action reflection
	PhaseConfidence DeliberationPhase = "confidence" // Confidence assessment
)

// DeliberationStep represents a single step in the deliberation process
type DeliberationStep struct {
	ID            string                 `json:"id"`
	Phase         DeliberationPhase      `json:"phase"`
	Content       string                 `json:"content"`
	Confidence    float64                `json:"confidence"`
	ReasoningPath []string               `json:"reasoning_path"`
	Timestamp     time.Time              `json:"timestamp"`
	Internal      bool                   `json:"internal"` // Don't include in conversation history
	Metadata      map[string]interface{} `json:"metadata"`
}

// DeliberationResult extends RunResult with deliberation information
type DeliberationResult struct {
	*RunResult
	DeliberationSteps []DeliberationStep `json:"deliberation_steps"`
	ThoughtCount      int                `json:"thought_count"`
	AverageConfidence float64            `json:"average_confidence"`
	ReflectionNotes   []string           `json:"reflection_notes"`
}

// DeliberationRunner extends AgentRunner with deliberation capabilities
type DeliberationRunner struct {
	*AgentRunner
	config          config.DeliberationConfig
	thoughtHistory  []DeliberationStep
	reflectionNotes []string
}

// NewDeliberationRunner creates a new deliberation-enabled agent runner
func NewDeliberationRunner(
	llmClient llm.Client,
	toolRegistry *agent.Registry,
	systemPrompt string,
	model string,
	deliberationConfig config.DeliberationConfig,
) *DeliberationRunner {
	baseRunner := NewAgentRunner(llmClient, toolRegistry, systemPrompt, model)

	return &DeliberationRunner{
		AgentRunner:     baseRunner,
		config:          deliberationConfig,
		thoughtHistory:  make([]DeliberationStep, 0),
		reflectionNotes: make([]string, 0),
	}
}

// RunWithDeliberation executes the agent with deliberation enabled
func (dr *DeliberationRunner) RunWithDeliberation(ctx context.Context, initialPrompt string) (*DeliberationResult, error) {
	return dr.RunWithDeliberationAndCommand(ctx, initialPrompt, "unknown")
}

// RunWithDeliberationAndCommand executes with deliberation and command tracking
func (dr *DeliberationRunner) RunWithDeliberationAndCommand(ctx context.Context, initialPrompt string, command string) (*DeliberationResult, error) {
	log := contextkeys.LoggerFromContext(ctx)

	if !dr.config.Enabled {
		// Fallback to regular execution
		result, err := dr.AgentRunner.RunWithCommand(ctx, initialPrompt, command)
		if err != nil {
			return nil, err
		}
		return &DeliberationResult{
			RunResult:         result,
			DeliberationSteps: []DeliberationStep{},
			ThoughtCount:      0,
			AverageConfidence: 0.0,
			ReflectionNotes:   []string{},
		}, nil
	}

	log.Info("Starting deliberation-enabled agent run",
		"confidence_threshold", dr.config.ConfidenceThreshold,
		"max_thought_depth", dr.config.MaxThoughtDepth)

	// Apply timeout from configuration
	if dr.config.ThoughtTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(dr.config.ThoughtTimeout)*time.Second)
		defer cancel()
	}

	// Initialize session if session manager is available
	if dr.sessionManager != nil && dr.currentSession == nil {
		dr.currentSession = dr.sessionManager.CreateSession(dr.systemPrompt, dr.model, command, dr.AgentRunner.config)
		log.Info("Created new deliberation session", "session_id", dr.currentSession.SessionID)
	}

	// Initialize message history
	messages := []Message{
		{Role: "system", Content: dr.systemPrompt},
		{Role: "user", Content: initialPrompt},
	}

	// Load existing messages if resuming session
	if dr.currentSession != nil && len(dr.currentSession.Messages) > 0 {
		messages = dr.currentSession.Messages
		if len(messages) == 0 || messages[len(messages)-1].Content != initialPrompt {
			messages = append(messages, Message{Role: "user", Content: initialPrompt})
		}
	}

	iterations := 0
	toolCalls := 0

	// Main deliberation loop
	for iterations < dr.maxIterations {
		iterations++
		log.Debug("Deliberation iteration", "iteration", iterations)

		// Phase 1: Thought Generation (if supported)
		if dr.llmClient.SupportsDeliberation() {
			thoughtResult, err := dr.generateThought(ctx, messages, iterations)
			if err != nil {
				log.Warn("Thought generation failed, falling back to direct action", "error", err)
			} else {
				// Add thought step to history
				thoughtStep := DeliberationStep{
					ID:            fmt.Sprintf("thought_%d_%d", iterations, len(dr.thoughtHistory)),
					Phase:         PhaseThought,
					Content:       thoughtResult.ThoughtContent,
					Confidence:    thoughtResult.Confidence,
					ReasoningPath: thoughtResult.ReasoningSteps,
					Timestamp:     time.Now(),
					Internal:      true, // Don't include in conversation
					Metadata: map[string]interface{}{
						"iteration":        iterations,
						"suggested_action": thoughtResult.SuggestedAction,
						"uncertainty":      thoughtResult.Uncertainty,
					},
				}
				dr.thoughtHistory = append(dr.thoughtHistory, thoughtStep)

				// Check confidence threshold
				if thoughtResult.Confidence < dr.config.ConfidenceThreshold {
					log.Debug("Low confidence detected, will require explanation",
						"confidence", thoughtResult.Confidence,
						"threshold", dr.config.ConfidenceThreshold)
				}
			}
		}

		// Phase 2: Action Generation (using regular orchestration)
		actionResult, err := dr.executeActionPhase(ctx, messages, iterations)
		if err != nil {
			return dr.createDeliberationResult(messages, toolCalls, iterations, false, err.Error()), nil
		}

		// Update counters and messages
		if actionResult.IsToolCall {
			toolCalls++
		}
		messages = append(messages, actionResult.Messages...)

		// Phase 3: Confidence Assessment (for significant actions)
		if actionResult.IsToolCall && dr.config.RequireExplanation {
			confidenceResult, err := dr.assessActionConfidence(ctx, actionResult, iterations)
			if err != nil {
				log.Warn("Confidence assessment failed", "error", err)
			} else {
				confidenceStep := DeliberationStep{
					ID:         fmt.Sprintf("confidence_%d_%d", iterations, len(dr.thoughtHistory)),
					Phase:      PhaseConfidence,
					Content:    fmt.Sprintf("Confidence: %.2f - %s", confidenceResult.Score, confidenceResult.Recommendation),
					Confidence: confidenceResult.Score,
					Timestamp:  time.Now(),
					Internal:   true,
					Metadata: map[string]interface{}{
						"iteration":      iterations,
						"factors":        confidenceResult.Factors,
						"uncertainties":  confidenceResult.Uncertainties,
						"recommendation": confidenceResult.Recommendation,
					},
				}
				dr.thoughtHistory = append(dr.thoughtHistory, confidenceStep)

				// Handle low confidence
				if confidenceResult.Score < dr.config.ConfidenceThreshold {
					if confidenceResult.Recommendation == "abort" {
						return dr.createDeliberationResult(messages, toolCalls, iterations, false,
							fmt.Sprintf("Action aborted due to low confidence: %.2f", confidenceResult.Score)), nil
					}
				}
			}
		}

		// Check for completion
		if actionResult.IsFinal {
			log.Info("Deliberation completed",
				"iterations", iterations,
				"tool_calls", toolCalls,
				"thought_steps", len(dr.thoughtHistory))

			// Phase 4: Reflection (if enabled)
			if dr.config.EnableReflection {
				dr.generateReflection(ctx, messages, iterations)
			}

			return dr.createDeliberationResult(messages, toolCalls, iterations, true, ""), nil
		}
	}

	// Max iterations reached
	log.Warn("Deliberation reached max iterations", "max_iterations", dr.maxIterations)
	return dr.createDeliberationResult(messages, toolCalls, iterations, false,
		fmt.Sprintf("reached maximum iterations (%d)", dr.maxIterations)), nil
}

// ActionResult represents the result of an action phase
type ActionResult struct {
	Messages   []Message
	IsToolCall bool
	IsFinal    bool
	ToolName   string
	Content    string
}

// generateThought performs internal reasoning before taking action
func (dr *DeliberationRunner) generateThought(ctx context.Context, messages []Message, iteration int) (*llm.ThoughtResponse, error) {
	prompt := dr.buildThoughtPrompt(messages, iteration)
	context := dr.buildContextForThought(messages)

	return dr.llmClient.GenerateThought(ctx, dr.model, prompt, context)
}

// executeActionPhase performs the regular agent action (tool call or response)
func (dr *DeliberationRunner) executeActionPhase(ctx context.Context, messages []Message, iteration int) (*ActionResult, error) {
	// Prepare tool definitions
	tools := dr.prepareToolDefinitions()

	// Call LLM with function calling support
	response, err := dr.llmClient.GenerateWithFunctions(
		ctx,
		dr.model,
		dr.buildPromptFromMessages(messages),
		"",
		tools,
	)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Process response
	if response.IsTextResponse {
		// Text response
		newMessage := Message{
			Role:    "assistant",
			Content: response.TextContent,
		}

		isFinal := dr.isFinalAnswer(response.TextContent, iteration)
		return &ActionResult{
			Messages:   []Message{newMessage},
			IsToolCall: false,
			IsFinal:    isFinal,
			Content:    response.TextContent,
		}, nil
	} else {
		// Function call
		functionCall := response.FunctionCall

		// Add function call message
		callMessage := Message{
			Role:     "assistant",
			Content:  "",
			ToolCall: functionCall,
		}

		// Execute tool
		toolResult, err := dr.executeTool(ctx, functionCall)
		resultMessage := Message{
			Role:       "tool",
			ToolCallID: functionCall.ID,
			Name:       functionCall.Name,
		}

		if err != nil {
			resultMessage.Content = fmt.Sprintf("Error executing tool: %v", err)
		} else {
			resultMessage.Content = dr.formatToolResult(toolResult)
		}

		return &ActionResult{
			Messages:   []Message{callMessage, resultMessage},
			IsToolCall: true,
			IsFinal:    false,
			ToolName:   functionCall.Name,
			Content:    resultMessage.Content,
		}, nil
	}
}

// assessActionConfidence evaluates confidence in the action taken
func (dr *DeliberationRunner) assessActionConfidence(ctx context.Context, actionResult *ActionResult, iteration int) (*llm.ConfidenceAssessment, error) {
	// Get the most recent thought
	var recentThought string
	if len(dr.thoughtHistory) > 0 {
		recentThought = dr.thoughtHistory[len(dr.thoughtHistory)-1].Content
	}

	// Build action description
	var proposedAction string
	if actionResult.IsToolCall {
		proposedAction = fmt.Sprintf("Tool execution: %s", actionResult.ToolName)
	} else {
		proposedAction = fmt.Sprintf("Response: %s", actionResult.Content)
	}

	return dr.llmClient.AssessConfidence(ctx, dr.model, recentThought, proposedAction)
}

// generateReflection performs post-completion reflection
func (dr *DeliberationRunner) generateReflection(ctx context.Context, messages []Message, iterations int) {
	// This could generate insights about the decision-making process
	// For now, we'll just log some basic reflection
	reflectionNote := fmt.Sprintf("Completed in %d iterations with %d deliberation steps",
		iterations, len(dr.thoughtHistory))
	dr.reflectionNotes = append(dr.reflectionNotes, reflectionNote)
}

// buildThoughtPrompt constructs a prompt for thought generation
func (dr *DeliberationRunner) buildThoughtPrompt(messages []Message, iteration int) string {
	var parts []string
	parts = append(parts, "Think step by step about the current situation:")

	// Add recent conversation context
	for i := max(0, len(messages)-3); i < len(messages); i++ {
		msg := messages[i]
		if msg.Role != "system" {
			parts = append(parts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
	}

	parts = append(parts, "\nWhat should I consider before taking action? What are the potential risks and benefits?")

	return strings.Join(parts, "\n")
}

// buildContextForThought builds context for thought generation
func (dr *DeliberationRunner) buildContextForThought(messages []Message) string {
	var context strings.Builder
	context.WriteString("Current conversation context:\n")

	for _, msg := range messages {
		if msg.Role != "system" {
			context.WriteString(fmt.Sprintf("- %s: %s\n", msg.Role, msg.Content))
		}
	}

	return context.String()
}

// createDeliberationResult creates the final deliberation result
func (dr *DeliberationRunner) createDeliberationResult(messages []Message, toolCalls, iterations int, success bool, errorMsg string) *DeliberationResult {
	// Calculate average confidence
	var totalConfidence float64
	var confidenceCount int
	for _, step := range dr.thoughtHistory {
		if step.Confidence > 0 {
			totalConfidence += step.Confidence
			confidenceCount++
		}
	}

	var avgConfidence float64
	if confidenceCount > 0 {
		avgConfidence = totalConfidence / float64(confidenceCount)
	}

	// Get final response
	finalResponse := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && messages[i].Content != "" {
			finalResponse = messages[i].Content
			break
		}
	}

	baseResult := &RunResult{
		FinalResponse: finalResponse,
		Messages:      messages,
		ToolCalls:     toolCalls,
		Iterations:    iterations,
		Success:       success,
		Error:         errorMsg,
	}

	return &DeliberationResult{
		RunResult:         baseResult,
		DeliberationSteps: dr.thoughtHistory,
		ThoughtCount:      len(dr.thoughtHistory),
		AverageConfidence: avgConfidence,
		ReflectionNotes:   dr.reflectionNotes,
	}
}

// Helper function for max calculation
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
