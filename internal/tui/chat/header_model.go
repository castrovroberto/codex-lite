package chat

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

// HeaderModel manages the header display with comprehensive system information
type HeaderModel struct {
	theme       *Theme
	provider    string
	modelName   string
	sessionID   string
	sessionUUID string
	status      string
	workingDir  string
	gitBranch   string
	gitRepo     bool
	sessionTime time.Time
	width       int
	multiLine   bool
	version     string
}

// NewHeaderModel creates a new header model
func NewHeaderModel(theme *Theme, provider, modelName, sessionID, status string) *HeaderModel {
	// Generate proper UUID for this session
	sessionUUID := uuid.New().String()

	// Get current working directory
	workingDir, _ := os.Getwd()
	if workingDir == "" {
		workingDir = "unknown"
	}

	// Get git information
	gitBranch, gitRepo := getGitInfo(workingDir)

	return &HeaderModel{
		theme:       theme,
		provider:    provider,
		modelName:   modelName,
		sessionID:   sessionID,
		sessionUUID: sessionUUID,
		status:      status,
		workingDir:  workingDir,
		gitBranch:   gitBranch,
		gitRepo:     gitRepo,
		sessionTime: time.Now(),
		width:       50, // Default width
		multiLine:   true,
		version:     "v1.0.0", // Could be made configurable
	}
}

// getGitInfo retrieves git branch and repository status
func getGitInfo(workingDir string) (string, bool) {
	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		return "", false
	}

	// Get current branch
	cmd = exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		// Try to get detached HEAD info
		cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
		cmd.Dir = workingDir
		output, err = cmd.Output()
		if err != nil {
			return "unknown", true
		}
		return fmt.Sprintf("detached@%s", strings.TrimSpace(string(output))), true
	}

	return strings.TrimSpace(string(output)), true
}

// Update handles header-specific updates
func (h *HeaderModel) Update(msg tea.Msg) (*HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		// Enable bordered display for wider terminals
		h.multiLine = msg.Width >= 80 // Lower threshold since we have nice borders
	}
	return h, nil
}

// View renders the header with comprehensive information
func (h *HeaderModel) View() string {
	if h.multiLine && h.width >= 80 {
		return h.renderBorderedHeader()
	}
	return h.renderCompactHeader()
}

// renderBorderedHeader renders the beautiful bordered header style
func (h *HeaderModel) renderBorderedHeader() string {
	// Create border styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(h.theme.Colors.Border).
		Padding(0, 1)

	// Calculate available width for content (accounting for borders and padding)
	contentWidth := h.width - 4 // 2 for borders + 2 for padding
	if contentWidth < 20 {
		contentWidth = 20
	}

	var result strings.Builder

	// First box: Application info
	appInfo := fmt.Sprintf("● CGE Chat (%s) %s", h.provider, h.version)
	appBox := borderStyle.
		Width(contentWidth).
		Foreground(h.theme.Colors.Primary).
		Bold(true).
		Render(appInfo)

	result.WriteString(appBox)
	result.WriteString("\n")

	// Second box: Session details
	sessionContent := h.buildSessionContent()
	sessionBox := borderStyle.
		Width(contentWidth).
		Foreground(h.theme.Colors.Secondary).
		Render(sessionContent)

	result.WriteString(sessionBox)

	return result.String()
}

// buildSessionContent creates the content for the session details box
func (h *HeaderModel) buildSessionContent() string {
	var lines []string

	// Main session line
	sessionLine := fmt.Sprintf("localhost session: %s", h.sessionUUID)
	lines = append(lines, sessionLine)

	// Working directory (with home directory shortening)
	workDir := h.workingDir
	if homeDir, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(workDir, homeDir) {
			workDir = "~" + workDir[len(homeDir):]
		}
	}
	lines = append(lines, fmt.Sprintf("↳ workdir: %s", workDir))

	// Model
	lines = append(lines, fmt.Sprintf("↳ model: %s", h.modelName))

	// Provider
	lines = append(lines, fmt.Sprintf("↳ provider: %s", h.provider))

	// Git branch (if available)
	if h.gitRepo {
		lines = append(lines, fmt.Sprintf("↳ branch: %s", h.gitBranch))
	}

	// Status/approval mode
	lines = append(lines, fmt.Sprintf("↳ status: %s", strings.ToLower(h.status)))

	return strings.Join(lines, "\n")
}

// renderCompactHeader renders a compact single-line header for narrow terminals
func (h *HeaderModel) renderCompactHeader() string {
	var parts []string

	// Essential info only
	parts = append(parts, fmt.Sprintf("CGE (%s)", h.provider))
	parts = append(parts, h.modelName)

	if h.gitRepo {
		parts = append(parts, fmt.Sprintf("@%s", h.gitBranch))
	}

	parts = append(parts, h.status)

	headerText := strings.Join(parts, " | ")

	// Truncate if too long
	if len(headerText) > h.width-4 {
		headerText = headerText[:h.width-7] + "..."
	}

	return h.theme.Header.Render(headerText)
}

// SetProvider updates the provider name
func (h *HeaderModel) SetProvider(provider string) {
	h.provider = provider
}

// SetModelName updates the model name
func (h *HeaderModel) SetModelName(modelName string) {
	h.modelName = modelName
}

// SetSessionID updates the session ID
func (h *HeaderModel) SetSessionID(sessionID string) {
	h.sessionID = sessionID
}

// SetStatus updates the status
func (h *HeaderModel) SetStatus(status string) {
	h.status = status
}

// SetVersion updates the version string
func (h *HeaderModel) SetVersion(version string) {
	h.version = version
}

// RefreshGitInfo refreshes the git branch information
func (h *HeaderModel) RefreshGitInfo() {
	h.gitBranch, h.gitRepo = getGitInfo(h.workingDir)
}

// GetHeight returns the header height based on display mode
func (h *HeaderModel) GetHeight() int {
	if h.multiLine && h.width >= 80 {
		// Two bordered boxes: each takes 3 lines (border + content + border)
		// Plus one line spacing between boxes
		return 7 // 3 + 1 + 3
	}
	return h.theme.HeaderHeight // Default single line
}

// GetSessionID returns the session ID
func (h *HeaderModel) GetSessionID() string {
	return h.sessionID
}

// GetSessionUUID returns the full session UUID
func (h *HeaderModel) GetSessionUUID() string {
	return h.sessionUUID
}

// GetModelName returns the model name
func (h *HeaderModel) GetModelName() string {
	return h.modelName
}

// GetProvider returns the provider name
func (h *HeaderModel) GetProvider() string {
	return h.provider
}

// GetStatus returns the status
func (h *HeaderModel) GetStatus() string {
	return h.status
}

// GetVersion returns the version string
func (h *HeaderModel) GetVersion() string {
	return h.version
}

// GetWorkingDirectory returns the working directory
func (h *HeaderModel) GetWorkingDirectory() string {
	return h.workingDir
}

// GetGitBranch returns the git branch
func (h *HeaderModel) GetGitBranch() string {
	return h.gitBranch
}

// IsGitRepo returns whether we're in a git repository
func (h *HeaderModel) IsGitRepo() bool {
	return h.gitRepo
}

// GetSessionTime returns the session start time
func (h *HeaderModel) GetSessionTime() time.Time {
	return h.sessionTime
}
