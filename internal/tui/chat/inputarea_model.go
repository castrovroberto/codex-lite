package chat

import (
	"strings"

	"github.com/castrovroberto/CGE/internal/logger"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputAreaModel manages the input area including textarea and suggestions
type InputAreaModel struct {
	theme             *Theme
	textarea          textarea.Model
	suggestions       []string
	selected          int
	availableCommands []string
	isEditing         bool
	editingIndex      int
	width             int
	lastInputValue    string // Track last input value to detect changes
}

// NewInputAreaModel creates a new input area model
func NewInputAreaModel(theme *Theme, availableCommands []string) *InputAreaModel {
	// Initialize textarea with better styling
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+E to edit last, Tab for completion)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 2000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	return &InputAreaModel{
		theme:             theme,
		textarea:          ta,
		suggestions:       nil,
		selected:          -1,
		availableCommands: availableCommands,
		isEditing:         false,
		editingIndex:      -1,
		width:             50,
		lastInputValue:    "",
	}
}

// Update handles input area updates
func (i *InputAreaModel) Update(msg tea.Msg) (*InputAreaModel, tea.Cmd) {
   var cmd tea.Cmd

   // Intercept window resize to adjust width only, avoid passing resize to textarea
   if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
       i.width = wmsg.Width
       i.textarea.SetWidth(wmsg.Width)
       return i, nil
   }
	// Update textarea for other messages
	i.textarea, cmd = i.textarea.Update(msg)

	// Only update suggestions if the input value actually changed
	currentValue := i.textarea.Value()
	if currentValue != i.lastInputValue {
		i.lastInputValue = currentValue
		i.updateSuggestions(currentValue)
	}

	return i, cmd
}

// View renders the input area
func (i *InputAreaModel) View() string {
	var view strings.Builder

	// Textarea
	view.WriteString(i.textarea.View())

	// Render suggestions if any
	if len(i.suggestions) > 0 {
		view.WriteString("\n") // Add a line break before suggestions
		suggestionLines := make([]string, len(i.suggestions))
		for idx, sug := range i.suggestions {
			if idx == i.selected {
				suggestionLines[idx] = i.theme.Suggestion.Copy().Reverse(true).Render("> " + sug)
			} else {
				suggestionLines[idx] = i.theme.Suggestion.Render("  " + sug)
			}
		}
		view.WriteString(strings.Join(suggestionLines, "\n"))
	}

	return view.String()
}

// updateSuggestions populates the suggestions list based on the input
func (i *InputAreaModel) updateSuggestions(input string) {
	previousSuggestionCount := len(i.suggestions)

	if strings.HasPrefix(input, "/") {
		i.suggestions = nil
		i.selected = 0
		for _, cmd := range i.availableCommands {
			if strings.HasPrefix(cmd, input) {
				i.suggestions = append(i.suggestions, cmd)
			}
		}
		// Ensure selected index is valid
		if len(i.suggestions) == 0 {
			i.selected = -1
		} else if i.selected >= len(i.suggestions) {
			i.selected = 0
		}

		// Log suggestion updates for debugging
		if len(i.suggestions) > 0 {
			logger.Get().Debug("Updated suggestions", "input", input, "count", len(i.suggestions), "selected", i.selected)
		}
	} else {
		i.suggestions = nil
		i.selected = -1
	}

	// Log if suggestion count changed
	if len(i.suggestions) != previousSuggestionCount {
		logger.Get().Debug("Suggestion count changed",
			"previousCount", previousSuggestionCount,
			"newCount", len(i.suggestions))
	}
}

// HandleSuggestionNavigation handles up/down arrow navigation
func (i *InputAreaModel) HandleSuggestionNavigation(direction string) bool {
	if len(i.suggestions) == 0 {
		return false // No suggestions to navigate
	}

	switch direction {
	case "up":
		i.selected = (i.selected - 1 + len(i.suggestions)) % len(i.suggestions)
	case "down":
		i.selected = (i.selected + 1) % len(i.suggestions)
	}

	return true // Handled
}

// ApplySelectedSuggestion applies the currently selected suggestion
func (i *InputAreaModel) ApplySelectedSuggestion() bool {
	if len(i.suggestions) > 0 && i.selected >= 0 && i.selected < len(i.suggestions) {
		selectedSuggestion := i.suggestions[i.selected]
		i.textarea.SetValue(selectedSuggestion)
		i.textarea.CursorEnd()
		i.lastInputValue = selectedSuggestion // Update tracked value
		i.suggestions = nil
		i.selected = -1
		return true
	}
	return false
}

// ClearSuggestions clears all suggestions
func (i *InputAreaModel) ClearSuggestions() {
	i.suggestions = nil
	i.selected = -1
}

// GetValue returns the current textarea value
func (i *InputAreaModel) GetValue() string {
	return i.textarea.Value()
}

// SetValue sets the textarea value
func (i *InputAreaModel) SetValue(value string) {
	i.textarea.SetValue(value)
	i.lastInputValue = value // Update tracked value
}

// Reset resets the textarea
func (i *InputAreaModel) Reset() {
	i.textarea.Reset()
	i.lastInputValue = "" // Reset tracked value
}

// CursorEnd moves cursor to end
func (i *InputAreaModel) CursorEnd() {
	i.textarea.CursorEnd()
}

// Focus focuses the textarea
func (i *InputAreaModel) Focus() {
	i.textarea.Focus()
}

// Blur blurs the textarea
func (i *InputAreaModel) Blur() {
	i.textarea.Blur()
}

// GetHeight returns the input area height including suggestions
func (i *InputAreaModel) GetHeight() int {
	height := i.textarea.Height()
	if len(i.suggestions) > 0 {
		height += len(i.suggestions) + 1 // suggestions + newline
	}
	return height
}

// GetSuggestionAreaHeight returns just the suggestion area height
func (i *InputAreaModel) GetSuggestionAreaHeight() int {
	if len(i.suggestions) > 0 {
		return len(i.suggestions) + 1 // suggestions + newline
	}
	return 0
}

// HasSuggestions returns true if there are active suggestions
func (i *InputAreaModel) HasSuggestions() bool {
	return len(i.suggestions) > 0
}

// StartEditing starts editing mode
func (i *InputAreaModel) StartEditing(text string, index int) {
	i.isEditing = true
	i.editingIndex = index
	i.textarea.SetValue(text)
	i.lastInputValue = text // Update tracked value
}

// StopEditing stops editing mode
func (i *InputAreaModel) StopEditing() {
	i.isEditing = false
	i.editingIndex = -1
	i.textarea.Blur()
	i.textarea.Reset()
	i.lastInputValue = "" // Reset tracked value
	i.textarea.Placeholder = "Type your message... (Ctrl+E to edit last, Tab for completion)"
}

// IsEditing returns true if in editing mode
func (i *InputAreaModel) IsEditing() bool {
	return i.isEditing
}

// GetEditingIndex returns the index being edited
func (i *InputAreaModel) GetEditingIndex() int {
	return i.editingIndex
}

// UpdateSuggestions manually updates suggestions for testing
func (i *InputAreaModel) UpdateSuggestions(input string) {
	i.updateSuggestions(input)
}
