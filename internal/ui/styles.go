package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var theme = struct {
	background color.Color
	panel      color.Color
	border     color.Color
	text       color.Color
	muted      color.Color
	accent     color.Color
	selected   color.Color
	green      color.Color
	yellow     color.Color
	red        color.Color
}{
	background: lipgloss.Color("#0F0F11"),
	panel:      lipgloss.Color("#171719"),
	border:     lipgloss.Color("#303036"),
	text:       lipgloss.Color("#E8E8EA"),
	muted:      lipgloss.Color("#898990"),
	accent:     lipgloss.Color("#7C83FF"),
	selected:   lipgloss.Color("#2A2B3A"),
	green:      lipgloss.Color("#4CB782"),
	yellow:     lipgloss.Color("#E2B93B"),
	red:        lipgloss.Color("#EB5757"),
}

var (
	appStyle = lipgloss.NewStyle().
			Background(theme.background).
			Foreground(theme.text)
	headerStyle = lipgloss.NewStyle().
			Background(theme.panel).
			Foreground(theme.text).
			Padding(0, 1)
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.accent)
	mutedStyle       = lipgloss.NewStyle().Foreground(theme.muted)
	accentStyle      = lipgloss.NewStyle().Foreground(theme.accent)
	selectedRowStyle = lipgloss.NewStyle().
				Background(theme.selected).
				Foreground(theme.text).
				Bold(true)
	panelStyle = lipgloss.NewStyle().
			Background(theme.panel).
			Foreground(theme.text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.border).
			Padding(0, 1)
)
