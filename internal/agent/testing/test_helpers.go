package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castrovroberto/CGE/internal/agent"
)

// TestWorkspace represents a temporary workspace for testing
type TestWorkspace struct {
	Dir     string
	T       *testing.T
	cleanup func()
}

// SetupTestWorkspace creates a temporary directory with sample files for testing
func SetupTestWorkspace(t *testing.T) *TestWorkspace {
	tempDir := t.TempDir()

	workspace := &TestWorkspace{
		Dir: tempDir,
		T:   t,
	}

	// Create some sample files and directories
	workspace.CreateFile("main.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`)

	workspace.CreateFile("README.md", `# Test Project

This is a test project for CGE testing.`)

	workspace.CreateDir("src")
	workspace.CreateFile("src/utils.go", `package src

func Add(a, b int) int {
	return a + b
}`)

	workspace.CreateFile("src/utils_test.go", `package src

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}
}`)

	workspace.CreateDir("docs")
	workspace.CreateFile("docs/api.md", `# API Documentation

## Endpoints

- GET /health
- POST /users`)

	return workspace
}

// CreateFile creates a file with the given content in the workspace
func (w *TestWorkspace) CreateFile(relativePath, content string) {
	fullPath := filepath.Join(w.Dir, relativePath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0750); err != nil {
		w.T.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
		w.T.Fatalf("Failed to create file %s: %v", fullPath, err)
	}
}

// CreateDir creates a directory in the workspace
func (w *TestWorkspace) CreateDir(relativePath string) {
	fullPath := filepath.Join(w.Dir, relativePath)
	if err := os.MkdirAll(fullPath, 0750); err != nil {
		w.T.Fatalf("Failed to create directory %s: %v", fullPath, err)
	}
}

// ReadFile reads a file from the workspace
func (w *TestWorkspace) ReadFile(relativePath string) string {
	fullPath := filepath.Join(w.Dir, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		w.T.Fatalf("Failed to read file %s: %v", fullPath, err)
	}
	return string(content)
}

// FileExists checks if a file exists in the workspace
func (w *TestWorkspace) FileExists(relativePath string) bool {
	fullPath := filepath.Join(w.Dir, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ListFiles returns all files in the workspace recursively
func (w *TestWorkspace) ListFiles() []string {
	var files []string
	err := filepath.Walk(w.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(w.Dir, path)
			files = append(files, relPath)
		}
		return nil
	})
	if err != nil {
		w.T.Fatalf("Failed to list files: %v", err)
	}
	return files
}

// AssertToolExecution executes a tool and validates the results
func AssertToolExecution(t *testing.T, tool agent.Tool, params json.RawMessage, expected *agent.ToolResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, params)

	// Check for execution errors
	if expected.Success && err != nil {
		t.Fatalf("Tool execution failed unexpectedly: %v", err)
	}
	if !expected.Success && err == nil && result.Success {
		t.Fatalf("Tool execution succeeded when it should have failed")
	}

	// Validate result
	if result == nil {
		t.Fatalf("Tool returned nil result")
	}

	if result.Success != expected.Success {
		t.Errorf("Expected success=%v, got success=%v", expected.Success, result.Success)
	}

	if expected.Error != "" && !strings.Contains(result.Error, expected.Error) {
		t.Errorf("Expected error to contain '%s', got '%s'", expected.Error, result.Error)
	}

	// Validate data if provided
	if expected.Data != nil {
		if result.Data == nil {
			t.Errorf("Expected data but got nil")
		} else {
			// Convert both to JSON for comparison
			expectedJSON, _ := json.Marshal(expected.Data)
			actualJSON, _ := json.Marshal(result.Data)
			if string(expectedJSON) != string(actualJSON) {
				t.Errorf("Expected data %s, got %s", string(expectedJSON), string(actualJSON))
			}
		}
	}
}

// AssertToolExecutionWithTimeout executes a tool with a custom timeout
func AssertToolExecutionWithTimeout(t *testing.T, tool agent.Tool, params json.RawMessage, timeout time.Duration, shouldTimeout bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	result, err := tool.Execute(ctx, params)
	duration := time.Since(start)

	if shouldTimeout {
		if err == nil || err != context.DeadlineExceeded {
			t.Errorf("Expected timeout error, got: %v", err)
		}
		if duration < timeout {
			t.Errorf("Expected execution to take at least %v, took %v", timeout, duration)
		}
	} else {
		if err == context.DeadlineExceeded {
			t.Errorf("Unexpected timeout after %v", duration)
		}
		if result == nil {
			t.Errorf("Expected result but got nil")
		}
	}
}

// CreateSampleProject generates a sample project structure for testing
func CreateSampleProject(t *testing.T, projectType string) *TestWorkspace {
	workspace := SetupTestWorkspace(t)

	switch projectType {
	case "go":
		workspace.CreateFile("go.mod", `module testproject

go 1.21`)
		workspace.CreateFile("main.go", `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}`)
		workspace.CreateFile("handler.go", `package main

import (
	"encoding/json"
	"net/http"
)

type User struct {
	ID   int    `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	users := []User{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}`)

	case "python":
		workspace.CreateFile("requirements.txt", `flask==2.3.0
pytest==7.4.0`)
		workspace.CreateFile("app.py", `from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def hello():
    return jsonify({"message": "Hello, World!"})

@app.route('/users')
def users():
    return jsonify([
        {"id": 1, "name": "Alice"},
        {"id": 2, "name": "Bob"}
    ])

if __name__ == '__main__':
    app.run(debug=True)`)
		workspace.CreateFile("test_app.py", `import pytest
from app import app

@pytest.fixture
def client():
    app.config['TESTING'] = True
    with app.test_client() as client:
        yield client

def test_hello(client):
    rv = client.get('/')
    assert b'Hello, World!' in rv.data`)

	case "node":
		workspace.CreateFile("package.json", `{
  "name": "test-project",
  "version": "1.0.0",
  "description": "Test Node.js project",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.0"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`)
		workspace.CreateFile("index.js", `const express = require('express');
const app = express();
const port = 3000;

app.get('/', (req, res) => {
  res.json({ message: 'Hello, World!' });
});

app.get('/users', (req, res) => {
  res.json([
    { id: 1, name: 'Alice' },
    { id: 2, name: 'Bob' }
  ]);
});

app.listen(port, () => {
  console.log(`+"`Server running on port ${port}`"+`);
});

module.exports = app;`)
		workspace.CreateFile("index.test.js", `const request = require('supertest');
const app = require('./index');

describe('GET /', () => {
  it('should return hello message', async () => {
    const res = await request(app).get('/');
    expect(res.statusCode).toBe(200);
    expect(res.body.message).toBe('Hello, World!');
  });
});`)

	default:
		t.Fatalf("Unknown project type: %s", projectType)
	}

	return workspace
}

// ValidateJSONSchema validates that a JSON string matches a basic schema
func ValidateJSONSchema(t *testing.T, jsonStr string, expectedFields []string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	for _, field := range expectedFields {
		if _, exists := data[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}
}

// AssertFileContent validates that a file contains expected content
func AssertFileContent(t *testing.T, workspace *TestWorkspace, filePath, expectedContent string) {
	if !workspace.FileExists(filePath) {
		t.Fatalf("File %s does not exist", filePath)
	}

	actualContent := workspace.ReadFile(filePath)
	if !strings.Contains(actualContent, expectedContent) {
		t.Errorf("File %s does not contain expected content.\nExpected: %s\nActual: %s",
			filePath, expectedContent, actualContent)
	}
}

// AssertFileNotExists validates that a file does not exist
func AssertFileNotExists(t *testing.T, workspace *TestWorkspace, filePath string) {
	if workspace.FileExists(filePath) {
		t.Errorf("File %s should not exist", filePath)
	}
}

// CreateTestParameters creates JSON parameters for tool testing
func CreateTestParameters(params map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(params)
	return json.RawMessage(data)
}

// MockLLMResponse represents a mock response from an LLM
type MockLLMResponse struct {
	IsTextResponse bool
	TextContent    string
	FunctionCall   *MockFunctionCall
}

// MockFunctionCall represents a mock function call from an LLM
type MockFunctionCall struct {
	Name      string
	Arguments json.RawMessage
	ID        string
}

// CreateMockLLMTextResponse creates a mock text response
func CreateMockLLMTextResponse(content string) MockLLMResponse {
	return MockLLMResponse{
		IsTextResponse: true,
		TextContent:    content,
	}
}

// CreateMockLLMFunctionCall creates a mock function call response
func CreateMockLLMFunctionCall(name string, args map[string]interface{}) MockLLMResponse {
	argsJSON, _ := json.Marshal(args)
	return MockLLMResponse{
		IsTextResponse: false,
		FunctionCall: &MockFunctionCall{
			Name:      name,
			Arguments: json.RawMessage(argsJSON),
			ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
		},
	}
}

// TestScenario represents a test scenario for integration testing
type TestScenario struct {
	Name           string
	Description    string
	InitialPrompt  string
	ExpectedTools  []string
	ExpectedFiles  []string
	ValidationFunc func(t *testing.T, workspace *TestWorkspace)
}

// RunTestScenario executes a test scenario
func RunTestScenario(t *testing.T, scenario TestScenario, workspace *TestWorkspace) {
	t.Run(scenario.Name, func(t *testing.T) {
		// This would be implemented with the actual agent runner
		// For now, it's a placeholder for the integration test structure

		if scenario.ValidationFunc != nil {
			scenario.ValidationFunc(t, workspace)
		}
	})
}
