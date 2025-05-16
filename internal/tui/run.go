package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run initializes and runs the TUI
func Run(provider, model, sessionID string) error {
	p := tea.NewProgram(
		NewModel(provider, model, sessionID),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Type assert to our model and check if there was an error
	if finalModel, ok := m.(Model); ok {
		if finalModel.err != nil {
			return finalModel.err
		}
	}

	return nil
}

// RunWithModel runs the TUI with a pre-configured model
func RunWithModel(m Model) error {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Type assert to our model and check if there was an error
	if m, ok := finalModel.(Model); ok {
		if m.err != nil {
			return m.err
		}
	}

	return nil
}
