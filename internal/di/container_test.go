package di

import (
	"context"
	"testing"

	"github.com/castrovroberto/CGE/internal/agent"
	"github.com/castrovroberto/CGE/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainer_DependencyInjection(t *testing.T) {
	t.Run("container_with_real_dependencies", func(t *testing.T) {
		cfg := &config.AppConfig{}
		cfg.LLM.Provider = "ollama"
		cfg.LLM.Model = "test-model"
		cfg.Project.WorkspaceRoot = "/tmp/test"

		container := NewContainer(cfg)

		// Test that we can get an LLM client
		llmClient := container.GetLLMClient()
		assert.NotNil(t, llmClient)

		// Test that we can get a tool registry
		toolRegistry := container.GetToolRegistry()
		assert.NotNil(t, toolRegistry)

		// Test that we can get a chat presenter
		chatPresenter := container.GetChatPresenter(context.Background(), "test-model", "test prompt")
		assert.NotNil(t, chatPresenter)
	})

	t.Run("container_with_mock_dependencies", func(t *testing.T) {
		cfg := &config.AppConfig{}
		cfg.LLM.Provider = "ollama"
		cfg.LLM.Model = "test-model"
		cfg.Project.WorkspaceRoot = "/tmp/test"

		// Create container with mock dependencies
		container := NewContainer(cfg).
			WithFileSystem(NewMockFileSystemService()).
			WithCommandExecutor(NewMockCommandExecutor()).
			WithHTTPClient(NewMockHTTPClient()).
			WithSessionStore(NewMockSessionStore())

		// Test that tool registry uses mock file system
		toolRegistry := container.GetToolRegistry()
		assert.NotNil(t, toolRegistry)

		// Test that we can get tools that use injected dependencies
		tools := toolRegistry.List()
		assert.Greater(t, len(tools), 0)
	})
}

func TestMockFileSystemService(t *testing.T) {
	fs := NewMockFileSystemService()

	t.Run("write_and_read_file", func(t *testing.T) {
		content := []byte("test content")
		path := "/test/file.txt"

		// Write file
		err := fs.WriteFile(path, content, 0644)
		require.NoError(t, err)

		// Check if file exists
		assert.True(t, fs.Exists(path))

		// Read file back
		readContent, err := fs.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, readContent)

		// Get file info
		info, err := fs.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, int64(len(content)), info.Size())
		assert.False(t, info.IsDir())
	})

	t.Run("create_directory", func(t *testing.T) {
		path := "/test/dir"

		// Create directory
		err := fs.MkdirAll(path, 0755)
		require.NoError(t, err)

		// Check if directory exists
		assert.True(t, fs.Exists(path))
		assert.True(t, fs.IsDir(path))

		// Get directory info
		info, err := fs.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestMockCommandExecutor(t *testing.T) {
	executor := NewMockCommandExecutor()

	t.Run("execute_command_with_result", func(t *testing.T) {
		expectedOutput := []byte("test output")
		executor.SetResult("echo", expectedOutput, nil)

		output, err := executor.Execute(context.Background(), "echo", "hello")
		require.NoError(t, err)
		assert.Equal(t, expectedOutput, output)

		// Verify command was recorded
		assert.Len(t, executor.Commands, 1)
		assert.Equal(t, "echo", executor.Commands[0].Command)
		assert.Equal(t, []string{"hello"}, executor.Commands[0].Args)
	})

	t.Run("execute_with_workdir", func(t *testing.T) {
		output, err := executor.ExecuteWithWorkDir(context.Background(), "/tmp", "pwd")
		require.NoError(t, err)
		assert.NotNil(t, output)

		// Verify command was recorded with work directory
		assert.Len(t, executor.Commands, 2) // Previous test + this one
		lastCmd := executor.Commands[len(executor.Commands)-1]
		assert.Equal(t, "pwd", lastCmd.Command)
		assert.Equal(t, "/tmp", lastCmd.WorkDir)
	})
}

func TestEnhancedToolFactory_Integration(t *testing.T) {
	// Create mock dependencies
	mockFS := NewMockFileSystemService()
	mockExecutor := NewMockCommandExecutor()

	// Create some test files in mock file system
	mockFS.WriteFile("/workspace/test.go", []byte("package main"), 0644)
	mockFS.MkdirAll("/workspace/src", 0755)

	// Create enhanced tool factory with mocks
	factory := agent.NewEnhancedToolFactory("/workspace", mockFS, mockExecutor)

	t.Run("create_generation_registry_with_mocks", func(t *testing.T) {
		registry := factory.CreateGenerationRegistry()
		assert.NotNil(t, registry)

		// Get tools and verify they exist
		tools := registry.List()
		assert.Greater(t, len(tools), 0)

		// Find file write tool and verify it works with mock FS
		var writeToolFound bool
		for _, tool := range tools {
			if tool.Name() == "write_file" {
				writeToolFound = true
				break
			}
		}
		assert.True(t, writeToolFound, "write_file tool should be in generation registry")
	})

	t.Run("create_review_registry_with_mocks", func(t *testing.T) {
		registry := factory.CreateReviewRegistry()
		assert.NotNil(t, registry)

		tools := registry.List()
		assert.Greater(t, len(tools), 0)

		// Review registry should have more tools than generation registry
		genRegistry := factory.CreateGenerationRegistry()
		genTools := genRegistry.List()
		assert.GreaterOrEqual(t, len(tools), len(genTools))
	})
}
