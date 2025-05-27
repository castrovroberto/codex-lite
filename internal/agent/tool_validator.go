package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
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
		return fmt.Errorf("file path cannot be empty")
	}

	// Check for dangerous path patterns
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("file path cannot contain '..' (path traversal)")
	}

	// Resolve full path
	fullPath := filepath.Join(v.workspaceRoot, filePath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure path is within workspace
	if !strings.HasPrefix(cleanPath, filepath.Clean(v.workspaceRoot)) {
		return fmt.Errorf("file path is outside workspace root")
	}

	return nil
}

// ValidateDirectoryPath validates that a directory path is safe and within the workspace
func (v *ToolValidator) ValidateDirectoryPath(dirPath string) error {
	if dirPath == "" {
		return fmt.Errorf("directory path cannot be empty")
	}

	// Check for dangerous path patterns
	if strings.Contains(dirPath, "..") {
		return fmt.Errorf("directory path cannot contain '..' (path traversal)")
	}

	// Resolve full path
	fullPath := filepath.Join(v.workspaceRoot, dirPath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure path is within workspace
	if !strings.HasPrefix(cleanPath, filepath.Clean(v.workspaceRoot)) {
		return fmt.Errorf("directory path is outside workspace root")
	}

	return nil
}

// ValidateJSONSchema validates parameters against a JSON schema (basic validation)
func (v *ToolValidator) ValidateJSONSchema(params json.RawMessage, schema json.RawMessage) error {
	// This is a basic implementation - in a production system you'd use a proper JSON schema validator
	var paramMap map[string]interface{}
	if err := json.Unmarshal(params, &paramMap); err != nil {
		return fmt.Errorf("invalid JSON parameters: %w", err)
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	// Check required fields
	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		if required, ok := schemaMap["required"].([]interface{}); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if _, exists := paramMap[reqStr]; !exists {
						return fmt.Errorf("required parameter '%s' is missing", reqStr)
					}
				}
			}
		}

		// Basic type checking
		for key, value := range paramMap {
			if prop, exists := properties[key]; exists {
				if propMap, ok := prop.(map[string]interface{}); ok {
					if expectedType, ok := propMap["type"].(string); ok {
						if err := v.validateType(key, value, expectedType); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// validateType performs basic type validation
func (v *ToolValidator) validateType(key string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter '%s' must be a string", key)
		}
	case "integer":
		switch value.(type) {
		case int, int32, int64, float64:
			// JSON numbers can be parsed as float64, so we accept that too
		default:
			return fmt.Errorf("parameter '%s' must be an integer", key)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be a boolean", key)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an array", key)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an object", key)
		}
	}
	return nil
}

// ValidateCommitMessage validates a Git commit message
func (v *ToolValidator) ValidateCommitMessage(message string) error {
	message = strings.TrimSpace(message)

	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	if len(message) > 500 {
		return fmt.Errorf("commit message is too long (max 500 characters)")
	}

	// Check for common problematic patterns
	if strings.Contains(message, "\x00") {
		return fmt.Errorf("commit message cannot contain null bytes")
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
		return fmt.Errorf("invalid test pattern: %w", err)
	}

	return nil
}

// ValidateTimeout validates a timeout value
func (v *ToolValidator) ValidateTimeout(timeout int) error {
	if timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	if timeout > 3600 { // 1 hour max
		return fmt.Errorf("timeout cannot exceed 3600 seconds (1 hour)")
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
	return strings.HasPrefix(cleanPath, filepath.Clean(v.workspaceRoot))
}

// GetSafePath returns a safe path within the workspace
func (v *ToolValidator) GetSafePath(relativePath string) (string, error) {
	if err := v.ValidateFilePath(relativePath); err != nil {
		return "", err
	}

	return filepath.Join(v.workspaceRoot, relativePath), nil
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

	return fmt.Errorf("file extension '%s' is not allowed. Allowed extensions: %v", ext, allowedExtensions)
}

// ValidateContentSize validates that content is not too large
func (v *ToolValidator) ValidateContentSize(content string, maxSizeBytes int) error {
	if maxSizeBytes <= 0 {
		return nil // No size limit
	}

	if len(content) > maxSizeBytes {
		return fmt.Errorf("content size (%d bytes) exceeds maximum allowed size (%d bytes)",
			len(content), maxSizeBytes)
	}

	return nil
}
