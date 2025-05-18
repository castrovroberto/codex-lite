package tui

import (
	"fmt"
	"os"

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
		os.Exit(1)
	}

	// Type assert to our model and check if there was an error
	if fm, ok := finalModel.(Model); ok {
		if fm.err != nil {
			return fm.err
		}
	} else if fmp, ok := finalModel.(*Model); ok {
		if fmp.err != nil {
			return fmp.err
		}
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
		os.Exit(1)
	}

	// Type assert to our model and check if there was an error
	if fm, ok := finalModel.(Model); ok {
		if fm.err != nil {
			return fm.err
		}
	} else if fmp, ok := finalModel.(*Model); ok {
		if fmp.err != nil {
			return fmp.err
		}
	}

	return nil
}
