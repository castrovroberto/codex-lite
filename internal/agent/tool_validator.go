package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ToolValidator provides common validation functions for tools
type ToolValidator struct {
	workspaceRoot string
}

// NewToolValidator creates a new tool validator
func NewToolValidator(workspaceRoot string) *ToolValidator {
	return &ToolValidator{
		workspaceRoot: workspaceRoot,
	}
}

// ValidateFilePath validates that a file path is safe and within the workspace
func (v *ToolValidator) ValidateFilePath(filePath string) error {
	if filePath == "" {
		return NewMissingParameterError("file_path")
	}

	// Check for dangerous path patterns
	if strings.Contains(filePath, "..") {
		return NewPathOutsideWorkspaceError(filePath)
	}

	// Check for absolute paths
	if filepath.IsAbs(filePath) {
		return NewPathOutsideWorkspaceError(filePath)
	}

	// Resolve full path
	fullPath := filepath.Join(v.workspaceRoot, filePath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure path is within workspace
	if !strings.HasPrefix(cleanPath, filepath.Clean(v.workspaceRoot)) {
		return NewPathOutsideWorkspaceError(filePath)
	}

	return nil
}

// ValidateFileExists validates that a file exists and is readable
func (v *ToolValidator) ValidateFileExists(filePath string) error {
	if err := v.ValidateFilePath(filePath); err != nil {
		return err
	}

	fullPath := filepath.Join(v.workspaceRoot, filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return NewFileNotFoundError(filePath)
	} else if err != nil {
		return NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Cannot access file: %s", filePath),
			"Check file permissions and ensure the file is accessible",
		).WithDetail("file_path", filePath)
	}

	return nil
}

// ValidateDirectoryPath validates that a directory path is safe and within the workspace
func (v *ToolValidator) ValidateDirectoryPath(dirPath string) error {
	if dirPath == "" {
		return NewMissingParameterError("directory_path")
	}

	// Check for dangerous path patterns
	if strings.Contains(dirPath, "..") {
		return NewPathOutsideWorkspaceError(dirPath)
	}

	// Check for absolute paths
	if filepath.IsAbs(dirPath) {
		return NewPathOutsideWorkspaceError(dirPath)
	}

	// Resolve full path
	fullPath := filepath.Join(v.workspaceRoot, dirPath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure path is within workspace
	if !strings.HasPrefix(cleanPath, filepath.Clean(v.workspaceRoot)) {
		return NewPathOutsideWorkspaceError(dirPath)
	}

	return nil
}

// ValidateDirectoryExists validates that a directory exists and is accessible
func (v *ToolValidator) ValidateDirectoryExists(dirPath string) error {
	if err := v.ValidateDirectoryPath(dirPath); err != nil {
		return err
	}

	fullPath := filepath.Join(v.workspaceRoot, dirPath)
	if stat, err := os.Stat(fullPath); os.IsNotExist(err) {
		return NewDirectoryNotFoundError(dirPath)
	} else if err != nil {
		return NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Cannot access directory: %s", dirPath),
			"Check directory permissions and ensure the directory is accessible",
		).WithDetail("directory_path", dirPath)
	} else if !stat.IsDir() {
		return NewParameterError("directory_path", fmt.Sprintf("'%s' is not a directory", dirPath))
	}

	return nil
}

// ValidateLineRange validates that start and end line numbers are valid
func (v *ToolValidator) ValidateLineRange(startLine, endLine int) error {
	if startLine < 1 {
		return NewParameterError("start_line", "must be a positive integer (lines start at 1)")
	}

	if endLine < 1 {
		return NewParameterError("end_line", "must be a positive integer (lines start at 1)")
	}

	if startLine > endLine {
		return NewInvalidLineRangeError(startLine, endLine)
	}

	return nil
}

// ValidateLineRangeForFile validates line range against an actual file
func (v *ToolValidator) ValidateLineRangeForFile(filePath string, startLine, endLine int) error {
	if err := v.ValidateLineRange(startLine, endLine); err != nil {
		return err
	}

	if err := v.ValidateFileExists(filePath); err != nil {
		return err
	}

	// Count lines in file
	fullPath := filepath.Join(v.workspaceRoot, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return NewStandardizedError(
			ErrorCodePermissionDenied,
			fmt.Sprintf("Cannot read file to validate line range: %s", filePath),
			"Ensure the file is readable and not corrupted",
		).WithDetail("file_path", filePath)
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if startLine > totalLines {
		return NewParameterError("start_line",
			fmt.Sprintf("line %d does not exist (file has %d lines)", startLine, totalLines))
	}

	if endLine > totalLines {
		return NewParameterError("end_line",
			fmt.Sprintf("line %d does not exist (file has %d lines)", endLine, totalLines))
	}

	return nil
}

// ValidateContentSize validates that content doesn't exceed size limits
func (v *ToolValidator) ValidateContentSize(content string, maxSizeBytes int) error {
	size := len([]byte(content))
	if size > maxSizeBytes {
		return NewContentTooLargeError(size, maxSizeBytes)
	}
	return nil
}

// ValidateJSONSchema validates parameters against a JSON schema with enhanced validation
func (v *ToolValidator) ValidateJSONSchema(params json.RawMessage, schema json.RawMessage) error {
	var paramMap map[string]interface{}
	if err := json.Unmarshal(params, &paramMap); err != nil {
		return NewStandardizedError(
			ErrorCodeInvalidParameters,
			"Invalid JSON parameters",
			"Ensure parameters are valid JSON format",
		).WithDetail("parse_error", err.Error())
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return NewStandardizedError(
			ErrorCodeInternalError,
			"Invalid JSON schema in tool definition",
			"This is a tool implementation error - contact support",
		).WithDetail("parse_error", err.Error())
	}

	// Check required fields
	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		if required, ok := schemaMap["required"].([]interface{}); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if _, exists := paramMap[reqStr]; !exists {
						return NewMissingParameterError(reqStr)
					}
				}
			}
		}

		// Enhanced type and constraint checking
		for key, value := range paramMap {
			if prop, exists := properties[key]; exists {
				if propMap, ok := prop.(map[string]interface{}); ok {
					if err := v.validateProperty(key, value, propMap); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateProperty performs enhanced property validation including constraints
func (v *ToolValidator) validateProperty(key string, value interface{}, propSchema map[string]interface{}) error {
	// Type validation
	if expectedType, ok := propSchema["type"].(string); ok {
		if err := v.validateType(key, value, expectedType); err != nil {
			return err
		}
	}

	// Pattern validation for strings
	if pattern, ok := propSchema["pattern"].(string); ok {
		if strValue, ok := value.(string); ok {
			matched, err := regexp.MatchString(pattern, strValue)
			if err != nil {
				return NewParameterError(key, fmt.Sprintf("invalid regex pattern in schema: %s", pattern))
			}
			if !matched {
				return NewParameterError(key, fmt.Sprintf("does not match required pattern: %s", pattern))
			}
		}
	}

	// Enum validation
	if enumValues, ok := propSchema["enum"].([]interface{}); ok {
		found := false
		for _, enumValue := range enumValues {
			if value == enumValue {
				found = true
				break
			}
		}
		if !found {
			return NewParameterError(key, fmt.Sprintf("must be one of: %v", enumValues))
		}
	}

	// Numeric constraints
	if numValue, ok := value.(float64); ok {
		// Minimum value
		if minimum, ok := propSchema["minimum"].(float64); ok {
			if numValue < minimum {
				return NewParameterError(key, fmt.Sprintf("must be >= %g", minimum))
			}
		}

		// Maximum value
		if maximum, ok := propSchema["maximum"].(float64); ok {
			if numValue > maximum {
				return NewParameterError(key, fmt.Sprintf("must be <= %g", maximum))
			}
		}
	}

	// String length constraints
	if strValue, ok := value.(string); ok {
		// Minimum length
		if minLength, ok := propSchema["minLength"].(float64); ok {
			if len(strValue) < int(minLength) {
				return NewParameterError(key, fmt.Sprintf("must be at least %d characters", int(minLength)))
			}
		}

		// Maximum length
		if maxLength, ok := propSchema["maxLength"].(float64); ok {
			if len(strValue) > int(maxLength) {
				return NewParameterError(key, fmt.Sprintf("must be at most %d characters", int(maxLength)))
			}
		}
	}

	// Array constraints
	if arrValue, ok := value.([]interface{}); ok {
		// Minimum items
		if minItems, ok := propSchema["minItems"].(float64); ok {
			if len(arrValue) < int(minItems) {
				return NewParameterError(key, fmt.Sprintf("must have at least %d items", int(minItems)))
			}
		}

		// Maximum items
		if maxItems, ok := propSchema["maxItems"].(float64); ok {
			if len(arrValue) > int(maxItems) {
				return NewParameterError(key, fmt.Sprintf("must have at most %d items", int(maxItems)))
			}
		}
	}

	return nil
}

// validateType performs enhanced type validation
func (v *ToolValidator) validateType(key string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return NewParameterError(key, "must be a string")
		}
	case "integer":
		// Accept both int and float64 (JSON numbers), but check if it's a whole number
		switch val := value.(type) {
		case float64:
			if val != float64(int64(val)) {
				return NewParameterError(key, "must be an integer (whole number)")
			}
		case int, int32, int64:
			// These are fine
		default:
			return NewParameterError(key, "must be an integer")
		}
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// All numeric types are acceptable
		default:
			return NewParameterError(key, "must be a number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return NewParameterError(key, "must be a boolean (true/false)")
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return NewParameterError(key, "must be an array")
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return NewParameterError(key, "must be an object")
		}
	}
	return nil
}

// ValidateCommitMessage validates a Git commit message with enhanced rules
func (v *ToolValidator) ValidateCommitMessage(message string) error {
	message = strings.TrimSpace(message)

	if message == "" {
		return NewMissingParameterError("commit_message")
	}

	if len(message) > 500 {
		return NewParameterError("commit_message", "must be 500 characters or less")
	}

	if len(message) < 10 {
		return NewParameterError("commit_message", "must be at least 10 characters long")
	}

	// Check for common problematic patterns
	if strings.Contains(message, "\x00") {
		return NewParameterError("commit_message", "cannot contain null bytes")
	}

	// Check for conventional commit format encouragement
	lines := strings.Split(message, "\n")
	firstLine := lines[0]

	if len(firstLine) > 72 {
		return NewParameterError("commit_message", "first line should be 72 characters or less for better git log display")
	}

	return nil
}

// ValidateTestPattern validates a test pattern for Go tests
func (v *ToolValidator) ValidateTestPattern(pattern string) error {
	if pattern == "" {
		return nil // Empty pattern is valid (runs all tests)
	}

	// Basic regex validation
	_, err := regexp.Compile(pattern)
	if err != nil {
		return NewStandardizedError(
			ErrorCodeInvalidTestPattern,
			fmt.Sprintf("Invalid test pattern: %s", err.Error()),
			"Use a valid regular expression for test pattern matching",
		).WithDetail("pattern", pattern).WithDetail("regex_error", err.Error())
	}

	return nil
}

// ValidateTimeout validates a timeout value with reasonable limits
func (v *ToolValidator) ValidateTimeout(timeout int) error {
	if timeout < 0 {
		return NewParameterError("timeout", "cannot be negative")
	}

	if timeout > 3600 { // 1 hour max
		return NewParameterError("timeout", "cannot exceed 3600 seconds (1 hour)")
	}

	return nil
}

// ValidateFileExtension validates that a file has an allowed extension
func (v *ToolValidator) ValidateFileExtension(filePath string, allowedExtensions []string) error {
	if len(allowedExtensions) == 0 {
		return nil // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	for _, allowed := range allowedExtensions {
		if strings.ToLower(allowed) == ext {
			return nil
		}
	}

	return NewParameterError("file_path",
		fmt.Sprintf("file extension '%s' not allowed. Allowed extensions: %v", ext, allowedExtensions))
}

// ValidatePortNumber validates a network port number
func (v *ToolValidator) ValidatePortNumber(port interface{}) error {
	var portNum int

	switch p := port.(type) {
	case int:
		portNum = p
	case float64:
		portNum = int(p)
	case string:
		var err error
		portNum, err = strconv.Atoi(p)
		if err != nil {
			return NewParameterError("port", "must be a valid integer")
		}
	default:
		return NewParameterError("port", "must be a number")
	}

	if portNum < 1 || portNum > 65535 {
		return NewParameterError("port", "must be between 1 and 65535")
	}

	return nil
}

// ValidateURL validates a basic URL format
func (v *ToolValidator) ValidateURL(url string) error {
	if url == "" {
		return NewMissingParameterError("url")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return NewParameterError("url", "must start with http:// or https://")
	}

	return nil
}

// SanitizeFilePath sanitizes a file path by removing dangerous characters
func (v *ToolValidator) SanitizeFilePath(filePath string) string {
	// Remove null bytes and other control characters
	filePath = strings.ReplaceAll(filePath, "\x00", "")

	// Remove leading/trailing whitespace
	filePath = strings.TrimSpace(filePath)

	// Normalize path separators
	filePath = filepath.Clean(filePath)

	return filePath
}

// IsWithinWorkspace checks if a path is within the workspace
func (v *ToolValidator) IsWithinWorkspace(path string) bool {
	fullPath := filepath.Join(v.workspaceRoot, path)
	cleanPath := filepath.Clean(fullPath)
	cleanWorkspace := filepath.Clean(v.workspaceRoot)
	return strings.HasPrefix(cleanPath, cleanWorkspace)
}

// GetSafePath returns a safe absolute path within the workspace
func (v *ToolValidator) GetSafePath(relativePath string) (string, error) {
	if err := v.ValidateFilePath(relativePath); err != nil {
		return "", err
	}
	return filepath.Join(v.workspaceRoot, relativePath), nil
}
