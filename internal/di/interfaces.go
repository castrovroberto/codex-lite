package di

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/castrovroberto/CGE/internal/orchestrator"
)

// FileSystemService abstracts file system operations
type FileSystemService interface {
	WriteFile(path string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	ListDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	Exists(path string) bool
	IsDir(path string) bool
}

// CommandExecutor abstracts command execution
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args ...string) (output []byte, err error)
	ExecuteWithWorkDir(ctx context.Context, workDir, command string, args ...string) (output []byte, err error)
	ExecuteStream(ctx context.Context, command string, args ...string) (stdout, stderr io.Reader, err error)
}

// HTTPClient abstracts HTTP client operations
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

// SessionStore abstracts session persistence
type SessionStore interface {
	Save(session *orchestrator.SessionState) error
	Load(sessionID string) (*orchestrator.SessionState, error)
	List() ([]string, error)
	Delete(sessionID string) error
	Exists(sessionID string) bool
}

// ConfigProvider abstracts configuration access
type ConfigProvider interface {
	GetLLMConfig() LLMConfig
	GetProjectConfig() ProjectConfig
	GetLoggingConfig() LoggingConfig
}

// LLMConfig represents LLM configuration
type LLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// ProjectConfig represents project configuration
type ProjectConfig struct {
	WorkspaceRoot string
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string
}
