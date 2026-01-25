// Package selector provides a TUI component for selecting orbital sessions.
package selector

import "github.com/charmbracelet/lipgloss"

// Styles contains all lipgloss styles for the session selector.
type Styles struct {
	// Title style for the selector header.
	Title lipgloss.Style

	// Separator style for horizontal rules.
	Separator lipgloss.Style

	// SessionValid style for valid session names.
	SessionValid lipgloss.Style

	// SessionInvalid style for invalid/stale session names.
	SessionInvalid lipgloss.Style

	// Cursor style for the selection indicator.
	Cursor lipgloss.Style

	// CursorInvalid style for cursor on invalid sessions.
	CursorInvalid lipgloss.Style

	// Label style for field labels.
	Label lipgloss.Style

	// Value style for field values.
	Value lipgloss.Style

	// ValueDim style for values on invalid sessions.
	ValueDim lipgloss.Style

	// Warning style for warning indicators.
	Warning lipgloss.Style

	// Success style for valid indicators.
	Success lipgloss.Style

	// Help style for the help bar at the bottom.
	Help lipgloss.Style

	// DialogTitle style for confirmation dialog titles.
	DialogTitle lipgloss.Style

	// DialogText style for confirmation dialog body text.
	DialogText lipgloss.Style

	// ButtonActive style for the active button in dialogs.
	ButtonActive lipgloss.Style

	// ButtonInactive style for inactive buttons in dialogs.
	ButtonInactive lipgloss.Style
}

// DefaultStyles returns the default style configuration.
func DefaultStyles() Styles {
	return Styles{
		Title:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")),
		Separator:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		SessionValid:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		SessionInvalid: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Cursor:         lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true),
		CursorInvalid:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		Label:          lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Value:          lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		ValueDim:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Warning:        lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Success:        lipgloss.NewStyle().Foreground(lipgloss.Color("82")),
		Help:           lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		DialogTitle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		DialogText:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		ButtonActive:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).Background(lipgloss.Color("240")).Padding(0, 2),
		ButtonInactive: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 2),
	}
}
