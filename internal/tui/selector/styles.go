// Package selector provides a TUI component for selecting orbital sessions.
package selector

import "github.com/charmbracelet/lipgloss"

// Theme represents the colour theme for the selector.
type Theme string

const (
	// ThemeDark uses colours optimised for dark terminal backgrounds.
	ThemeDark Theme = "dark"
	// ThemeLight uses colours optimised for light terminal backgrounds.
	ThemeLight Theme = "light"
)

// Dark theme colour palette for session selector
const (
	colourAmber       = lipgloss.Color("214") // #FFB000 - Primary amber
	colourAmberDim    = lipgloss.Color("136") // #996600 - Inactive, separators
	colourAmberLight  = lipgloss.Color("222") // #FFD966 - Body text, values
	colourAmberFaded  = lipgloss.Color("178") // #B38F00 - Labels, secondary
	colourBackground  = lipgloss.Color("0")   // #000000 - Background
	colourSuccess     = lipgloss.Color("82")  // #00FF00 - Valid states
	colourWarning     = lipgloss.Color("208") // #FFAA00 - Warnings
)

// Light theme colour palette
const (
	colourAmberDark       = lipgloss.Color("94")  // #8B6914 - Primary dark amber
	colourAmberDarkDim    = lipgloss.Color("58")  // #5C4A0A - Inactive, separators
	colourAmberDarkMid    = lipgloss.Color("94")  // #6B5A1E - Body text, values
	colourAmberDarkFaded  = lipgloss.Color("101") // #7A6A30 - Labels, secondary
	colourBackgroundLight = lipgloss.Color("231") // #FFFFFF - Light background
	colourSuccessDark     = lipgloss.Color("22")  // #008000 - Valid states
	colourWarningDark     = lipgloss.Color("166") // #CC5500 - Warnings
)

// Box drawing characters
const (
	boxTopLeft     = "╔"
	boxTopRight    = "╗"
	boxBottomLeft  = "╚"
	boxBottomRight = "╝"
	boxHorizontal  = "═"
	boxVertical    = "║"
	boxLeftT       = "╠"
	boxRightT      = "╣"
)

// Styles contains all lipgloss styles for the session selector.
type Styles struct {
	// Border style for frame borders.
	Border lipgloss.Style

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

	// HelpKey style for keyboard shortcuts.
	HelpKey lipgloss.Style

	// DialogTitle style for confirmation dialog titles.
	DialogTitle lipgloss.Style

	// DialogText style for confirmation dialog body text.
	DialogText lipgloss.Style

	// ButtonActive style for the active button in dialogs.
	ButtonActive lipgloss.Style

	// ButtonInactive style for inactive buttons in dialogs.
	ButtonInactive lipgloss.Style

	// Brand style for the logo.
	Brand lipgloss.Style
}

// DefaultStyles returns the dark theme style configuration.
func DefaultStyles() Styles {
	return DarkStyles()
}

// DarkStyles returns the amber terminal style configuration for dark backgrounds.
func DarkStyles() Styles {
	return Styles{
		Border:         lipgloss.NewStyle().Foreground(colourAmber),
		Title:          lipgloss.NewStyle().Bold(true).Foreground(colourAmber),
		Separator:      lipgloss.NewStyle().Foreground(colourAmber),
		SessionValid:   lipgloss.NewStyle().Foreground(colourAmberLight),
		SessionInvalid: lipgloss.NewStyle().Foreground(colourAmberDim),
		Cursor:         lipgloss.NewStyle().Foreground(colourAmber).Bold(true),
		CursorInvalid:  lipgloss.NewStyle().Foreground(colourWarning).Bold(true),
		Label:          lipgloss.NewStyle().Foreground(colourAmberFaded),
		Value:          lipgloss.NewStyle().Foreground(colourAmberLight),
		ValueDim:       lipgloss.NewStyle().Foreground(colourAmberDim),
		Warning:        lipgloss.NewStyle().Foreground(colourWarning),
		Success:        lipgloss.NewStyle().Foreground(colourSuccess),
		Help:           lipgloss.NewStyle().Foreground(colourAmberDim),
		HelpKey:        lipgloss.NewStyle().Foreground(colourAmberFaded),
		DialogTitle:    lipgloss.NewStyle().Bold(true).Foreground(colourAmber),
		DialogText:     lipgloss.NewStyle().Foreground(colourAmberLight),
		ButtonActive:   lipgloss.NewStyle().Bold(true).Foreground(colourBackground).Background(colourAmber).Padding(0, 2),
		ButtonInactive: lipgloss.NewStyle().Foreground(colourAmberFaded).Padding(0, 2),
		Brand:          lipgloss.NewStyle().Bold(true).Foreground(colourAmber),
	}
}

// LightStyles returns the style configuration for light backgrounds.
func LightStyles() Styles {
	return Styles{
		Border:         lipgloss.NewStyle().Foreground(colourAmberDark),
		Title:          lipgloss.NewStyle().Bold(true).Foreground(colourAmberDark),
		Separator:      lipgloss.NewStyle().Foreground(colourAmberDark),
		SessionValid:   lipgloss.NewStyle().Foreground(colourAmberDarkMid),
		SessionInvalid: lipgloss.NewStyle().Foreground(colourAmberDarkDim),
		Cursor:         lipgloss.NewStyle().Foreground(colourAmberDark).Bold(true),
		CursorInvalid:  lipgloss.NewStyle().Foreground(colourWarningDark).Bold(true),
		Label:          lipgloss.NewStyle().Foreground(colourAmberDarkFaded),
		Value:          lipgloss.NewStyle().Foreground(colourAmberDarkMid),
		ValueDim:       lipgloss.NewStyle().Foreground(colourAmberDarkDim),
		Warning:        lipgloss.NewStyle().Foreground(colourWarningDark),
		Success:        lipgloss.NewStyle().Foreground(colourSuccessDark),
		Help:           lipgloss.NewStyle().Foreground(colourAmberDarkDim),
		HelpKey:        lipgloss.NewStyle().Foreground(colourAmberDarkFaded),
		DialogTitle:    lipgloss.NewStyle().Bold(true).Foreground(colourAmberDark),
		DialogText:     lipgloss.NewStyle().Foreground(colourAmberDarkMid),
		ButtonActive:   lipgloss.NewStyle().Bold(true).Foreground(colourBackgroundLight).Background(colourAmberDark).Padding(0, 2),
		ButtonInactive: lipgloss.NewStyle().Foreground(colourAmberDarkFaded).Padding(0, 2),
		Brand:          lipgloss.NewStyle().Bold(true).Foreground(colourAmberDark),
	}
}

// GetStyles returns the Styles for the given theme.
// Falls back to dark theme for unknown values.
func GetStyles(theme Theme) Styles {
	switch theme {
	case ThemeLight:
		return LightStyles()
	default:
		return DarkStyles()
	}
}

// RenderTopBorder renders the top border of a frame.
func RenderTopBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(boxTopLeft + boxTopRight)
	}
	return style.Render(boxTopLeft + repeatString(boxHorizontal, width-2) + boxTopRight)
}

// RenderBottomBorder renders the bottom border of a frame.
func RenderBottomBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(boxBottomLeft + boxBottomRight)
	}
	return style.Render(boxBottomLeft + repeatString(boxHorizontal, width-2) + boxBottomRight)
}

// RenderMidBorder renders a horizontal mid-frame border.
func RenderMidBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(boxLeftT + boxRightT)
	}
	return style.Render(boxLeftT + repeatString(boxHorizontal, width-2) + boxRightT)
}

// repeatString repeats a string n times.
func repeatString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
