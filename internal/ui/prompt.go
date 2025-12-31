package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// RenderPrompt renders a Powerline-style prompt
func RenderPrompt(user, path, contextName string, inVault bool) string {
	userBg := currentTheme.Mauve
	userFg := currentTheme.Base
	pathBg := currentTheme.Surface
	pathFg := currentTheme.Text
	ctxBg := currentTheme.Blue
	if inVault {
		ctxBg = currentTheme.Red
	}
	ctxFg := currentTheme.Base

	userStyle := lipgloss.NewStyle().Background(userBg).Foreground(userFg).Padding(0, 1).Bold(true)
	pathStyle := lipgloss.NewStyle().Background(pathBg).Foreground(pathFg).Padding(0, 1)
	ctxStyle := lipgloss.NewStyle().Background(ctxBg).Foreground(ctxFg).Padding(0, 1)

	seg1 := userStyle.Render(user)
	sep1 := lipgloss.NewStyle().Foreground(userBg).Background(pathBg).Render("")
	seg2 := pathStyle.Render(path)

	if contextName != "" {
		sep2 := lipgloss.NewStyle().Foreground(pathBg).Background(ctxBg).Render("")
		seg3 := ctxStyle.Render(contextName)
		sep3 := lipgloss.NewStyle().Foreground(ctxBg).Render("")
		return fmt.Sprintf("%s%s%s%s%s%s ", seg1, sep1, seg2, sep2, seg3, sep3)
	}

	sep2 := lipgloss.NewStyle().Foreground(pathBg).Render("")
	return fmt.Sprintf("%s%s%s%s ", seg1, sep1, seg2, sep2)
}
