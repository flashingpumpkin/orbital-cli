package dtui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/flashingpumpkin/orbital/internal/daemon"
)

// Run starts the manager TUI.
func Run(client *daemon.Client, projectDir string) error {
	model := NewModel(client, projectDir)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
