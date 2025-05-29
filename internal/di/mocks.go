package di

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// MockFileSystemService provides a mock implementation for testing
type MockFileSystemService struct {
	Files map[string][]byte
	Dirs  map[string]bool
}

func NewMockFileSystemService() *MockFileSystemService {
	return &MockFileSystemService{
		Files: make(map[string][]byte),
		Dirs:  make(map[string]bool),
	}
}

func (m *MockFileSystemService) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.Files[path] = make([]byte, len(data))
	copy(m.Files[path], data)
	return nil
}

func (m *MockFileSystemService) ReadFile(path string) ([]byte, error) {
	if data, exists := m.Files[path]; exists {
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystemService) ListDir(path string) ([]os.DirEntry, error) {
	// Simple implementation - just return empty list
	return []os.DirEntry{}, nil
}

func (m *MockFileSystemService) Stat(path string) (os.FileInfo, error) {
	if data, exists := m.Files[path]; exists {
		return &MockFileInfo{
			name: path,
			size: int64(len(data)),
			mode: 0644,
		}, nil
	}
	if m.Dirs[path] {
		return &MockFileInfo{
			name: path,
			size: 0,
			mode: os.ModeDir | 0755,
		}, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFileSystemService) MkdirAll(path string, perm os.FileMode) error {
	m.Dirs[path] = true
	return nil
}

func (m *MockFileSystemService) Remove(path string) error {
	delete(m.Files, path)
	delete(m.Dirs, path)
	return nil
}

func (m *MockFileSystemService) Exists(path string) bool {
	_, fileExists := m.Files[path]
	dirExists := m.Dirs[path]
	return fileExists || dirExists
}

func (m *MockFileSystemService) IsDir(path string) bool {
	return m.Dirs[path]
}

// MockFileInfo implements os.FileInfo for testing
type MockFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return m.size }
func (m *MockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *MockFileInfo) ModTime() time.Time { return time.Now() }
func (m *MockFileInfo) IsDir() bool        { return m.mode.IsDir() }
func (m *MockFileInfo) Sys() interface{}   { return nil }

// MockCommandExecutor provides a mock implementation for testing
type MockCommandExecutor struct {
	Commands []MockCommand
	Results  map[string]MockCommandResult
}

type MockCommand struct {
	Command string
	Args    []string
	WorkDir string
}

type MockCommandResult struct {
	Output []byte
	Error  error
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Commands: make([]MockCommand, 0),
		Results:  make(map[string]MockCommandResult),
	}
}

func (m *MockCommandExecutor) SetResult(command string, output []byte, err error) {
	m.Results[command] = MockCommandResult{
		Output: output,
		Error:  err,
	}
}

func (m *MockCommandExecutor) Execute(ctx context.Context, command string, args ...string) ([]byte, error) {
	m.Commands = append(m.Commands, MockCommand{
		Command: command,
		Args:    args,
	})

	cmdKey := command + " " + strings.Join(args, " ")
	if result, exists := m.Results[cmdKey]; exists {
		return result.Output, result.Error
	}
	if result, exists := m.Results[command]; exists {
		return result.Output, result.Error
	}

	return []byte("mock output"), nil
}

func (m *MockCommandExecutor) ExecuteWithWorkDir(ctx context.Context, workDir, command string, args ...string) ([]byte, error) {
	m.Commands = append(m.Commands, MockCommand{
		Command: command,
		Args:    args,
		WorkDir: workDir,
	})

	cmdKey := command + " " + strings.Join(args, " ")
	if result, exists := m.Results[cmdKey]; exists {
		return result.Output, result.Error
	}
	if result, exists := m.Results[command]; exists {
		return result.Output, result.Error
	}

	return []byte("mock output"), nil
}

func (m *MockCommandExecutor) ExecuteStream(ctx context.Context, command string, args ...string) (io.Reader, io.Reader, error) {
	m.Commands = append(m.Commands, MockCommand{
		Command: command,
		Args:    args,
	})

	return strings.NewReader("mock stdout"), strings.NewReader("mock stderr"), nil
}

// MockHTTPClient provides a mock implementation for testing
type MockHTTPClient struct {
	Responses map[string]*http.Response
	Requests  []*http.Request
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		Responses: make(map[string]*http.Response),
		Requests:  make([]*http.Request, 0),
	}
}

func (m *MockHTTPClient) SetResponse(url string, resp *http.Response) {
	m.Responses[url] = resp
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.Requests = append(m.Requests, req)

	if resp, exists := m.Responses[req.URL.String()]; exists {
		return resp, nil
	}

	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("mock response")),
	}, nil
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	return m.Do(req)
}

func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", contentType)
	return m.Do(req)
}

// MockSessionStore provides a mock implementation for testing
type MockSessionStore struct {
	Sessions map[string]*orchestrator.SessionState
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		Sessions: make(map[string]*orchestrator.SessionState),
	}
}

func (m *MockSessionStore) Save(session *orchestrator.SessionState) error {
	m.Sessions[session.SessionID] = session
	return nil
}

func (m *MockSessionStore) Load(sessionID string) (*orchestrator.SessionState, error) {
	if session, exists := m.Sessions[sessionID]; exists {
		return session, nil
	}
	return nil, errors.New("session not found")
}

func (m *MockSessionStore) List() ([]string, error) {
	var sessions []string
	for id := range m.Sessions {
		sessions = append(sessions, id)
	}
	return sessions, nil
}

func (m *MockSessionStore) Delete(sessionID string) error {
	delete(m.Sessions, sessionID)
	return nil
}

func (m *MockSessionStore) Exists(sessionID string) bool {
	_, exists := m.Sessions[sessionID]
	return exists
}
