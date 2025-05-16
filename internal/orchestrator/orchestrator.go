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
	appCfgPtr := contextkeys.ConfigPtrFromContext(ctx)
	if appCfgPtr == nil {
		if log != nil {
			log.Error("AppConfig not found in context; cannot run agents", "file", filePath)
		}
		return nil, fmt.Errorf("AppConfig not found in context")
	}
	appCfg := *appCfgPtr
	modelForAgents := appCfg.DefaultModel
	if modelForAgents == "" {
		if log != nil {
			log.Warn("No default model configured in AppConfig, agents might fail if they require a model name.")
		}
	}
	if len(agentNames) == 0 {
		if log != nil {
			log.Info("No agents specified to run.", "file", filePath)
		}
		return results, nil
	}
	for _, currentAgentName := range agentNames {
		select {
		case <-ctx.Done():
			if log != nil {
				log.Info("Agent orchestration cancelled by context", "file", filePath, "last_agent_to_run", currentAgentName)
			}
			return results, ctx.Err()
		default:
		}
		agent, err := o.GetAgent(currentAgentName)
		if err != nil {
			if log != nil {
				log.Warn("Requested agent not found, skipping", "agent_name", currentAgentName, "file", filePath, "error", err.Error())
			}
			results = append(results, agents.Result{
				AgentName: currentAgentName,
				File:      filePath,
				Error:     fmt.Errorf("agent '%s' not found during execution: %w", currentAgentName, err),
			})
			continue
		}
		if log != nil {
			log.Info("Running agent", "agent_name", agent.Name(), "file", filePath, "model_name_for_agent", modelForAgents)
		}
		result, agentErr := agent.Analyze(ctx, modelForAgents, filePath, fileContent)
		if agentErr != nil {
			if log != nil {
				log.Error("Agent execution failed", "agent_name", agent.Name(), "file", filePath, "error", agentErr.Error())
			}
			results = append(results, agents.Result{
				AgentName: agent.Name(),
				File:      filePath,
				Error:     agentErr,
			})
			continue
		}
		results = append(results, result)
	}
	if log != nil {
		log.Debug("Finished running agents", "file", filePath, "agents_processed_count", len(agentNames), "results_count", len(results))
	}
	return results, nil
}
