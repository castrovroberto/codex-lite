package orchestrator

import (
	"context"
	"fmt"
	// "log/slog" // For logging within the orchestrator itself if needed, separate from context logger for now

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/contextkeys" // For retrieving values from context
)

// Orchestrator defines the interface for managing and running agents.
type Orchestrator interface {
	RegisterAgent(agent agents.Agent) error
	GetAgent(name string) (agents.Agent, error)
	AvailableAgentNames() []string
	RunAgents(
		ctx context.Context, // This context will have config and logger
		agentNames []string,
		filePath string,
		fileContent string,
	) ([]agents.Result, error) // error is for orchestrator-level failures like context cancellation
}

// BasicOrchestrator is a simple implementation of the Orchestrator interface.
type BasicOrchestrator struct {
	agents map[string]agents.Agent
	// log    *slog.Logger // Orchestrator's own logger, if needed distinct from context one. For now, use context's.
}

// NewBasicOrchestrator creates a new instance of BasicOrchestrator.
func NewBasicOrchestrator() Orchestrator {
	return &BasicOrchestrator{
		agents: make(map[string]agents.Agent),
	}
}

// RegisterAgent adds an agent to the orchestrator.
// Returns an error if an agent with the same name already exists.
func (o *BasicOrchestrator) RegisterAgent(agent agents.Agent) error {
	name := agent.Name()
	if _, exists := o.agents[name]; exists {
		return fmt.Errorf("agent '%s' already registered", name)
	}
	o.agents[name] = agent
	return nil
}

// GetAgent retrieves a registered agent by name.
// Returns an error if the agent is not found.
func (o *BasicOrchestrator) GetAgent(name string) (agents.Agent, error) {
	agent, ok := o.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent '%s' not found", name)
	}
	return agent, nil
}

// AvailableAgentNames returns a slice of names of all registered agents.
func (o *BasicOrchestrator) AvailableAgentNames() []string {
	names := make([]string, 0, len(o.agents))
	for name := range o.agents {
		names = append(names, name)
	}
	return names
}

// RunAgents executes the specified agents sequentially for the given file content.
// It collects results and errors for each agent run.
func (o *BasicOrchestrator) RunAgents(
	ctx context.Context,
	agentNames []string,
	filePath string,
	fileContent string,
) ([]agents.Result, error) {
	results := make([]agents.Result, 0, len(agentNames))
	log := contextkeys.LoggerFromContext(ctx)    // Retrieve logger from context
	appCfg := contextkeys.ConfigFromContext(ctx) // Retrieve config from context

	modelToUse := appCfg.ModelName

	for _, name := range agentNames {
		// Check for context cancellation before running the next agent
		select {
		case <-ctx.Done():
			log.Info("Agent orchestration cancelled", "file", filePath, "agent_to_run", name)
			return results, ctx.Err() // Return already collected results and context error
		default:
			// Continue
		}

		agent, err := o.GetAgent(name)
		if err != nil {
			log.Warn("Requested agent not found, skipping", "agent_name", name, "file", filePath)
			results = append(results, agents.Result{
				AgentName: name,
				File:      filePath,
				Error:     fmt.Errorf("agent '%s' not found during execution", name),
			})
			continue // Skip to the next agent
		}

		log.Info("Running agent", "agent_name", agent.Name(), "file", filePath, "model_name", modelToUse)

		// Pass the enriched context down to the agent's Analyze method
		result, agentErr := agent.Analyze(ctx, modelToUse, filePath, fileContent)

		if agentErr != nil {
			log.Error("Agent analysis returned an error", "agent_name", agent.Name(), "file", filePath, "error", agentErr)
			// Ensure AgentName and File are set in the result, even if Analyze returned a partially filled Result
			if result.AgentName == "" {
				result.AgentName = agent.Name()
			}
			if result.File == "" {
				result.File = filePath
			}
			result.Error = agentErr // Store the error in the result struct
		} else {
			log.Info("Agent analysis complete", "agent_name", agent.Name(), "file",filePath)
			// Ensure AgentName and File are set on success as well, for consistency
			if result.AgentName == "" {
				result.AgentName = agent.Name()
			}
			if result.File == "" {
				result.File = filePath
			}
		}
		results = append(results, result)
	}

	// RunAgents collects all results/errors; a non-nil error return from RunAgents itself
	// indicates an orchestrator-level failure (like context cancellation, handled above).
	return results, nil
}