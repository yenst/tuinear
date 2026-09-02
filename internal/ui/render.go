package ui

import (
	"fmt"
	"strings"
	"time"

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
	case m.editor != nil:
		body = m.renderTitleEditor(width, bodyHeight)
	case m.choiceEditor != nil:
		body = m.renderChoiceEditor(width, bodyHeight)
	case m.labelEditor != nil:
		body = m.renderLabelEditor(width, bodyHeight)
	case m.descriptionEditor != nil:
		body = m.renderDescriptionEditor(width, bodyHeight)
	case m.archiveConfirm != nil:
		body = m.renderArchiveConfirmation(width, bodyHeight)
	case m.actionMenu != nil:
		body = m.renderActionMenu(width, bodyHeight)
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
	} else if m.pendingArchive != nil {
		mode = lipgloss.NewStyle().Foreground(theme.yellow).Render("archiving " + m.pendingArchive.identifier + "…")
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
	if m.choiceEditor != nil {
		help := "enter save  ·  esc cancel  ·  j/k or arrows choose " + m.choiceEditor.field
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.labelEditor != nil {
		help := "space toggle  ·  enter apply  ·  esc cancel  ·  j/k or arrows choose label"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.descriptionEditor != nil {
		help := "ctrl+s save  ·  esc cancel  ·  enter newline  ·  arrows move  ·  ctrl+u clear"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.archiveConfirm != nil {
		help := "enter choose  ·  esc cancel  ·  arrows select  ·  cancellation is selected by default"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.actionMenu != nil {
		help := "enter choose  ·  esc cancel  ·  j/k or arrows move  ·  e/s/p/u/P/l/d/space/x quick action"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.editErr != nil {
		help := "Action: " + m.editErr.Error() + "  ·  retry with e/s/p/u/P/l/d/x"
		return lipgloss.NewStyle().Foreground(theme.red).Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	if m.browserErr != nil {
		help := "Browser: " + m.browserErr.Error() + "  ·  space retries"
		return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
	}
	help := "enter actions  ·  e title  ·  s status  ·  p priority  ·  u assignee  ·  P project  ·  l labels  ·  d description  ·  x archive  ·  space open  ·  / search  ·  f filters  ·  j/k move  ·  tab team  ·  a account  ·  r refresh  ·  q quit"
	if width < 96 {
		help = "enter actions  ·  e title  ·  s status  ·  p priority  ·  u assignee  ·  P project  ·  l labels  ·  d description  ·  x archive  ·  space open  ·  / search  ·  f filters  ·  q quit"
	}
	if width < 72 {
		help = "enter actions  ·  e/s/p/u/P/l/d edit  ·  x archive  ·  space open  ·  / search  ·  f filters  ·  q quit"
	}
	return mutedStyle.Background(theme.background).Width(width).Padding(0, 1).Render(clip(help, max(1, width-2)))
}
