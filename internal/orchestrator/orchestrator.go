package orchestrator

import (
	"context"
	"fmt"
	//"log/slog" // Using slog from the context

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/contextkeys" // For retrieving values from context
	// No direct import of "github.com/castrovroberto/codex-lite/internal/config" needed here
	// as appCfg is retrieved from context and its type is handled by contextkeys.
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
	) ([]agents.Result, error) // error is for orchestrator-level failures
}

// BasicOrchestrator is a simple implementation of the Orchestrator interface.
type BasicOrchestrator struct {
	agents map[string]agents.Agent
}

// NewBasicOrchestrator creates a new instance of BasicOrchestrator.
// It returns an Orchestrator interface, satisfied by *BasicOrchestrator.
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
	// Optionally log agent registration
	// log := contextkeys.LoggerFromContext(context.Background()) // Or pass context if available
	// log.Debug("Agent registered", "agent_name", name)
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

// RunAgents executes the specified agents for a given file.
// It retrieves configuration and logger from the provided context.
func (o *BasicOrchestrator) RunAgents(
	ctx context.Context,
	agentNames []string,
	filePath string,
	fileContent string,
) ([]agents.Result, error) {
	results := make([]agents.Result, 0, len(agentNames))
	log := contextkeys.LoggerFromContext(ctx)
	appCfg := contextkeys.ConfigFromContext(ctx) // Retrieves config.AppConfig

	// Use the DefaultModel field from AppConfig
	modelForAgents := appCfg.DefaultModel
	if modelForAgents == "" {
		log.Warn("No default model configured in AppConfig, agents might fail if they require a model name.")
		// Depending on agent requirements, you might set a fallback or return an error.
		// For now, we'll let agents handle missing model names if they can.
	} else {
		log.Debug("Orchestrator using model for agents", "model", modelForAgents)
	}

	if len(agentNames) == 0 {
		log.Info("No agents specified to run.", "file", filePath)
		return results, nil // Or return an error if agents are mandatory
	}

	for _, currentAgentName := range agentNames {
		select {
		case <-ctx.Done():
			log.Info("Agent orchestration cancelled by context", "file", filePath, "last_agent_to_run", currentAgentName)
			return results, ctx.Err()
		default:
			// Continue processing
		}

		agent, err := o.GetAgent(currentAgentName)
		if err != nil {
			log.Warn("Requested agent not found, skipping", "agent_name", currentAgentName, "file", filePath, "error", err.Error())
			results = append(results, agents.Result{
				AgentName: currentAgentName,
				File:      filePath,
				Error:     fmt.Errorf("agent '%s' not found during execution: %w", currentAgentName, err),
			})
			continue
		}

		log.Info("Running agent", "agent_name", agent.Name(), "file", filePath, "model_name_for_agent", modelForAgents)

		// Pass the model name (modelForAgents) to the agent's Analyze method
		result, agentErr := agent.Analyze(ctx, modelForAgents, filePath, fileContent)
		if agentErr != nil {
			log.Error("Agent execution failed", "agent_name", agent.Name(), "file", filePath, "error", agentErr.Error())
			// Append a result even if there's an error from the agent
			results = append(results, agents.Result{
				AgentName: agent.Name(),
				File:      filePath,
				Error:     agentErr, // Store the agent's error
			})
			// Decide if one agent error should stop all others or just continue
			continue
		}
		results = append(results, result)
	}

	log.Debug("Finished running agents", "file", filePath, "agents_processed_count", len(agentNames), "results_count", len(results))
	return results, nil
}