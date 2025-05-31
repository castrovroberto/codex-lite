package testing

import (
	"encoding/json"
)

// TestFixtures contains common test data and configurations
type TestFixtures struct {
	SampleGoCode     string
	SamplePythonCode string
	SampleJSCode     string
	SampleTestOutput string
	SampleLintOutput string
	SampleGitStatus  string
}

// NewTestFixtures creates a new set of test fixtures
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{
		SampleGoCode: `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", handleRoot)
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}`,

		SamplePythonCode: `from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def hello():
    return jsonify({"message": "Hello, World!"})

@app.route('/users/<int:user_id>')
def get_user(user_id):
    # TODO: Implement user lookup
    return jsonify({"id": user_id, "name": "User"})

if __name__ == '__main__':
    app.run(debug=True)`,

		SampleJSCode: `const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

app.use(express.json());

app.get('/', (req, res) => {
  res.json({ message: 'Hello, World!' });
});

app.get('/users/:id', (req, res) => {
  const userId = parseInt(req.params.id);
  // TODO: Implement user lookup
  res.json({ id: userId, name: 'User' });
});

app.listen(port, () => {
  console.log(` + "`Server running on port ${port}`" + `);
});

module.exports = app;`,

		SampleTestOutput: `=== RUN TestMain
=== RUN TestMain/TestHandleRoot
--- PASS: TestMain/TestHandleRoot (0.00s)
=== RUN TestMain/TestGetUser
--- FAIL: TestMain/TestGetUser (0.00s)
    main_test.go:25: Expected status 200, got 404
=== RUN TestUtils
=== RUN TestUtils/TestAdd
--- PASS: TestUtils/TestAdd (0.00s)
=== RUN TestUtils/TestSubtract
--- FAIL: TestUtils/TestSubtract (0.00s)
    utils_test.go:15: Expected 5, got 3
FAIL
coverage: 75.0% of statements
exit status 1`,

		SampleLintOutput: `main.go:15:1: exported function handleRoot should have comment or be unexported (missing-doc)
main.go:23:2: should use fmt.Fprint instead of fmt.Fprintf (simplify)
utils.go:8:1: function Add is missing documentation (missing-doc)
utils.go:12:1: function Subtract is missing documentation (missing-doc)
handler.go:45:1: cyclomatic complexity 15 of function processRequest is high (> 10) (gocyclo)`,

		SampleGitStatus: `On branch main
Your branch is up to date with 'origin/main'.

Changes to be committed:
  (use "git reset HEAD <file>..." to unstage)
	modified:   main.go
	new file:   handler.go

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)
	modified:   utils.go
	modified:   README.md

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	config.json
	temp.log`,
	}
}

// ToolTestCase represents a test case for tool testing
type ToolTestCase struct {
	Name           string
	Description    string
	Parameters     map[string]interface{}
	ExpectedResult *ToolTestResult
	ShouldFail     bool
	ErrorContains  string
}

// ToolTestResult represents expected results from tool execution
type ToolTestResult struct {
	Success bool
	Data    map[string]interface{}
	Error   string
}

// GetReadFileTestCases returns test cases for read_file tool
func GetReadFileTestCases() []ToolTestCase {
	return []ToolTestCase{
		{
			Name:        "read_existing_file",
			Description: "Read an existing file successfully",
			Parameters: map[string]interface{}{
				"target_file": "main.go",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"content": "package main", // Should contain this
				},
			},
		},
		{
			Name:        "read_nonexistent_file",
			Description: "Attempt to read a file that doesn't exist",
			Parameters: map[string]interface{}{
				"target_file": "nonexistent.go",
			},
			ShouldFail:    true,
			ErrorContains: "no such file",
		},
		{
			Name:        "read_file_with_range",
			Description: "Read specific lines from a file",
			Parameters: map[string]interface{}{
				"target_file":                    "main.go",
				"start_line_one_indexed":         1,
				"end_line_one_indexed_inclusive": 5,
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"content": "package main",
				},
			},
		},
		{
			Name:        "read_file_invalid_range",
			Description: "Attempt to read with invalid line range",
			Parameters: map[string]interface{}{
				"target_file":                    "main.go",
				"start_line_one_indexed":         10,
				"end_line_one_indexed_inclusive": 5,
			},
			ShouldFail:    true,
			ErrorContains: "invalid range",
		},
	}
}

// GetWriteFileTestCases returns test cases for write_file tool
func GetWriteFileTestCases() []ToolTestCase {
	return []ToolTestCase{
		{
			Name:        "write_new_file",
			Description: "Create a new file with content",
			Parameters: map[string]interface{}{
				"file_path": "new_file.go",
				"content":   "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"bytes_written": 45,
				},
			},
		},
		{
			Name:        "overwrite_existing_file",
			Description: "Overwrite an existing file",
			Parameters: map[string]interface{}{
				"file_path": "main.go",
				"content":   "package main\n\n// Updated content",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
			},
		},
		{
			Name:        "write_file_in_subdirectory",
			Description: "Create a file in a subdirectory (should create dirs)",
			Parameters: map[string]interface{}{
				"file_path": "src/handlers/user.go",
				"content":   "package handlers",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
			},
		},
		{
			Name:        "write_file_invalid_path",
			Description: "Attempt to write to an invalid path",
			Parameters: map[string]interface{}{
				"file_path": "/root/restricted.go",
				"content":   "package main",
			},
			ShouldFail:    true,
			ErrorContains: "permission denied",
		},
	}
}

// GetShellRunTestCases returns test cases for shell command tool
func GetShellRunTestCases() []ToolTestCase {
	return []ToolTestCase{
		{
			Name:        "run_simple_command",
			Description: "Run a simple shell command",
			Parameters: map[string]interface{}{
				"command": "echo 'Hello, World!'",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"stdout":    "Hello, World!",
					"exit_code": 0,
				},
			},
		},
		{
			Name:        "run_command_with_error",
			Description: "Run a command that fails",
			Parameters: map[string]interface{}{
				"command": "ls /nonexistent/directory",
			},
			ExpectedResult: &ToolTestResult{
				Success: true, // Tool succeeds, but command fails
				Data: map[string]interface{}{
					"exit_code": 1,
				},
			},
		},
		{
			Name:        "run_go_test",
			Description: "Run Go tests",
			Parameters: map[string]interface{}{
				"command": "go test ./...",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
			},
		},
		{
			Name:        "run_command_with_timeout",
			Description: "Run a command with timeout",
			Parameters: map[string]interface{}{
				"command":         "sleep 10",
				"timeout_seconds": 2,
			},
			ShouldFail:    true,
			ErrorContains: "timeout",
		},
	}
}

// GetGitCommitTestCases returns test cases for git commit tool
func GetGitCommitTestCases() []ToolTestCase {
	return []ToolTestCase{
		{
			Name:        "commit_staged_files",
			Description: "Commit files that are already staged",
			Parameters: map[string]interface{}{
				"commit_message": "Add new feature",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"commit_hash": "abc123",
				},
			},
		},
		{
			Name:        "commit_specific_files",
			Description: "Stage and commit specific files",
			Parameters: map[string]interface{}{
				"commit_message": "Update handlers",
				"files_to_stage": []string{"handler.go", "utils.go"},
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
			},
		},
		{
			Name:        "commit_with_empty_message",
			Description: "Attempt to commit with empty message",
			Parameters: map[string]interface{}{
				"commit_message": "",
			},
			ShouldFail:    true,
			ErrorContains: "empty commit message",
		},
		{
			Name:        "commit_no_changes",
			Description: "Attempt to commit when there are no changes",
			Parameters: map[string]interface{}{
				"commit_message": "No changes to commit",
			},
			ShouldFail:    true,
			ErrorContains: "nothing to commit",
		},
	}
}

// GetTestRunnerTestCases returns test cases for test runner tool
func GetTestRunnerTestCases() []ToolTestCase {
	return []ToolTestCase{
		{
			Name:        "run_passing_tests",
			Description: "Run tests that all pass",
			Parameters: map[string]interface{}{
				"test_command": "go test ./...",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"passed":       true,
					"test_count":   5,
					"failed_count": 0,
				},
			},
		},
		{
			Name:        "run_failing_tests",
			Description: "Run tests with some failures",
			Parameters: map[string]interface{}{
				"test_command": "go test ./...",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"passed":       false,
					"test_count":   5,
					"failed_count": 2,
				},
			},
		},
		{
			Name:        "run_tests_with_coverage",
			Description: "Run tests with coverage reporting",
			Parameters: map[string]interface{}{
				"test_command":     "go test -cover ./...",
				"include_coverage": true,
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"coverage_percentage": 75.0,
				},
			},
		},
	}
}

// GetParseTestResultsTestCases returns test cases for test result parsing
func GetParseTestResultsTestCases() []ToolTestCase {
	fixtures := NewTestFixtures()

	return []ToolTestCase{
		{
			Name:        "parse_go_test_output",
			Description: "Parse Go test output with failures",
			Parameters: map[string]interface{}{
				"test_output": fixtures.SampleTestOutput,
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"total_tests":  4,
					"passed_tests": 2,
					"failed_tests": 2,
					"coverage":     75.0,
				},
			},
		},
		{
			Name:        "parse_empty_output",
			Description: "Parse empty test output",
			Parameters: map[string]interface{}{
				"test_output": "",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"total_tests": 0,
				},
			},
		},
	}
}

// GetParseLintResultsTestCases returns test cases for lint result parsing
func GetParseLintResultsTestCases() []ToolTestCase {
	fixtures := NewTestFixtures()

	return []ToolTestCase{
		{
			Name:        "parse_golangci_lint_output",
			Description: "Parse golangci-lint output with issues",
			Parameters: map[string]interface{}{
				"lint_output": fixtures.SampleLintOutput,
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"total_issues":      5,
					"files_with_issues": []string{"main.go", "utils.go", "handler.go"},
				},
			},
		},
		{
			Name:        "parse_clean_lint_output",
			Description: "Parse lint output with no issues",
			Parameters: map[string]interface{}{
				"lint_output": "",
			},
			ExpectedResult: &ToolTestResult{
				Success: true,
				Data: map[string]interface{}{
					"total_issues": 0,
				},
			},
		},
	}
}

// CreateParametersJSON converts a map to JSON parameters
func CreateParametersJSON(params map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(params)
	return json.RawMessage(data)
}

// SampleProjects contains sample project structures for testing
var SampleProjects = map[string]map[string]string{
	"simple_go": {
		"go.mod": `module testproject

go 1.21`,
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
		"utils.go": `package main

func Add(a, b int) int {
	return a + b
}`,
		"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Expected 5, got %d", result)
	}
}`,
	},
	"web_api": {
		"go.mod": `module webapi

go 1.21

require github.com/gorilla/mux v1.8.0`,
		"main.go": `package main

import (
	"log"
	"net/http"
	
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/users/{id}", UserHandler)
	
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}`,
		"handlers.go": `package main

import (
	"encoding/json"
	"net/http"
	
	"github.com/gorilla/mux"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"message": "Hello, World!"})
}

func UserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	
	json.NewEncoder(w).Encode(map[string]string{
		"id": userID,
		"name": "User " + userID,
	})
}`,
	},
}
