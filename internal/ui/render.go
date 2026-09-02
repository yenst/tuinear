package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

const (
	minWidth  = 44
	minHeight = 12
)

func (m Model) render() string {
	width := max(m.width, minWidth)
	height := max(m.height, minHeight)

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	bodyHeight := height - lipgloss.Height(header) - lipgloss.Height(footer)

	var body string
	switch {
	case m.loading:
		body = centeredState(width, bodyHeight, "Loading Linear tickets…", "Fetching your workspace")
	case m.err != nil:
		body = centeredState(width, bodyHeight, "Could not load Linear", m.err.Error()+"  ·  press r to retry")
	case width < 72:
		body = m.renderIssuesPanel(width, bodyHeight)
	case width < 104:
		issuesWidth := width * 48 / 100
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderIssuesPanel(issuesWidth, bodyHeight),
			m.renderDetailsPanel(width-issuesWidth, bodyHeight),
		)
	default:
		teamWidth := clamp(width/6, 20, 27)
		issuesWidth := clamp(width*43/100, 44, 66)
		detailsWidth := width - teamWidth - issuesWidth
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderTeamsPanel(teamWidth, bodyHeight),
			m.renderIssuesPanel(issuesWidth, bodyHeight),
			m.renderDetailsPanel(detailsWidth, bodyHeight),
		)
	}

	return appStyle.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body, footer),
	)
}

func (m Model) renderHeader(width int) string {
	viewer := m.dashboard.Viewer.Label()
	if viewer == "" {
		viewer = "Linear workspace"
	}
	left := brandStyle.Render("TUINEAR") + "  " + mutedStyle.Render("read-only MVP")
	right := fmt.Sprintf("%s  ·  %s  ·  %d tickets", viewer, m.activeTeamName(), len(m.issues))
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-2)
	return headerStyle.Width(width).Render(left + strings.Repeat(" ", space) + right)
}

func (m Model) renderFooter(width int) string {
	help := "j/k move  ·  tab team  ·  r refresh  ·  q quit"
	if width < 72 {
		help = "j/k move  ·  tab team  ·  q quit"
	}
	return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(help)
}

func (m Model) renderTeamsPanel(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	lines := make([]string, 0, len(m.dashboard.Teams)+1)
	lines = append(lines, teamRow("All issues", m.teamIndex == 0, innerWidth))
	for index, team := range m.dashboard.Teams {
		label := team.Key + "  " + team.Name
		lines = append(lines, teamRow(label, m.teamIndex == index+1, innerWidth))
	}
	content := fitLines(lines, innerHeight)
	return panel("Teams", width, height, content)
}

func teamRow(label string, selected bool, width int) string {
	prefix := "  "
	if selected {
		prefix = "› "
		return selectedRowStyle.Width(width).Render(prefix + clip(label, width-2))
	}
	return mutedStyle.Width(width).Render(prefix + clip(label, width-2))
}

func (m Model) renderIssuesPanel(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if len(m.issues) == 0 {
		return panel("Issues", width, height, fitLines([]string{"", mutedStyle.Render("No tickets in this view.")}, innerHeight))
	}

	start := 0
	if m.selected >= innerHeight {
		start = m.selected - innerHeight + 1
	}
	end := min(len(m.issues), start+innerHeight)
	lines := make([]string, 0, innerHeight)
	for index := start; index < end; index++ {
		issue := m.issues[index]
		prefix := "  "
		if index == m.selected {
			prefix = "› "
		}
		identifierWidth := min(11, max(7, innerWidth/4))
		titleWidth := max(8, innerWidth-identifierWidth-5)
		line := fmt.Sprintf("%s%-*s %s %s",
			prefix,
			identifierWidth,
			clip(issue.Identifier, identifierWidth),
			statusGlyph(issue.State.Type),
			clip(issue.Title, titleWidth),
		)
		if index == m.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Issues", width, height, fitLines(lines, innerHeight))
}

func (m Model) renderDetailsPanel(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	issue := m.selectedIssue()
	if issue == nil {
		return panel("Details", width, height, fitLines([]string{"", mutedStyle.Render("Select a ticket to inspect it.")}, innerHeight))
	}

	assignee := "Unassigned"
	if issue.Assignee != nil && issue.Assignee.Label() != "" {
		assignee = issue.Assignee.Label()
	}
	project := "No project"
	if issue.Project != nil && issue.Project.Name != "" {
		project = issue.Project.Name
	}
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}
	if len(labels) == 0 {
		labels = append(labels, "None")
	}

	lines := []string{
		accentStyle.Bold(true).Render(issue.Identifier),
		lipgloss.NewStyle().Bold(true).Width(innerWidth).Render(issue.Title),
		"",
		field("Status", issue.State.Name),
		field("Priority", issue.PriorityLabel()),
		field("Assignee", assignee),
		field("Project", project),
		field("Labels", strings.Join(labels, ", ")),
		field("Updated", relativeTime(issue.UpdatedAt)),
		"",
		mutedStyle.Render("Description"),
		strings.Repeat("─", max(1, innerWidth)),
	}
	description := strings.TrimSpace(issue.Description)
	if description == "" {
		description = "No description provided."
	}
	lines = append(lines, wrapText(description, innerWidth)...)
	return panel("Details", width, height, fitLines(lines, innerHeight))
}

func panel(title string, width, height int, content string) string {
	titleLine := accentStyle.Bold(true).Render(title)
	return panelStyle.
		Width(width).
		Height(height).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Render(titleLine + "\n" + content)
}

func panelInnerSize(width, height int) (int, int) {
	return max(1, width-4), max(1, height-3)
}

func centeredState(width, height int, title, detail string) string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		brandStyle.Render(title),
		mutedStyle.Width(max(20, width-10)).Align(lipgloss.Center).Render(detail),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(theme.background)),
	)
}

func field(label, value string) string {
	return fmt.Sprintf("%-10s %s", mutedStyle.Render(label), value)
}

func statusGlyph(stateType string) string {
	switch strings.ToLower(stateType) {
	case "completed":
		return lipgloss.NewStyle().Foreground(theme.green).Render("●")
	case "started":
		return lipgloss.NewStyle().Foreground(theme.yellow).Render("◐")
	case "canceled", "cancelled":
		return lipgloss.NewStyle().Foreground(theme.red).Render("×")
	case "backlog":
		return mutedStyle.Render("○")
	default:
		return accentStyle.Render("○")
	}
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "Unknown"
	}
	delta := time.Since(value)
	switch {
	case delta < time.Minute:
		return "just now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	case delta < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
	default:
		return value.Format("2 Jan 2006")
	}
}

func wrapText(text string, width int) []string {
	if width <= 1 {
		return []string{clip(text, 1)}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width {
				line += " " + word
			} else {
				lines = append(lines, clip(line, width))
				line = word
			}
		}
		lines = append(lines, clip(line, width))
	}
	return lines
}

func fitLines(lines []string, height int) string {
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func clip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(strings.ReplaceAll(value, "\n", " "))
	if len(runes) <= width {
		return string(runes)
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
