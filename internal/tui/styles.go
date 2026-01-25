// Package tui provides the terminal user interface for orbit using bubbletea.
package tui

import "github.com/charmbracelet/lipgloss"

// Amber Terminal colour palette
// These constants define the visual identity of the Orbital TUI.
const (
	// Primary colours
	ColourAmber       = lipgloss.Color("214") // #FFB000 - Headers, active states, borders
	ColourAmberDim    = lipgloss.Color("136") // #996600 - Inactive borders, separators
	ColourAmberLight  = lipgloss.Color("222") // #FFD966 - Body text, values
	ColourAmberFaded  = lipgloss.Color("178") // #B38F00 - Labels, secondary text
	ColourBackground  = lipgloss.Color("0")   // #000000 - Terminal background
	ColourSuccess     = lipgloss.Color("82")  // #00FF00 - Completed tasks, valid states
	ColourWarning     = lipgloss.Color("208") // #FFAA00 - >80% budget/iterations
	ColourError       = lipgloss.Color("196") // #FF3300 - Errors, invalid states
	ColourWorktree    = lipgloss.Color("141") // Purple - Worktree label accent
	ColourWorktreeVal = lipgloss.Color("183") // Light purple - Worktree values
)

// Box drawing characters for the UI frame.
// Outer frame uses double lines, inner divisions use single lines.
const (
	// Outer frame (double line)
	BoxTopLeft     = "╔"
	BoxTopRight    = "╗"
	BoxBottomLeft  = "╚"
	BoxBottomRight = "╝"
	BoxHorizontal  = "═"
	BoxVertical    = "║"
	BoxLeftT       = "╠"
	BoxRightT      = "╣"
	BoxCross       = "╬"

	// Inner divisions (single line)
	InnerHorizontal = "─"
	InnerVertical   = "│"
	InnerLeftT      = "├"
	InnerRightT     = "┤"
	InnerTopT       = "┬"
	InnerBottomT    = "┴"
	InnerCross      = "┼"
)

// Progress bar characters
const (
	BarFilled = "█"
	BarEmpty  = "░"
	BarWidth  = 20
)

// Status indicator icons
const (
	IconPending    = "○"
	IconInProgress = "→"
	IconComplete   = "●"
	IconError      = "✗"
	IconValid      = "✓"
	IconWarning    = "⚠"
	IconWorktree   = "⎇"
	IconBrand      = "◆"
)

// Styles contains all lipgloss styles for the UI.
type Styles struct {
	// Frame and borders
	Border    lipgloss.Style
	BorderDim lipgloss.Style

	// Text hierarchy
	Header lipgloss.Style // Bold, full amber for headers
	Label  lipgloss.Style // Dim amber for labels
	Value  lipgloss.Style // Light amber for values

	// Status colours
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style

	// Task states
	TaskPending    lipgloss.Style
	TaskInProgress lipgloss.Style
	TaskComplete   lipgloss.Style

	// Special areas
	ScrollArea      lipgloss.Style
	TooSmallMessage lipgloss.Style

	// Worktree styling
	WorktreeLabel lipgloss.Style
	WorktreeValue lipgloss.Style

	// Tab bar
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBar      lipgloss.Style

	// Help bar
	HelpBar lipgloss.Style
	HelpKey lipgloss.Style

	// Brand
	Brand lipgloss.Style
}

// defaultStyles returns the amber terminal style configuration.
func defaultStyles() Styles {
	return Styles{
		// Frame and borders
		Border:    lipgloss.NewStyle().Foreground(ColourAmber),
		BorderDim: lipgloss.NewStyle().Foreground(ColourAmberDim),

		// Text hierarchy
		Header: lipgloss.NewStyle().Foreground(ColourAmber).Bold(true),
		Label:  lipgloss.NewStyle().Foreground(ColourAmberFaded),
		Value:  lipgloss.NewStyle().Foreground(ColourAmberLight),

		// Status colours
		Success: lipgloss.NewStyle().Foreground(ColourSuccess),
		Warning: lipgloss.NewStyle().Foreground(ColourWarning),
		Error:   lipgloss.NewStyle().Foreground(ColourError),

		// Task states
		TaskPending:    lipgloss.NewStyle().Foreground(ColourAmberDim),
		TaskInProgress: lipgloss.NewStyle().Foreground(ColourAmber),
		TaskComplete:   lipgloss.NewStyle().Foreground(ColourSuccess),

		// Special areas
		ScrollArea:      lipgloss.NewStyle(),
		TooSmallMessage: lipgloss.NewStyle().Foreground(ColourWarning).Bold(true),

		// Worktree styling
		WorktreeLabel: lipgloss.NewStyle().Foreground(ColourWorktree).Bold(true),
		WorktreeValue: lipgloss.NewStyle().Foreground(ColourWorktreeVal),

		// Tab bar - active tab with amber background
		TabActive:   lipgloss.NewStyle().Foreground(ColourBackground).Background(ColourAmber).Bold(true).Padding(0, 1),
		TabInactive: lipgloss.NewStyle().Foreground(ColourAmberFaded).Padding(0, 1),
		TabBar:      lipgloss.NewStyle().Foreground(ColourAmberDim),

		// Help bar
		HelpBar: lipgloss.NewStyle().Foreground(ColourAmberDim),
		HelpKey: lipgloss.NewStyle().Foreground(ColourAmberFaded),

		// Brand
		Brand: lipgloss.NewStyle().Foreground(ColourAmber).Bold(true),
	}
}

// RenderProgressBar renders a progress bar with the given ratio (0.0 to 1.0).
// Returns a string like [████████░░░░░░░░░░░░].
func RenderProgressBar(ratio float64, width int, normalStyle, warningStyle lipgloss.Style) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	// Calculate filled portion
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}

	// Build the bar
	bar := ""
	for i := 0; i < filled; i++ {
		bar += BarFilled
	}
	for i := filled; i < width; i++ {
		bar += BarEmpty
	}

	// Apply colour based on ratio
	style := normalStyle
	if ratio > 0.8 {
		style = warningStyle
	}

	return "[" + style.Render(bar) + "]"
}

// RenderDoubleBorder renders a horizontal double-line border of the given width.
func RenderDoubleBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(BoxLeftT + BoxRightT)
	}
	return style.Render(BoxLeftT + repeatString(BoxHorizontal, width-2) + BoxRightT)
}

// RenderTopBorder renders the top border of the frame.
func RenderTopBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(BoxTopLeft + BoxTopRight)
	}
	return style.Render(BoxTopLeft + repeatString(BoxHorizontal, width-2) + BoxTopRight)
}

// RenderBottomBorder renders the bottom border of the frame.
func RenderBottomBorder(width int, style lipgloss.Style) string {
	if width <= 2 {
		return style.Render(BoxBottomLeft + BoxBottomRight)
	}
	return style.Render(BoxBottomLeft + repeatString(BoxHorizontal, width-2) + BoxBottomRight)
}

// RenderSingleBorder renders a horizontal single-line border of the given width.
func RenderSingleBorder(width int, style lipgloss.Style) string {
	return style.Render(repeatString(InnerHorizontal, width))
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
