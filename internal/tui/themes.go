// Package tui provides the terminal user interface for orbit using bubbletea.
package tui

import (
	"os"

	"github.com/muesli/termenv"
)

// Theme represents the colour theme for the TUI.
type Theme string

const (
	// ThemeAuto automatically detects the terminal background colour.
	ThemeAuto Theme = "auto"
	// ThemeDark uses the amber colour palette designed for dark backgrounds.
	ThemeDark Theme = "dark"
	// ThemeLight uses darker colours designed for light backgrounds.
	ThemeLight Theme = "light"
)

// DetectTheme queries the terminal to determine if it has a dark or light background.
// Returns ThemeDark if the background is dark, ThemeLight if light.
// Falls back to ThemeDark if detection fails.
func DetectTheme() Theme {
	output := termenv.NewOutput(os.Stdout)
	if output.HasDarkBackground() {
		return ThemeDark
	}
	return ThemeLight
}

// ResolveTheme converts ThemeAuto to the actual detected theme.
// If the theme is already ThemeDark or ThemeLight, it is returned unchanged.
func ResolveTheme(configured Theme) Theme {
	if configured == ThemeAuto {
		return DetectTheme()
	}
	return configured
}

// ValidTheme checks if the given string is a valid theme name.
func ValidTheme(s string) bool {
	switch Theme(s) {
	case ThemeAuto, ThemeDark, ThemeLight:
		return true
	default:
		return false
	}
}
