package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeFileOps_ValidatePath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Get current working directory for relative path tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	safeOps := NewSafeFileOps(tmpDir, cwd)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid path within allowed root",
			path:    filepath.Join(tmpDir, "test.txt"),
			wantErr: false,
		},
		{
			name:    "valid relative path",
			path:    "test.txt",
			wantErr: false,
		},
		{
			name:    "path traversal attempt with ..",
			path:    filepath.Join(tmpDir, "../../../etc/passwd"),
			wantErr: true,
		},
		{
			name:    "path outside allowed root",
			path:    "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "path with .. in middle",
			path:    filepath.Join(tmpDir, "subdir", "..", "..", "outside.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := safeOps.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafeFileOps_SafeReadFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(testContent), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	safeOps := NewSafeFileOps(tmpDir)

	// Test reading valid file
	content, err := safeOps.SafeReadFile(testFile)
	if err != nil {
		t.Errorf("SafeReadFile() error = %v", err)
	}
	if string(content) != testContent {
		t.Errorf("SafeReadFile() content = %v, want %v", string(content), testContent)
	}

	// Test reading file outside allowed root
	_, err = safeOps.SafeReadFile("/etc/passwd")
	if err == nil {
		t.Error("SafeReadFile() should have failed for path outside allowed root")
	}
}

func TestSafeFileOps_SafeWriteFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	safeOps := NewSafeFileOps(tmpDir)

	// Test writing to valid path
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("Hello, World!")
	err = safeOps.SafeWriteFile(testFile, testContent, 0600)
	if err != nil {
		t.Errorf("SafeWriteFile() error = %v", err)
	}

	// Verify file was written using our safe operations
	content, err := safeOps.SafeReadFile(testFile)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Written content = %v, want %v", string(content), string(testContent))
	}

	// Test writing to path outside allowed root
	err = safeOps.SafeWriteFile("/tmp/outside.txt", testContent, 0600)
	if err == nil {
		t.Error("SafeWriteFile() should have failed for path outside allowed root")
	}
}
