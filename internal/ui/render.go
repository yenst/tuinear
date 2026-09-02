package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/jihmy/tuinear/internal/linear"
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
	case m.editor != nil:
		body = m.renderTitleEditor(width, bodyHeight)
	case m.statusEditor != nil:
		body = m.renderStatusEditor(width, bodyHeight)
	case m.palette:
		body = m.renderFilterPalette(width, bodyHeight)
	case width < 72:
		body = m.renderIssuesPanel(width, bodyHeight)
	default:
		teamWidth := clamp(width/5, 20, 27)
		if len(m.dashboard.Accounts) > 0 && width >= 96 {
			teamWidth = max(teamWidth, 24)
		}
		issuesWidth := clamp(width*38/100, 27, 66)
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
		viewer = "Unknown user"
	}
	account := viewer
	if workspace := m.dashboard.Organization.Name; workspace != "" {
		account = workspace + " / " + viewer
	}
	mode := mutedStyle.Render("browse + edit")
	if m.pendingEdit != nil {
		mode = lipgloss.NewStyle().Foreground(theme.yellow).Render("saving " + m.pendingEdit.identifier + "…")
	} else if m.fromCache {
		age := cacheAge(m.cachedAt)
		if m.refreshErr != nil {
			mode = lipgloss.NewStyle().Foreground(theme.yellow).Render("offline · cached " + age)
		} else {
			mode = lipgloss.NewStyle().Foreground(theme.yellow).Render("cached " + age + " · refreshing")
		}
	} else if m.refreshErr != nil {
		mode = lipgloss.NewStyle().Foreground(theme.yellow).Render("refresh failed")
	} else if m.refreshing {
		mode = mutedStyle.Render("refreshing…")
	}
	left := brandStyle.Render("TUINEAR") + "  " + mode
	right := fmt.Sprintf("%s  ·  %s  ·  %d tickets", account, m.activeTeamName(), len(m.issues))
	right = clip(right, max(8, width-lipgloss.Width(left)-3))
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-2)
	base := headerStyle.Width(width).Render(left + strings.Repeat(" ", space) + right)
	return lipgloss.JoinVertical(lipgloss.Left, base, m.renderSearchBar(width))
}

func cacheAge(value time.Time) string {
	if value.IsZero() {
		return "earlier"
	}
	delta := time.Since(value)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return "now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh", int(delta.Hours()))
	default:
		return fmt.Sprintf("%dd", int(delta.Hours()/24))
	}
}

func (m Model) renderSearchBar(width int) string {
	search := "Search: —"
	if m.query != "" {
		search = "Search: " + m.query
	} else if m.searching {
		search = "Search: _"
	} else {
		search = "Search: press /"
	}
	filters := "Filters: —"
	active := m.activeFilters()
	if len(active) > 0 {
		filters = "Filters: " + strings.Join(active, "  ")
	}
	label := search + "  ·  " + filters
	if m.palette {
		label += "  ·  Filter palette"
	}
	return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(label, max(1, width-2)))
}

func (m Model) activeFilters() []string {
	active := make([]string, 0, 4)
	if m.filters.Assignee != "" {
		active = append(active, "assignee="+m.filterDisplayValue(filterAssignee, m.filters.Assignee))
	}
	if m.filters.Status != "" {
		active = append(active, "status="+m.filterDisplayValue(filterStatus, m.filters.Status))
	}
	if m.filters.Priority != "" {
		active = append(active, "priority="+m.filterDisplayValue(filterPriority, m.filters.Priority))
	}
	if m.filters.Project != "" {
		active = append(active, "project="+m.filterDisplayValue(filterProject, m.filters.Project))
	}
	return active
}

func (m Model) filterDisplayValue(field filterField, value string) string {
	if value == "__unassigned__" {
		return "Unassigned"
	}
	for _, option := range m.valuesFor(field) {
		if option.value == value {
			return strings.TrimPrefix(option.label, filterFieldName(field)+": ")
		}
	}
	return value
}

func (m Model) renderFilterPalette(width, height int) string {
	innerHeight := max(1, height-3)
	options := m.filterOptions()
	lines := []string{"Filter palette", mutedStyle.Render("Choose a value; filters compose. esc closes."), ""}
	capacity := max(1, innerHeight-len(lines))
	start := 0
	if m.paletteIdx >= capacity {
		start = m.paletteIdx - capacity + 1
	}
	end := min(len(options), start+capacity)
	for index := start; index < end; index++ {
		option := options[index]
		prefix := "  "
		if index == m.paletteIdx {
			prefix = "› "
		}
		line := prefix + option.label
		if index == m.paletteIdx {
			line = selectedRowStyle.Width(max(1, width-4)).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(max(1, width-4)).Render(line)
		}
		lines = append(lines, line)
	}
	if len(options) == 0 {
		lines = append(lines, mutedStyle.Render("No filter values available."))
	}
	return panel("Filters", width, height, fitLines(lines, innerHeight))
}

func (m Model) renderFooter(width int) string {
	if m.editor != nil {
		help := "enter save  ·  esc cancel  ·  ctrl+u clear  ·  ←/→ move"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.statusEditor != nil {
		help := "enter save  ·  esc cancel  ·  j/k or arrows choose status"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.editErr != nil {
		help := "Edit: " + m.editErr.Error() + "  ·  e retries"
		return lipgloss.NewStyle().Foreground(theme.red).Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.browserErr != nil {
		help := "Browser: " + m.browserErr.Error() + "  ·  space retries"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	help := "e title  ·  s status  ·  space open  ·  / search  ·  f filters  ·  j/k move  ·  tab team  ·  a account  ·  r refresh  ·  q quit"
	if width < 96 {
		help = "e title  ·  s status  ·  space open  ·  / search  ·  f filters  ·  j/k move  ·  a account  ·  q quit"
	}
	if width < 72 {
		help = "e title  ·  s status  ·  space open  ·  / search  ·  f filters  ·  q quit"
	}
	return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
}

func (m Model) renderTeamsPanel(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	teamLines := make([]string, 0, len(m.dashboard.Teams)+1)
	teamLines = append(teamLines, teamRow("All issues", m.teamIndex == 0, innerWidth))
	for index, team := range m.dashboard.Teams {
		label := team.Key + "  " + team.Name
		teamLines = append(teamLines, teamRow(label, m.teamIndex == index+1, innerWidth))
	}
	lines := teamLines
	if len(m.dashboard.Accounts) > 0 {
		accountSpace := min(len(m.dashboard.Accounts)+2, max(3, innerHeight/2))
		teamSpace := max(1, innerHeight-accountSpace)
		lines = visibleRows(teamLines, m.teamIndex, teamSpace)
		lines = append(lines, "", mutedStyle.Bold(true).Render("Accounts"))
		for _, account := range visibleAccounts(m.dashboard.Accounts, m.dashboard.ActiveAccountID, innerHeight-len(lines)) {
			lines = append(lines, accountRow(account, account.ID == m.dashboard.ActiveAccountID, innerWidth))
		}
	}
	content := fitLines(lines, innerHeight)
	return panel("Teams", width, height, content)
}

func visibleRows(rows []string, selected, limit int) []string {
	if limit <= 0 || len(rows) == 0 {
		return nil
	}
	if len(rows) <= limit {
		return rows
	}
	start := 0
	if selected >= limit {
		start = selected - limit + 1
	}
	end := min(len(rows), start+limit)
	return rows[start:end]
}

func visibleAccounts(accounts []linear.Account, activeID string, limit int) []linear.Account {
	if limit <= 0 || len(accounts) == 0 {
		return nil
	}
	if len(accounts) <= limit {
		return accounts
	}
	active := 0
	for index, account := range accounts {
		if account.ID == activeID {
			active = index
			break
		}
	}
	start := 0
	if active >= limit {
		start = active - limit + 1
	}
	return accounts[start:min(len(accounts), start+limit)]
}

func accountRow(account linear.Account, active bool, width int) string {
	label := clip(account.Label(), max(1, width-2))
	if active {
		return activeAccountStyle.Width(width).Render("● " + label)
	}
	return mutedStyle.Width(width).Render("○ " + label)
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
		lines := []string{"", mutedStyle.Render("No tickets match this view.")}
		if m.query != "" || m.hasFilters() {
			lines = append(lines, mutedStyle.Render("Esc clears search/filters; f opens filters."))
		} else {
			lines = append(lines, mutedStyle.Render("Try / to search or f to filter."))
		}
		return panel("Issues", width, height, fitLines(lines, innerHeight))
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
		lipgloss.NewStyle().Bold(true).Render(clip(issue.Title, innerWidth)),
		"",
		field("Status", issue.State.Name, innerWidth),
		field("Priority", issue.PriorityLabel(), innerWidth),
		field("Assignee", assignee, innerWidth),
		field("Project", project, innerWidth),
		field("Labels", strings.Join(labels, ", "), innerWidth),
		field("Updated", relativeTime(issue.UpdatedAt), innerWidth),
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

func field(label, value string, width int) string {
	labelWidth := min(10, max(1, width-1))
	valueWidth := max(0, width-labelWidth-1)
	return mutedStyle.Width(labelWidth).Render(clip(label, labelWidth)) + " " + clip(value, valueWidth)
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
