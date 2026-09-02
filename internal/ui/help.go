package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) renderHelpOverlay(base string, width, height int) string {
	popupWidth := min(72, max(36, width-6))
	popupHeight := min(22, max(8, height-2))
	innerWidth, innerHeight := panelInnerSize(popupWidth, popupHeight)

	var lines []string
	if popupWidth >= 64 {
		columnWidth := (innerWidth - 3) / 2
		left, right := helpColumns(columnWidth)
		leftColumn := fitLines(left, innerHeight)
		rightColumn := fitLines(right, innerHeight)
		columns := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(columnWidth).Render(leftColumn),
			mutedStyle.Render(" │ "),
			lipgloss.NewStyle().Width(columnWidth).Render(rightColumn),
		)
		lines = strings.Split(columns, "\n")
	} else {
		left, right := helpColumns(innerWidth)
		lines = append(left, "")
		lines = append(lines, right...)
	}

	popup := panel("Tuinear keybindings", popupWidth, popupHeight, fitLines(lines, innerHeight))
	x := max(0, (width-lipgloss.Width(popup))/2)
	y := max(0, (height-lipgloss.Height(popup))/2)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(popup).X(x).Y(y).Z(1),
	).Render()
}

func helpColumns(width int) ([]string, []string) {
	left := []string{
		accentStyle.Bold(true).Render("Navigate"),
		helpBinding("j / k", "next / previous issue", width),
		helpBinding("g / G", "first / last issue", width),
		helpBinding("tab / shift+tab", "next / previous team", width),
		helpBinding("a / A", "next / previous account", width),
		"",
		accentStyle.Bold(true).Render("Find & reload"),
		helpBinding("/", "search", width),
		helpBinding("f", "saved filters", width),
		helpBinding("r", "refresh", width),
	}
	right := []string{
		accentStyle.Bold(true).Render("Ticket actions"),
		helpBinding("enter", "all actions", width),
		helpBinding("e / s / p", "title / status / priority", width),
		helpBinding("u / P / l", "assignee / project / labels", width),
		helpBinding("d", "description", width),
		helpBinding("space", "open in Linear", width),
		helpBinding("x", "archive (with confirmation)", width),
		"",
		accentStyle.Bold(true).Render("General"),
		helpBinding("h / ? / esc", "close this help", width),
		helpBinding("q / ctrl+c", "quit", width),
	}
	return left, right
}

func helpBinding(key, action string, width int) string {
	keyWidth := min(18, max(7, width/2))
	actionWidth := max(1, width-keyWidth-1)
	return accentStyle.Width(keyWidth).Render(clip(key, keyWidth)) + " " + clip(action, actionWidth)
}
