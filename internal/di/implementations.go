package di

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// RealFileSystemService implements FileSystemService using os package
type RealFileSystemService struct{}

func (r *RealFileSystemService) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (r *RealFileSystemService) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (r *RealFileSystemService) ListDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (r *RealFileSystemService) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (r *RealFileSystemService) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (r *RealFileSystemService) Remove(path string) error {
	return os.Remove(path)
}

func (r *RealFileSystemService) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (r *RealFileSystemService) IsDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

// RealCommandExecutor implements CommandExecutor using exec package
type RealCommandExecutor struct{}

func (r *RealCommandExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	return cmd.Output()
}

func (r *RealCommandExecutor) ExecuteWithWorkDir(ctx context.Context, workDir, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	return cmd.Output()
}

func (r *RealCommandExecutor) ExecuteStream(ctx context.Context, command string, args ...string) (io.Reader, io.Reader, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	err = cmd.Start()
	return stdout, stderr, err
}

// RealSessionStore implements SessionStore using file system
type RealSessionStore struct {
	baseDir string
}

func NewRealSessionStore() *RealSessionStore {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return &RealSessionStore{
		baseDir: filepath.Join(homeDir, ".cge", "sessions"),
	}
}

func (r *RealSessionStore) Save(session *orchestrator.SessionState) error {
	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(r.baseDir, 0755); err != nil {
		return err
	}

	// For now, this is a placeholder implementation
	// You would implement actual session serialization here
	sessionPath := filepath.Join(r.baseDir, session.SessionID+".json")
	return os.WriteFile(sessionPath, []byte("{}"), 0644)
}

func (r *RealSessionStore) Load(sessionID string) (*orchestrator.SessionState, error) {
	// Placeholder implementation
	// You would implement actual session deserialization here
	sessionPath := filepath.Join(r.baseDir, sessionID+".json")
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, err
	}

	// Return a basic session state for now
	return &orchestrator.SessionState{
		SessionID: sessionID,
	}, nil
}

func (r *RealSessionStore) List() ([]string, error) {
	if _, err := os.Stat(r.baseDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

func (r *RealSessionStore) Delete(sessionID string) error {
	sessionPath := filepath.Join(r.baseDir, sessionID+".json")
	return os.Remove(sessionPath)
}

func (r *RealSessionStore) Exists(sessionID string) bool {
	sessionPath := filepath.Join(r.baseDir, sessionID+".json")
	_, err := os.Stat(sessionPath)
	return err == nil
}
