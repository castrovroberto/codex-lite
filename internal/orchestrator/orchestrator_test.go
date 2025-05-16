package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/castrovroberto/codex-lite/internal/agents"
	"github.com/castrovroberto/codex-lite/internal/config"
	"github.com/castrovroberto/codex-lite/internal/contextkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Agents ---

type mockAgent struct {
	name        string
	description string
	analyzeFunc func(ctx context.Context, modelName, filePath, fileContent string) (agents.Result, error)
	err         error // If non-nil, Analyze returns this error
	output      string
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Description() string { return m.description }
func (m *mockAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (agents.Result, error) {
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, modelName, filePath, fileContent)
	}
	if m.err != nil {
		return agents.Result{AgentName: m.name, File: filePath, Output: "", Error: m.err}, m.err
	}
	return agents.Result{
		AgentName: m.name,
		File:      filePath,
		Output:    fmt.Sprintf("%s output for %s (model: %s)", m.output, filePath, modelName),
		Error:     nil,
	}, nil
}

type spyAgent struct {
	name            string
	description     string
	mu              sync.Mutex
	capturedCtx     context.Context
	capturedModel   string
	capturedPath    string
	capturedContent string
}

func (s *spyAgent) Name() string        { return s.name }
func (s *spyAgent) Description() string { return s.description }
func (s *spyAgent) Analyze(ctx context.Context, modelName, filePath, fileContent string) (agents.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capturedCtx = ctx
	s.capturedModel = modelName
	s.capturedPath = filePath
	s.capturedContent = fileContent
	return agents.Result{
		AgentName: s.name,
		File:      filePath,
		Output:    "spy agent output",
		Error:     nil,
	}, nil
}

// --- Tests ---

func TestNewBasicOrchestrator(t *testing.T) {
	o := NewBasicOrchestrator()
	require.NotNil(t, o, "NewBasicOrchestrator should return a non-nil orchestrator")
	_, ok := o.(*BasicOrchestrator)
	require.True(t, ok, "NewBasicOrchestrator should return a *BasicOrchestrator")
	// Instead of checking the unexported field, check the public API:
	assert.Empty(t, o.AvailableAgentNames(), "New orchestrator should have no agents registered")
}

func TestRegisterAgent(t *testing.T) {
	o := NewBasicOrchestrator()
	agent1 := &mockAgent{name: "agent1", description: "Test Agent 1"}

	// Test successful registration
	err := o.RegisterAgent(agent1)
	require.NoError(t, err, "RegisterAgent should not return an error for a new agent")

	retrievedAgent, getErr := o.GetAgent("agent1")
	require.NoError(t, getErr, "GetAgent should retrieve a registered agent")
	assert.Equal(t, agent1, retrievedAgent, "Retrieved agent should be the one registered")

	// Test error on duplicate registration
	agent1Duplicate := &mockAgent{name: "agent1", description: "Duplicate Test Agent 1"}
	err = o.RegisterAgent(agent1Duplicate)
	require.Error(t, err, "RegisterAgent should return an error for a duplicate agent name")
	assert.Contains(t, err.Error(), "agent 'agent1' already registered", "Error message should indicate duplicate registration")
}

func TestGetAgent(t *testing.T) {
	o := NewBasicOrchestrator()
	agent1 := &mockAgent{name: "agent1", description: "Test Agent 1"}
	err := o.RegisterAgent(agent1)
	require.NoError(t, err)

	// Test successful retrieval
	retrievedAgent, err := o.GetAgent("agent1")
	require.NoError(t, err, "GetAgent should not return an error for an existing agent")
	assert.Equal(t, agent1, retrievedAgent, "Retrieved agent should match the registered one")

	// Test error on retrieving non-existent agent
	_, err = o.GetAgent("nonexistent-agent")
	require.Error(t, err, "GetAgent should return an error for a non-existent agent")
	assert.Contains(t, err.Error(), "agent 'nonexistent-agent' not found", "Error message should indicate agent not found")
}

func TestAvailableAgentNames(t *testing.T) {
	o := NewBasicOrchestrator()

	// Test with no agents registered
	names := o.AvailableAgentNames()
	assert.Empty(t, names, "AvailableAgentNames should return an empty slice when no agents are registered")

	// Test with one or more agents registered
	agent1 := &mockAgent{name: "agent1"}
	agent2 := &mockAgent{name: "agent2-beta"}
	agent3 := &mockAgent{name: "agent3-alpha"}

	require.NoError(t, o.RegisterAgent(agent1))
	require.NoError(t, o.RegisterAgent(agent2))
	require.NoError(t, o.RegisterAgent(agent3))

	expectedNames := []string{"agent1", "agent2-beta", "agent3-alpha"}
	actualNames := o.AvailableAgentNames()

	sort.Strings(expectedNames)
	sort.Strings(actualNames)
	assert.Equal(t, expectedNames, actualNames, "AvailableAgentNames should return the names of all registered agents")
}

func TestRunAgents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // Discard logs for most tests
	baseAppCfg := &config.AppConfig{DefaultModel: "test-model-from-config"}

	createTestContext := func(appCfg *config.AppConfig) context.Context {
		ctx := context.Background()
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, logger)
		if appCfg != nil {
			ctx = context.WithValue(ctx, contextkeys.ConfigKey, appCfg)
		}
		return ctx
	}

	filePath := "test/file.go"
	fileContent := "package test\n\nfunc main() {}"

	t.Run("SingleSuccessfulAgent", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		require.NoError(t, o.RegisterAgent(agentA))

		ctx := createTestContext(baseAppCfg)
		results, err := o.RunAgents(ctx, []string{"agentA"}, filePath, fileContent)

		require.NoError(t, err, "Orchestrator should not return an error")
		require.Len(t, results, 1, "Should have one result")
		assert.Equal(t, "agentA", results[0].AgentName)
		assert.Equal(t, filePath, results[0].File)
		assert.Contains(t, results[0].Output, "AgentA output for test/file.go (model: test-model-from-config)")
		assert.NoError(t, results[0].Error)
	})

	t.Run("MultipleSuccessfulAgents", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		agentB := &mockAgent{name: "agentB", output: "AgentB"}
		require.NoError(t, o.RegisterAgent(agentA))
		require.NoError(t, o.RegisterAgent(agentB))

		ctx := createTestContext(baseAppCfg)
		results, err := o.RunAgents(ctx, []string{"agentA", "agentB"}, filePath, fileContent)

		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.Equal(t, "agentA", results[0].AgentName)
		assert.Contains(t, results[0].Output, "AgentA output")
		assert.NoError(t, results[0].Error)

		assert.Equal(t, "agentB", results[1].AgentName)
		assert.Contains(t, results[1].Output, "AgentB output")
		assert.NoError(t, results[1].Error)
	})

	t.Run("AgentReturnsError", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		agentBError := errors.New("agent B failed")
		agentB := &mockAgent{name: "agentB", err: agentBError}
		agentC := &mockAgent{name: "agentC", output: "AgentC"}

		require.NoError(t, o.RegisterAgent(agentA))
		require.NoError(t, o.RegisterAgent(agentB))
		require.NoError(t, o.RegisterAgent(agentC))

		ctx := createTestContext(baseAppCfg)
		results, err := o.RunAgents(ctx, []string{"agentA", "agentB", "agentC"}, filePath, fileContent)

		require.NoError(t, err, "Orchestrator itself should not error if an agent fails")
		require.Len(t, results, 3)

		assert.Equal(t, "agentA", results[0].AgentName)
		assert.NoError(t, results[0].Error)
		assert.NotEmpty(t, results[0].Output)

		assert.Equal(t, "agentB", results[1].AgentName)
		assert.ErrorIs(t, results[1].Error, agentBError)
		assert.Empty(t, results[1].Output)

		assert.Equal(t, "agentC", results[2].AgentName)
		assert.NoError(t, results[2].Error)
		assert.NotEmpty(t, results[2].Output, "AgentC should run after AgentB's failure")
	})

	t.Run("UnregisteredAgent", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		require.NoError(t, o.RegisterAgent(agentA))

		ctx := createTestContext(baseAppCfg)
		results, err := o.RunAgents(ctx, []string{"agentA", "unknown-agent"}, filePath, fileContent)

		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.Equal(t, "agentA", results[0].AgentName)
		assert.NoError(t, results[0].Error)

		assert.Equal(t, "unknown-agent", results[1].AgentName)
		assert.Error(t, results[1].Error)
		assert.Contains(t, results[1].Error.Error(), "agent 'unknown-agent' not found during execution")
		// Instead of accessing the internal orchestrator, check error type and message only
	})

	t.Run("EmptyAgentList", func(t *testing.T) {
		o := NewBasicOrchestrator()
		ctx := createTestContext(baseAppCfg)
		results, err := o.RunAgents(ctx, []string{}, filePath, fileContent)

		require.NoError(t, err)
		assert.Empty(t, results, "Should return empty results for an empty agent list")
	})

	t.Run("ContextPropagationAndConfigUsage", func(t *testing.T) {
		o := NewBasicOrchestrator()
		spy := &spyAgent{name: "spy"}
		require.NoError(t, o.RegisterAgent(spy))

		customModel := "custom-gpt4"
		appCfgWithCustomModel := &config.AppConfig{DefaultModel: customModel}
		ctx := createTestContext(appCfgWithCustomModel)

		_, err := o.RunAgents(ctx, []string{"spy"}, filePath, fileContent)
		require.NoError(t, err)

		spy.mu.Lock()
		defer spy.mu.Unlock()

		assert.NotNil(t, spy.capturedCtx, "Context should be passed to agent")
		assert.Equal(t, logger, spy.capturedCtx.Value(contextkeys.LoggerKey), "Logger from context should be passed")
		assert.Equal(t, appCfgWithCustomModel, spy.capturedCtx.Value(contextkeys.ConfigKey), "AppConfig from context should be passed")
		assert.Equal(t, customModel, spy.capturedModel, "DefaultModel from AppConfig should be used")
		assert.Equal(t, filePath, spy.capturedPath)
		assert.Equal(t, fileContent, spy.capturedContent)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		o := NewBasicOrchestrator()

		blockerChan := make(chan struct{}) // To control when the agent finishes
		agentStartedChan := make(chan struct{}, 1)

		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		agentB := &mockAgent{
			name:   "agentB-cancellable",
			output: "AgentB",
			analyzeFunc: func(ctx context.Context, modelName, filePath, fileContent string) (agents.Result, error) {
				agentStartedChan <- struct{}{} // Signal that agentB has started
				select {
				case <-ctx.Done():
					return agents.Result{AgentName: "agentB-cancellable", File: filePath, Output: "", Error: ctx.Err()}, ctx.Err()
				case <-blockerChan: // Wait until test unblocks
					return agents.Result{AgentName: "agentB-cancellable", File: filePath, Output: "agentB output", Error: nil}, nil
				}
			},
		}
		agentC := &mockAgent{name: "agentC", output: "AgentC"}

		require.NoError(t, o.RegisterAgent(agentA))
		require.NoError(t, o.RegisterAgent(agentB))
		require.NoError(t, o.RegisterAgent(agentC))

		ctx, cancel := context.WithCancel(createTestContext(baseAppCfg))
		defer cancel()

		var results []agents.Result
		var runErr error
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			results, runErr = o.RunAgents(ctx, []string{"agentA", "agentB-cancellable", "agentC"}, filePath, fileContent)
		}()

		// Wait for agentB to start
		select {
		case <-agentStartedChan:
			// Agent B has started, now cancel the context
			cancel()
			// Unblock agent B so it can observe the cancellation
			close(blockerChan)
		case <-time.After(2 * time.Second): // Timeout to prevent test hanging
			t.Fatal("Timed out waiting for agentB to start")
			// Ensure agentB is unblocked if it did start late, and cancel context
			cancel()
			close(blockerChan)
		}

		wg.Wait() // Wait for RunAgents goroutine to finish

		// Orchestrator level error should be context.Canceled
		require.ErrorIs(t, runErr, context.Canceled, "Orchestrator RunAgents should return context.Canceled")

		// Results should contain agentA (success) and agentB (canceled)
		// agentC should not have run.
		require.Len(t, results, 2, "Should have results for agentA and agentB")

		// Agent A should succeed
		assert.Equal(t, "agentA", results[0].AgentName)
		assert.NoError(t, results[0].Error, "AgentA should succeed before cancellation affected orchestrator loop")
		assert.Contains(t, results[0].Output, "AgentA output")

		// Agent B's Analyze method should have returned context.Canceled
		assert.Equal(t, "agentB-cancellable", results[1].AgentName)
		assert.ErrorIs(t, results[1].Error, context.Canceled, "AgentB's result should indicate context.Canceled")
		assert.Empty(t, results[1].Output)
	})

	t.Run("MissingAppConfigInContext", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		require.NoError(t, o.RegisterAgent(agentA))

		// Create context without AppConfig
		ctx := context.Background()
		ctx = context.WithValue(ctx, contextkeys.LoggerKey, logger)
		// Deliberately omit contextkeys.ConfigKey

		results, err := o.RunAgents(ctx, []string{"agentA"}, filePath, fileContent)

		// This assertion depends on the BasicOrchestrator's implementation.
		// Based on the assumed implementation in thought process, it should error out.
		// If BasicOrchestrator is more lenient (e.g. empty modelName), this test needs adjustment.
		// The requirement "orchestrator uses the DefaultModel from AppConfig" implies AppConfig is mandatory.
		require.Error(t, err, "Orchestrator should return an error if AppConfig is missing from context")
		assert.True(t, strings.Contains(err.Error(), "AppConfig not found in context") || strings.Contains(err.Error(), "config not found in context"), "Error message should indicate missing AppConfig")
		assert.Empty(t, results, "Results should be empty if orchestrator fails due to missing config")
	})

	t.Run("ContextCancelledBeforeRun", func(t *testing.T) {
		o := NewBasicOrchestrator()
		agentA := &mockAgent{name: "agentA", output: "AgentA"}
		require.NoError(t, o.RegisterAgent(agentA))

		ctx, cancel := context.WithCancel(createTestContext(baseAppCfg))
		cancel() // Cancel immediately

		results, err := o.RunAgents(ctx, []string{"agentA"}, filePath, fileContent)

		require.ErrorIs(t, err, context.Canceled, "Orchestrator RunAgents should return context.Canceled")
		// Depending on when the check happens, results might be empty or contain the first agent if it's super fast.
		// The current assumed orchestrator checks at the start of the loop for each agent.
		// If agentNames is not empty, it will enter the loop, check context, and return.
		assert.Empty(t, results, "Results should be empty if context is cancelled before any agent processing starts")
	})
}
