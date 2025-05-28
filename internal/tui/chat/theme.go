package chat

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Theme contains all styling and dimension constants for the TUI
type Theme struct {
	// Layout dimensions
	HeaderHeight      int
	StatusBarHeight   int
	MinViewportHeight int

	// Color palette
	Colors struct {
		Primary    lipgloss.Color
		Secondary  lipgloss.Color
		Background lipgloss.Color
		Surface    lipgloss.Color
		Error      lipgloss.Color
		Success    lipgloss.Color
		Warning    lipgloss.Color
		Muted      lipgloss.Color
		Border     lipgloss.Color
	}

	// Component styles
	Header       lipgloss.Style
	StatusBar    lipgloss.Style
	Error        lipgloss.Style
	Sender       lipgloss.Style
	Time         lipgloss.Style
	Code         lipgloss.Style
	Suggestion   lipgloss.Style
	ThinkingTime lipgloss.Style

	// Tool-specific styles
	ToolCall    lipgloss.Style
	ToolResult  lipgloss.Style
	ToolSuccess lipgloss.Style
	ToolError   lipgloss.Style
	ToolParams  lipgloss.Style

	// Viewport styling
	ViewportBorder lipgloss.Style
}

// NewDefaultTheme creates the default theme configuration
func NewDefaultTheme() *Theme {
	theme := &Theme{
		// Layout dimensions
		HeaderHeight:      2,
		StatusBarHeight:   1,
		MinViewportHeight: 3,
	}

	// Color palette
	theme.Colors.Primary = lipgloss.Color("63")
	theme.Colors.Secondary = lipgloss.Color("230")
	theme.Colors.Background = lipgloss.Color("236")
	theme.Colors.Surface = lipgloss.Color("237")
	theme.Colors.Error = lipgloss.Color("196")
	theme.Colors.Success = lipgloss.Color("28")
	theme.Colors.Warning = lipgloss.Color("214")
	theme.Colors.Muted = lipgloss.Color("241")
	theme.Colors.Border = lipgloss.Color("63")

	// Component styles
	theme.Header = lipgloss.NewStyle().
		Background(theme.Colors.Primary).
		Foreground(theme.Colors.Secondary).
		Padding(0, 1)

	theme.StatusBar = lipgloss.NewStyle().
		Background(theme.Colors.Surface).
		Foreground(lipgloss.Color("252"))

	theme.Error = lipgloss.NewStyle().
		Foreground(theme.Colors.Error)

	theme.Sender = lipgloss.NewStyle().
		Foreground(theme.Colors.Primary).
		Bold(true)

	theme.Time = lipgloss.NewStyle().
		Foreground(theme.Colors.Muted).
		Italic(true)

	theme.Code = lipgloss.NewStyle().
		Background(theme.Colors.Background).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	theme.Suggestion = lipgloss.NewStyle().
		Background(theme.Colors.Surface).
		Foreground(lipgloss.Color("252"))

	theme.ThinkingTime = lipgloss.NewStyle().
		Foreground(lipgloss.Color("242"))

	// Tool-specific styles
	theme.ToolCall = lipgloss.NewStyle().
		Background(lipgloss.Color("25")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Margin(0, 0, 1, 0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33"))

	theme.ToolResult = lipgloss.NewStyle().
		Background(theme.Colors.Background).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	theme.ToolSuccess = lipgloss.NewStyle().
		Background(lipgloss.Color("22")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Colors.Success)

	theme.ToolError = lipgloss.NewStyle().
		Background(lipgloss.Color("52")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Colors.Error)

	theme.ToolParams = lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

	// Viewport styling
	theme.ViewportBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Colors.Border)

	return theme
}

// LayoutDimensions provides centralized dimension calculations
type LayoutDimensions struct {
	theme *Theme
}

// NewLayoutDimensions creates a new layout dimension calculator
func NewLayoutDimensions(theme *Theme) *LayoutDimensions {
	return &LayoutDimensions{theme: theme}
}

// GetHeaderHeight returns the header height
func (ld *LayoutDimensions) GetHeaderHeight() int {
	return ld.theme.HeaderHeight
}

// GetStatusBarHeight returns the status bar height
func (ld *LayoutDimensions) GetStatusBarHeight() int {
	return ld.theme.StatusBarHeight
}

// GetMinViewportHeight returns the minimum viewport height
func (ld *LayoutDimensions) GetMinViewportHeight() int {
	return ld.theme.MinViewportHeight
}

// GetViewportFrameHeight returns the height consumed by viewport border frames
func (ld *LayoutDimensions) GetViewportFrameHeight() int {
	return 2 // Border frame height (top + bottom)
}

// CalculateViewportHeight calculates the optimal viewport height given constraints
func (ld *LayoutDimensions) CalculateViewportHeight(windowHeight, textareaHeight, suggestionAreaHeight, viewportFrameHeight int) int {
	headerHeight := ld.GetHeaderHeight()
	statusBarHeight := ld.GetStatusBarHeight()

	// Calculate available height for viewport content
	availableHeight := windowHeight - headerHeight - statusBarHeight - textareaHeight - suggestionAreaHeight - viewportFrameHeight

	// Ensure minimum viewport height
	if availableHeight < ld.GetMinViewportHeight() {
		return ld.GetMinViewportHeight()
	}

	return availableHeight
}

// ValidateLayout validates that all component heights sum to the window height
func (ld *LayoutDimensions) ValidateLayout(windowHeight, textareaHeight, suggestionAreaHeight, viewportFrameHeight int) error {
	totalHeight := ld.GetHeaderHeight() + ld.GetStatusBarHeight() +
		textareaHeight + suggestionAreaHeight + viewportFrameHeight
	calculatedViewportHeight := ld.CalculateViewportHeight(windowHeight, textareaHeight, suggestionAreaHeight, viewportFrameHeight)

	actualTotal := totalHeight + calculatedViewportHeight
	if actualTotal != windowHeight {
		return fmt.Errorf("layout height mismatch: components sum to %d, but window height is %d (diff: %d)",
			actualTotal, windowHeight, windowHeight-actualTotal)
	}
	return nil
}
