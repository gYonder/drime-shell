package ui

import "github.com/charmbracelet/lipgloss"

// Theme represents the user interface color theme
type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

// DetectTheme returns the detected terminal theme (Dark or Light)
func DetectTheme() Theme {
	if lipgloss.HasDarkBackground() {
		return ThemeDark
	}
	return ThemeLight
}
