package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run initializes and runs the TUI
func Run(provider, model, sessionID string) error {
	m := NewModel(provider, model, sessionID)
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	m.SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		return fmt.Errorf("error running TUI program: %w", err)
	}

	if fm, ok := finalModel.(*Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	} else {
		return fmt.Errorf("unexpected model type returned from TUI: %T", finalModel)
	}

	return nil
}

// RunWithModel runs the TUI with a pre-configured model
func RunWithModel(m *Model) error {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	m.SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		return fmt.Errorf("error running TUI program: %w", err)
	}

	if fm, ok := finalModel.(*Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	} else {
		return fmt.Errorf("unexpected model type returned from TUI: %T", finalModel)
	}

	return nil
}
