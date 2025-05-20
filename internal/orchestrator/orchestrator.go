package orchestrator

import (
	"context"
	"errors"
	"fmt"  // For sorting agent names if needed, and potentially results if order is critical and not guaranteed
	"sync" // For WaitGroup and Mutex if needed
	"time"

	//"log/slog" // Using slog from the context

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/contextkeys" // For retrieving values from context
	// No direct import of "github.com/castrovroberto/codex-lite/internal/config" needed here
	// as appCfg is retrieved from context and its type is handled by contextkeys.
)

// AgentStatus defines the type for agent progress status.
const (
	StatusStarting  = "STARTING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
	StatusTimedOut  = "TIMED_OUT"
	StatusSkipped   = "SKIPPED"
)

// AgentProgressUpdate is sent by the orchestrator to report progress of agent execution.
// FilePath, AgentIndex, and TotalAgents are for context by the receiver.
type AgentProgressUpdate struct {
	AgentName   string
	AgentIndex  int           // Original index of the agent for a given file run
	TotalAgents int           // Total agents scheduled for this file run
	Status      string        // e.g., "STARTING", "COMPLETED", "FAILED", "TIMED_OUT"
	Duration    time.Duration // Only for COMPLETED, FAILED, TIMED_OUT statuses
	Error       error         // If Status is FAILED or TIMED_OUT
	FilePath    string        // The file being processed
}

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
		progressChan chan<- AgentProgressUpdate, // Channel to send progress updates
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

// indexedResult is a helper struct to pass results along with their original index.
type indexedResult struct {
	index  int
	result agents.Result
}

// RunAgents executes the specified agents for a given file, with concurrency.
func (o *BasicOrchestrator) RunAgents(
	ctx context.Context,
	agentNames []string,
	filePath string,
	fileContent string,
	progressChan chan<- AgentProgressUpdate,
) ([]agents.Result, error) {
	log := contextkeys.LoggerFromContext(ctx)
	appCfgPtr := contextkeys.ConfigPtrFromContext(ctx)
	if appCfgPtr == nil {
		if log != nil {
			log.Error("AppConfig not found in context; cannot run agents", "file", filePath)
		}
		return nil, fmt.Errorf("AppConfig not found in context")
	}
	appCfg := *appCfgPtr // Dereference once
	modelForAgents := appCfg.DefaultModel

	numAgents := len(agentNames)
	if numAgents == 0 {
		if log != nil {
			log.Info("No agents specified to run.", "file", filePath)
		}
		return []agents.Result{}, nil
	}

	// Determine concurrency level
	concurrency := appCfg.MaxAgentConcurrency
	if concurrency < 1 {
		concurrency = 1 // Ensure at least 1 worker
	}
	if concurrency > numAgents {
		concurrency = numAgents // Don't create more workers than agents
	}

	if log != nil {
		log.Info("Orchestrator starting RunAgents", "file", filePath, "num_agents_requested", numAgents, "concurrency_level", concurrency)
	}

	var wg sync.WaitGroup
	resultsChan := make(chan indexedResult, numAgents) // Buffered channel for results
	semaphore := make(chan struct{}, concurrency)      // Semaphore to limit concurrency

	orderedResults := make([]agents.Result, numAgents)

	for i, agentName := range agentNames {
		wg.Add(1)
		go func(agentIdx int, currentAgentName string) {
			defer wg.Done()
			startTime := time.Now()

			sendProgress := func(status string, duration time.Duration, err error) {
				if progressChan != nil {
					progressChan <- AgentProgressUpdate{
						AgentName:   currentAgentName,
						AgentIndex:  agentIdx,
						TotalAgents: numAgents,
						Status:      status,
						Duration:    duration,
						Error:       err,
						FilePath:    filePath,
					}
				}
			}

			// Acquire semaphore slot
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }() // Release semaphore slot when done
			case <-ctx.Done():
				if log != nil {
					log.Info("Agent execution skipped due to parent context cancellation before acquiring semaphore",
						"agent_name", currentAgentName, "file", filePath)
				}
				sendProgress(StatusSkipped, 0, ctx.Err())
				resultsChan <- indexedResult{
					index: agentIdx,
					result: agents.Result{
						AgentName: currentAgentName,
						File:      filePath,
						Error:     ctx.Err(),
					},
				}
				return
			}

			// Create a new context with timeout for this specific agent execution
			var agentCtx context.Context
			var cancelAgentCtx context.CancelFunc

			if appCfg.AgentTimeout > 0 {
				agentCtx, cancelAgentCtx = context.WithTimeout(ctx, appCfg.AgentTimeout)
			} else {
				agentCtx, cancelAgentCtx = context.WithCancel(ctx)
			}
			defer cancelAgentCtx()

			// Check agent-specific context before heavy work (e.g. if timeout is very short or parent ctx got cancelled)
			select {
			case <-agentCtx.Done():
				duration := time.Since(startTime)
				status := StatusSkipped
				if agentCtx.Err() == context.DeadlineExceeded {
					status = StatusTimedOut
				}
				if log != nil {
					log.Info("Agent execution cancelled or timed out by agent-specific context before starting work",
						"agent_name", currentAgentName, "file", filePath, "error", agentCtx.Err(), "duration", duration, "status", status)
				}
				sendProgress(status, duration, agentCtx.Err())
				resultsChan <- indexedResult{
					index: agentIdx,
					result: agents.Result{
						AgentName: currentAgentName,
						File:      filePath,
						Error:     agentCtx.Err(),
					},
				}
				return
			default:
			}

			agent, err := o.GetAgent(currentAgentName)
			if err != nil {
				duration := time.Since(startTime)
				if log != nil {
					log.Warn("Requested agent not found, skipping",
						"agent_name", currentAgentName, "file", filePath, "error", err, "duration", duration)
				}
				errWrapped := fmt.Errorf("agent '%s' not found: %w", currentAgentName, err)
				sendProgress(StatusFailed, duration, errWrapped)
				resultsChan <- indexedResult{
					index: agentIdx,
					result: agents.Result{
						AgentName: currentAgentName,
						File:      filePath,
						Error:     errWrapped,
					},
				}
				return
			}

			sendProgress(StatusStarting, 0, nil)
			if log != nil {
				log.Info("Running agent", "agent_name", agent.Name(), "file", filePath, "model_name_for_agent", modelForAgents, "timeout", appCfg.AgentTimeout)
			}

			result, agentErr := agent.Analyze(agentCtx, modelForAgents, filePath, fileContent)
			duration := time.Since(startTime)

			// Check if the agent-specific context timed out, if agentErr is nil or not context.DeadlineExceeded itself
			if agentErr == nil && agentCtx.Err() == context.DeadlineExceeded {
				agentErr = context.DeadlineExceeded
				if log != nil {
					log.Warn("Agent execution timed out via agent-specific context",
						"agent_name", agent.Name(), "file", filePath, "timeout", appCfg.AgentTimeout, "duration", duration)
				}
			}

			if agentErr != nil {
				status := StatusFailed
				if agentErr == context.DeadlineExceeded || (agentCtx.Err() == context.DeadlineExceeded && errors.Is(agentErr, context.Canceled)) {
					// If agentErr is DeadlineExceeded, or if agentCtx timed out and agentErr is Canceled (often happens when agent respects context)
					status = StatusTimedOut
					agentErr = context.DeadlineExceeded // Standardize to DeadlineExceeded for timeout
				}
				if log != nil {
					log.Error("Agent execution failed",
						"agent_name", agent.Name(), "file", filePath, "error", agentErr, "duration", duration, "status", status)
				}
				sendProgress(status, duration, agentErr)
				// Ensure result is not nil even on error, populate required fields
				if result.AgentName == "" {
					result.AgentName = agent.Name()
				}
				if result.File == "" {
					result.File = filePath
				}
				result.Error = agentErr
			} else {
				sendProgress(StatusCompleted, duration, nil)
			}
			resultsChan <- indexedResult{index: agentIdx, result: result}
		}(i, agentName)
	}

	// Goroutine to close resultsChan once all agent goroutines are done.
	go func() {
		wg.Wait()
		close(resultsChan)
		// Also close progressChan once all agents are done and all updates have been sent.
		// This happens after wg.Wait() ensures all agent goroutines (and thus all sendProgress calls) have completed.
		if progressChan != nil {
			close(progressChan)
		}
	}()

	// Collect results from the channel and place them in order.
	for iResult := range resultsChan {
		if iResult.index >= 0 && iResult.index < numAgents {
			orderedResults[iResult.index] = iResult.result
		} else {
			// This case should ideally not happen if indexing is correct.
			if log != nil {
				log.Error("Received result with out-of-bounds index",
					"index", iResult.index, "num_agents", numAgents, "agent_name", iResult.result.AgentName)
			}
			// Decide how to handle: append, ignore, or error out.
			// For now, appending to ensure data isn't lost, but this breaks strict ordering for this item.
			// orderedResults = append(orderedResults, iResult.result) // This would break fixed size slice
		}
	}

	if log != nil {
		log.Debug("Finished running agents in parallel", "file", filePath, "agents_processed_count", numAgents, "results_count", len(orderedResults))
	}
	return orderedResults, nil
}
