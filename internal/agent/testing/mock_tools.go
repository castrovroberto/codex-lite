package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
)

// MockBehavior defines how a mock tool should behave during execution
type MockBehavior struct {
	ShouldSucceed  bool          `json:"should_succeed"`
	ReturnData     interface{}   `json:"return_data"`
	ExecutionTime  time.Duration `json:"execution_time"`
	ErrorMessage   string        `json:"error_message"`
	CallCount      int           `json:"-"` // Track how many times Execute was called
	MaxCalls       int           `json:"max_calls"`
	ValidateParams bool          `json:"validate_params"`
	ExpectedParams interface{}   `json:"expected_params"`
}

// MockToolConfig holds configuration for creating mock tools
type MockToolConfig struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Behavior    MockBehavior    `json:"behavior"`
}

// MockTool implements the agent.Tool interface for testing
type MockTool struct {
	config MockToolConfig
}

// NewMockTool creates a new mock tool with the given configuration
func NewMockTool(config MockToolConfig) agent.Tool {
	return &MockTool{config: config}
}

// Name returns the tool's name
func (m *MockTool) Name() string {
	return m.config.Name
}

// Description returns the tool's description
func (m *MockTool) Description() string {
	return m.config.Description
}

// Parameters returns the tool's parameter schema
func (m *MockTool) Parameters() json.RawMessage {
	return m.config.Parameters
}

// Execute simulates tool execution based on the configured behavior
func (m *MockTool) Execute(ctx context.Context, params json.RawMessage) (*agent.ToolResult, error) {
	// Increment call count
	m.config.Behavior.CallCount++

	// Check if we've exceeded max calls
	if m.config.Behavior.MaxCalls > 0 && m.config.Behavior.CallCount > m.config.Behavior.MaxCalls {
		return nil, fmt.Errorf("mock tool %s exceeded max calls (%d)", m.config.Name, m.config.Behavior.MaxCalls)
	}

	// Validate parameters if required
	if m.config.Behavior.ValidateParams && m.config.Behavior.ExpectedParams != nil {
		// Parse both the received and expected parameters for proper comparison
		var receivedParams interface{}
		var expectedParams interface{}

		if err := json.Unmarshal(params, &receivedParams); err != nil {
			return nil, fmt.Errorf("mock tool %s received invalid JSON parameters: %v", m.config.Name, err)
		}

		expectedBytes, _ := json.Marshal(m.config.Behavior.ExpectedParams)
		if err := json.Unmarshal(expectedBytes, &expectedParams); err != nil {
			return nil, fmt.Errorf("mock tool %s has invalid expected parameters: %v", m.config.Name, err)
		}

		// Compare the unmarshaled objects
		if !compareJSONObjects(receivedParams, expectedParams) {
			return nil, fmt.Errorf("mock tool %s received unexpected parameters: got %s, expected %s",
				m.config.Name, string(params), string(expectedBytes))
		}
	}

	// Simulate execution time
	if m.config.Behavior.ExecutionTime > 0 {
		select {
		case <-time.After(m.config.Behavior.ExecutionTime):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Return error if configured to fail
	if !m.config.Behavior.ShouldSucceed {
		errorMsg := m.config.Behavior.ErrorMessage
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("mock tool %s failed as configured", m.config.Name)
		}
		return &agent.ToolResult{
			Success: false,
			Error:   errorMsg,
		}, nil
	}

	// Return success with configured data
	return &agent.ToolResult{
		Success: true,
		Data:    m.config.Behavior.ReturnData,
	}, nil
}

// GetCallCount returns how many times Execute was called
func (m *MockTool) GetCallCount() int {
	return m.config.Behavior.CallCount
}

// ResetCallCount resets the call counter
func (m *MockTool) ResetCallCount() {
	m.config.Behavior.CallCount = 0
}

// Predefined Mock Tool Configurations

// NewMockReadFileTool creates a mock read_file tool
func NewMockReadFileTool(content string, shouldSucceed bool) agent.Tool {
	return NewMockTool(MockToolConfig{
		Name:        "read_file",
		Description: "Mock read file tool",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"target_file": {"type": "string"}}}`),
		Behavior: MockBehavior{
			ShouldSucceed: shouldSucceed,
			ReturnData:    map[string]interface{}{"content": content},
		},
	})
}

// NewMockWriteFileTool creates a mock write_file tool
func NewMockWriteFileTool(shouldSucceed bool) agent.Tool {
	return NewMockTool(MockToolConfig{
		Name:        "write_file",
		Description: "Mock write file tool",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"file_path": {"type": "string"}, "content": {"type": "string"}}}`),
		Behavior: MockBehavior{
			ShouldSucceed: shouldSucceed,
			ReturnData:    map[string]interface{}{"bytes_written": 100},
		},
	})
}

// NewMockShellRunTool creates a mock shell command tool
func NewMockShellRunTool(output string, exitCode int, shouldSucceed bool) agent.Tool {
	return NewMockTool(MockToolConfig{
		Name:        "run_shell_command",
		Description: "Mock shell command tool",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"command": {"type": "string"}}}`),
		Behavior: MockBehavior{
			ShouldSucceed: shouldSucceed,
			ReturnData: map[string]interface{}{
				"stdout":    output,
				"stderr":    "",
				"exit_code": exitCode,
			},
		},
	})
}

// NewMockTestRunnerTool creates a mock test runner tool
func NewMockTestRunnerTool(testOutput string, passed bool) agent.Tool {
	return NewMockTool(MockToolConfig{
		Name:        "run_tests",
		Description: "Mock test runner tool",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"test_command": {"type": "string"}}}`),
		Behavior: MockBehavior{
			ShouldSucceed: true,
			ReturnData: map[string]interface{}{
				"output":     testOutput,
				"passed":     passed,
				"test_count": 5,
				"failed_count": func() int {
					if passed {
						return 0
					} else {
						return 2
					}
				}(),
			},
		},
	})
}

// NewMockGitCommitTool creates a mock git commit tool
func NewMockGitCommitTool(commitHash string, shouldSucceed bool) agent.Tool {
	return NewMockTool(MockToolConfig{
		Name:        "git_commit",
		Description: "Mock git commit tool",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}}`),
		Behavior: MockBehavior{
			ShouldSucceed: shouldSucceed,
			ReturnData: map[string]interface{}{
				"commit_hash":  commitHash,
				"files_staged": []string{"file1.go", "file2.go"},
			},
		},
	})
}

// MockToolRegistry provides a registry of mock tools for testing
type MockToolRegistry struct {
	tools map[string]agent.Tool
}

// NewMockToolRegistry creates a new mock tool registry
func NewMockToolRegistry() *MockToolRegistry {
	return &MockToolRegistry{
		tools: make(map[string]agent.Tool),
	}
}

// Register adds a mock tool to the registry
func (r *MockToolRegistry) Register(tool agent.Tool) error {
	r.tools[tool.Name()] = tool
	return nil
}

// Get retrieves a tool by name
func (r *MockToolRegistry) Get(name string) (agent.Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *MockToolRegistry) List() []agent.Tool {
	tools := make([]agent.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Clear removes all tools from the registry
func (r *MockToolRegistry) Clear() {
	r.tools = make(map[string]agent.Tool)
}

// GetCallCounts returns call counts for all mock tools
func (r *MockToolRegistry) GetCallCounts() map[string]int {
	counts := make(map[string]int)
	for name, tool := range r.tools {
		if mockTool, ok := tool.(*MockTool); ok {
			counts[name] = mockTool.GetCallCount()
		}
	}
	return counts
}

// ResetAllCallCounts resets call counters for all mock tools
func (r *MockToolRegistry) ResetAllCallCounts() {
	for _, tool := range r.tools {
		if mockTool, ok := tool.(*MockTool); ok {
			mockTool.ResetCallCount()
		}
	}
}

// compareJSONObjects compares two JSON objects for equality
func compareJSONObjects(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
