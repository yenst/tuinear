package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var theme = struct {
	border color.Color
	text   color.Color
	muted  color.Color
	accent color.Color
	green  color.Color
	yellow color.Color
	red    color.Color
}{
	border: lipgloss.Color("8"),
	text:   lipgloss.NoColor{},
	muted:  lipgloss.Color("8"),
	accent: lipgloss.Color("5"),
	green:  lipgloss.Color("2"),
	yellow: lipgloss.Color("3"),
	red:    lipgloss.Color("1"),
}

var (
	appStyle    = lipgloss.NewStyle()
	headerStyle = lipgloss.NewStyle().
			Foreground(theme.text).
			Padding(0, 1)
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.accent)
	mutedStyle         = lipgloss.NewStyle().Foreground(theme.muted)
	accentStyle        = lipgloss.NewStyle().Foreground(theme.accent)
	activeAccountStyle = lipgloss.NewStyle().Foreground(theme.green).Bold(true)
	selectedRowStyle   = lipgloss.NewStyle().
				Reverse(true).
				Bold(true)
	panelStyle = lipgloss.NewStyle().
			Foreground(theme.text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.border).
			Padding(0, 1)
)
