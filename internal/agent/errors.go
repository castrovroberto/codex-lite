package agent

import (
	"fmt"
	"strings"
)

// ToolErrorCode represents standardized error codes for tools
type ToolErrorCode string

const (
	// Parameter validation errors
	ErrorCodeInvalidParameters    ToolErrorCode = "INVALID_PARAMETERS"
	ErrorCodeMissingParameter     ToolErrorCode = "MISSING_PARAMETER"
	ErrorCodeInvalidPathFormat    ToolErrorCode = "INVALID_PATH_FORMAT"
	ErrorCodePathOutsideWorkspace ToolErrorCode = "PATH_OUTSIDE_WORKSPACE"

	// File system errors
	ErrorCodeFileNotFound      ToolErrorCode = "FILE_NOT_FOUND"
	ErrorCodeDirectoryNotFound ToolErrorCode = "DIRECTORY_NOT_FOUND"
	ErrorCodeFileAlreadyExists ToolErrorCode = "FILE_ALREADY_EXISTS"
	ErrorCodePermissionDenied  ToolErrorCode = "PERMISSION_DENIED"
	ErrorCodeInsufficientSpace ToolErrorCode = "INSUFFICIENT_SPACE"

	// Content validation errors
	ErrorCodeInvalidFileFormat ToolErrorCode = "INVALID_FILE_FORMAT"
	ErrorCodeContentTooLarge   ToolErrorCode = "CONTENT_TOO_LARGE"
	ErrorCodeInvalidLineRange  ToolErrorCode = "INVALID_LINE_RANGE"
	ErrorCodeInvalidEncoding   ToolErrorCode = "INVALID_ENCODING"

	// Git operation errors
	ErrorCodeGitNotRepository     ToolErrorCode = "GIT_NOT_REPOSITORY"
	ErrorCodeGitNothingToCommit   ToolErrorCode = "GIT_NOTHING_TO_COMMIT"
	ErrorCodeGitConflict          ToolErrorCode = "GIT_CONFLICT"
	ErrorCodeInvalidCommitMessage ToolErrorCode = "INVALID_COMMIT_MESSAGE"

	// Test/lint operation errors
	ErrorCodeTestFailure        ToolErrorCode = "TEST_FAILURE"
	ErrorCodeLintErrors         ToolErrorCode = "LINT_ERRORS"
	ErrorCodeCompilationFailure ToolErrorCode = "COMPILATION_FAILURE"
	ErrorCodeInvalidTestPattern ToolErrorCode = "INVALID_TEST_PATTERN"

	// Shell/command execution errors
	ErrorCodeCommandNotFound    ToolErrorCode = "COMMAND_NOT_FOUND"
	ErrorCodeCommandTimeout     ToolErrorCode = "COMMAND_TIMEOUT"
	ErrorCodeCommandFailed      ToolErrorCode = "COMMAND_FAILED"
	ErrorCodeInvalidCommandArgs ToolErrorCode = "INVALID_COMMAND_ARGS"

	// General system errors
	ErrorCodeInternalError        ToolErrorCode = "INTERNAL_ERROR"
	ErrorCodeTimeout              ToolErrorCode = "TIMEOUT"
	ErrorCodeResourceLimit        ToolErrorCode = "RESOURCE_LIMIT"
	ErrorCodeUnsupportedOperation ToolErrorCode = "UNSUPPORTED_OPERATION"
)

// StandardizedToolError represents a rich error structure for tools
type StandardizedToolError struct {
	Code             ToolErrorCode          `json:"code"`
	Message          string                 `json:"message"`
	SuggestionForLLM string                 `json:"suggestion_for_llm"`
	Details          map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *StandardizedToolError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewStandardizedError creates a new standardized tool error
func NewStandardizedError(code ToolErrorCode, message string, suggestion string) *StandardizedToolError {
	return &StandardizedToolError{
		Code:             code,
		Message:          message,
		SuggestionForLLM: suggestion,
		Details:          make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error
func (e *StandardizedToolError) WithDetail(key string, value interface{}) *StandardizedToolError {
	e.Details[key] = value
	return e
}

// FormatForLLM formats the error in a way that's helpful for the LLM
func (e *StandardizedToolError) FormatForLLM() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("ERROR: %s", e.Message))

	if e.SuggestionForLLM != "" {
		parts = append(parts, fmt.Sprintf("SUGGESTION: %s", e.SuggestionForLLM))
	}

	if len(e.Details) > 0 {
		details := make([]string, 0, len(e.Details))
		for key, value := range e.Details {
			details = append(details, fmt.Sprintf("%s: %v", key, value))
		}
		parts = append(parts, fmt.Sprintf("DETAILS: %s", strings.Join(details, ", ")))
	}

	return strings.Join(parts, "\n")
}

// Common error factory functions for convenience

// NewParameterError creates a parameter validation error
func NewParameterError(paramName string, reason string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeInvalidParameters,
		fmt.Sprintf("Invalid parameter '%s': %s", paramName, reason),
		fmt.Sprintf("Please check the '%s' parameter and ensure it meets the requirements: %s", paramName, reason),
	).WithDetail("parameter", paramName)
}

// NewMissingParameterError creates a missing parameter error
func NewMissingParameterError(paramName string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeMissingParameter,
		fmt.Sprintf("Required parameter '%s' is missing", paramName),
		fmt.Sprintf("Please provide the required parameter '%s' in your tool call", paramName),
	).WithDetail("parameter", paramName)
}

// NewFileNotFoundError creates a file not found error
func NewFileNotFoundError(filePath string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeFileNotFound,
		fmt.Sprintf("File not found: %s", filePath),
		fmt.Sprintf("The file '%s' does not exist. Use the 'list_directory' tool to verify the correct path, or check if the file needs to be created first", filePath),
	).WithDetail("file_path", filePath)
}

// NewDirectoryNotFoundError creates a directory not found error
func NewDirectoryNotFoundError(dirPath string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeDirectoryNotFound,
		fmt.Sprintf("Directory not found: %s", dirPath),
		fmt.Sprintf("The directory '%s' does not exist. Use the 'list_directory' tool to verify the correct path, or ensure parent directories are created first", dirPath),
	).WithDetail("directory_path", dirPath)
}

// NewPathOutsideWorkspaceError creates a path outside workspace error
func NewPathOutsideWorkspaceError(path string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodePathOutsideWorkspace,
		fmt.Sprintf("Path is outside workspace: %s", path),
		"Ensure all file paths are relative to the workspace root and do not use '..' or absolute paths that escape the workspace boundary",
	).WithDetail("invalid_path", path)
}

// NewInvalidLineRangeError creates an invalid line range error
func NewInvalidLineRangeError(startLine, endLine int) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeInvalidLineRange,
		fmt.Sprintf("Invalid line range: start_line=%d, end_line=%d", startLine, endLine),
		"Ensure start_line is less than or equal to end_line, and both are positive integers within the file's line count",
	).WithDetail("start_line", startLine).WithDetail("end_line", endLine)
}

// NewContentTooLargeError creates a content too large error
func NewContentTooLargeError(size, maxSize int) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeContentTooLarge,
		fmt.Sprintf("Content too large: %d bytes (max: %d)", size, maxSize),
		fmt.Sprintf("Reduce the content size to under %d bytes, or consider breaking it into smaller chunks", maxSize),
	).WithDetail("size", size).WithDetail("max_size", maxSize)
}

// NewGitNotRepositoryError creates a git not repository error
func NewGitNotRepositoryError(path string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeGitNotRepository,
		fmt.Sprintf("Not a git repository: %s", path),
		"Ensure you are working within a git repository. Initialize with 'git init' if this is a new project",
	).WithDetail("path", path)
}

// NewTestFailureError creates a test failure error
func NewTestFailureError(failureCount int, details string) *StandardizedToolError {
	return NewStandardizedError(
		ErrorCodeTestFailure,
		fmt.Sprintf("Tests failed: %d failures", failureCount),
		"Review the test failures and fix the underlying issues. Use the parse_test_results tool to get detailed failure information",
	).WithDetail("failure_count", failureCount).WithDetail("details", details)
}

// GetErrorCodeSuggestions returns general suggestions for each error code
func GetErrorCodeSuggestions() map[ToolErrorCode]string {
	return map[ToolErrorCode]string{
		ErrorCodeInvalidParameters:    "Carefully review the tool's parameter schema and ensure all parameters match the expected types and formats",
		ErrorCodeMissingParameter:     "Check the tool's required parameters and provide all mandatory fields",
		ErrorCodeFileNotFound:         "Use 'list_directory' to verify file existence and correct paths",
		ErrorCodeDirectoryNotFound:    "Use 'list_directory' to verify directory structure and ensure parent directories exist",
		ErrorCodePathOutsideWorkspace: "Use relative paths within the workspace and avoid '..' or absolute paths",
		ErrorCodeInvalidLineRange:     "Ensure start_line <= end_line and both are within the file's line count",
		ErrorCodeContentTooLarge:      "Break large content into smaller chunks or use streaming operations",
		ErrorCodeGitNotRepository:     "Ensure you're working within a git repository or initialize one if needed",
		ErrorCodeTestFailure:          "Review test failures and fix underlying issues before proceeding",
		ErrorCodeCommandFailed:        "Check command syntax, arguments, and ensure required dependencies are available",
		ErrorCodeTimeout:              "Reduce operation scope or increase timeout limits for complex operations",
	}
}
