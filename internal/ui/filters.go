package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/issuefilter"
	"github.com/yenst/tuinear/internal/linear"
)

// IssueFilters is kept as a UI-facing alias while its serializable state lives
// in a package shared with the persistent cache.
type IssueFilters = issuefilter.State

type filterField uint8

const (
	filterAssignee filterField = iota
	filterStatus
	filterPriority
	filterProject
	filterActive
	filterClear
)

type filterOption struct {
	label string
	field filterField
	value string
}

func (m *Model) filterIssues() {
	teamIssues := m.dashboard.Issues
	if m.teamIndex != 0 {
		teamID := m.dashboard.Teams[m.teamIndex-1].ID
		teamIssues = nil
		for _, issue := range m.dashboard.Issues {
			if issue.Team.ID == teamID {
				teamIssues = append(teamIssues, issue)
			}
		}
	}
	m.issues = FilterIssues(teamIssues, m.filters, m.query)
	if len(m.issues) == 0 {
		m.selected = 0
	} else if m.selected >= len(m.issues) {
		m.selected = len(m.issues) - 1
	}
}

// FilterIssues applies the local search and all active filters. It does not
// mutate its input and is intentionally independent of the TUI model.
func FilterIssues(issues []linear.Issue, filters IssueFilters, query string) []linear.Issue {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]linear.Issue, 0, len(issues))
	for _, issue := range issues {
		if query != "" && !strings.Contains(strings.ToLower(issue.Identifier), query) &&
			!strings.Contains(strings.ToLower(issue.Title), query) {
			continue
		}
		assigneeValues := []string{"__unassigned__", "Unassigned"}
		if issue.Assignee != nil {
			assigneeValues = []string{issue.Assignee.ID, issue.Assignee.Label()}
		}
		if matchesAny(filters.ExcludedAssignees, assigneeValues...) {
			continue
		}
		if filters.Assignee != "" {
			if issue.Assignee == nil {
				if !matchesValue(filters.Assignee, "__unassigned__", "Unassigned") {
					continue
				}
			} else if !matchesValue(filters.Assignee, issue.Assignee.ID, issue.Assignee.Label()) {
				continue
			}
		}
		if matchesAny(filters.ExcludedStatuses, issue.State.Type, issue.State.Name) {
			continue
		}
		if filters.Status != "" && !matchesValue(filters.Status, issue.State.Type, issue.State.Name) {
			continue
		}
		if matchesAny(filters.ExcludedPriorities, fmt.Sprintf("%d", issue.Priority), issue.PriorityLabel()) {
			continue
		}
		if filters.Priority != "" && !matchesValue(filters.Priority, fmt.Sprintf("%d", issue.Priority), issue.PriorityLabel()) {
			continue
		}
		projectValues := []string{"__no_project__", "No project"}
		if issue.Project != nil {
			projectValues = []string{issue.Project.ID, issue.Project.Name}
		}
		if matchesAny(filters.ExcludedProjects, projectValues...) {
			continue
		}
		if filters.Project != "" {
			if issue.Project == nil {
				if !matchesValue(filters.Project, "__no_project__", "No project") {
					continue
				}
			} else if !matchesValue(filters.Project, issue.Project.ID, issue.Project.Name) {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func matchesAny(wants []string, values ...string) bool {
	for _, want := range wants {
		if matchesValue(want, values...) {
			return true
		}
	}
	return false
}

func matchesValue(want string, values ...string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

func (m Model) hasFilters() bool {
	return !m.filters.Empty()
}

func (m *Model) updateSearch(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc":
		if m.query != "" {
			m.query = ""
			m.filterIssues()
		} else {
			m.searching = false
		}
	case "enter", "return":
		m.searching = false
	case "backspace", "ctrl+h":
		if m.query != "" {
			_, size := utf8.DecodeLastRuneInString(m.query)
			m.query = m.query[:len(m.query)-size]
			m.filterIssues()
		}
	default:
		text := msg.Text
		if text == "" && msg.Code >= 32 && msg.Code != utf8.RuneError {
			text = string(msg.Code)
		}
		if text != "" && (msg.Mod == 0 || msg.Mod == tea.ModShift) {
			m.query += text
			m.filterIssues()
		}
	}
}

func (m *Model) updatePalette(msg tea.KeyPressMsg) tea.Cmd {
	options := m.filterOptions()
	if len(options) == 0 {
		m.palette = false
		return nil
	}
	// A background refresh can remove options while the palette is open.
	m.paletteIdx = clamp(m.paletteIdx, 0, len(options)-1)
	switch msg.String() {
	case "esc":
		m.palette = false
	case "j", "down":
		m.paletteIdx = (m.paletteIdx + 1) % len(options)
	case "k", "up":
		m.paletteIdx = (m.paletteIdx - 1 + len(options)) % len(options)
	case "g", "home":
		m.paletteIdx = 0
	case "G", "end":
		m.paletteIdx = len(options) - 1
	case "!":
		option := options[m.paletteIdx]
		if option.field == filterActive {
			m.applyActiveFilterPreset()
			m.filterIssues()
			return m.saveIssueFilters()
		}
		if option.field == filterClear || option.value == "" {
			return nil
		}
		m.toggleExcludedFilter(option.field, option.value)
		m.filterIssues()
		return m.saveIssueFilters()
	case "enter", "return":
		option := options[m.paletteIdx]
		if option.field == filterActive {
			m.applyActiveFilterPreset()
		} else if option.field == filterClear {
			m.filters = IssueFilters{}
		} else {
			m.includeFilter(option.field, option.value)
		}
		m.filterIssues()
		m.palette = false
		return m.saveIssueFilters()
	}
	return nil
}

func (m Model) filterOptions() []filterOption {
	options := []filterOption{{label: "Assignee: Any", field: filterAssignee}}
	options = append(options, m.valuesFor(filterAssignee)...)
	options = append(options,
		filterOption{label: "Status: Any", field: filterStatus},
		filterOption{label: "Status: Active (NOT completed/canceled)", field: filterActive},
	)
	options = append(options, m.valuesFor(filterStatus)...)
	options = append(options, filterOption{label: "Priority: Any", field: filterPriority})
	options = append(options, m.valuesFor(filterPriority)...)
	options = append(options, filterOption{label: "Project: Any", field: filterProject})
	options = append(options, m.valuesFor(filterProject)...)
	if m.hasFilters() {
		options = append(options, filterOption{label: "Clear all filters", field: filterClear})
	}
	return options
}

func (m Model) valuesFor(field filterField) []filterOption {
	type value struct{ label, value string }
	seenLabels := map[string]bool{}
	seenValues := map[string]bool{}
	values := make([]value, 0)
	add := func(label, val string) {
		labelKey := strings.ToLower(strings.TrimSpace(label))
		valueKey := strings.ToLower(strings.TrimSpace(val))
		if labelKey == "" || seenLabels[labelKey] || (valueKey != "" && seenValues[valueKey]) {
			return
		}
		seenLabels[labelKey] = true
		seenValues[valueKey] = true
		values = append(values, value{label: label, value: val})
	}
	if field == filterAssignee && m.dashboard.Viewer.ID != "" {
		label := "Me"
		if viewer := m.dashboard.Viewer.Label(); viewer != "" {
			label += " (" + viewer + ")"
		}
		add(label, m.dashboard.Viewer.ID)
	}
	for _, issue := range m.dashboard.Issues {
		switch field {
		case filterAssignee:
			if issue.Assignee == nil {
				add("Unassigned", "__unassigned__")
			} else {
				add(issue.Assignee.Label(), issue.Assignee.ID)
			}
		case filterStatus:
			add(issue.State.Name, issue.State.Name)
		case filterPriority:
			add(issue.PriorityLabel(), fmt.Sprintf("%d", issue.Priority))
		case filterProject:
			if issue.Project != nil {
				add(issue.Project.Name, issue.Project.ID)
			} else {
				add("No project", "__no_project__")
			}
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].label < values[j].label })
	options := make([]filterOption, 0, len(values))
	for _, item := range values {
		label := item.label
		if m.isExcludedFilter(field, item.value) {
			label = "NOT " + label
		}
		options = append(options, filterOption{label: filterFieldName(field) + ": " + label, value: item.value, field: field})
	}
	return options
}

func (m *Model) includeFilter(field filterField, value string) {
	switch field {
	case filterAssignee:
		m.filters.Assignee = value
		m.filters.ExcludedAssignees = nil
	case filterStatus:
		m.filters.Status = value
		m.filters.ExcludedStatuses = nil
	case filterPriority:
		m.filters.Priority = value
		m.filters.ExcludedPriorities = nil
	case filterProject:
		m.filters.Project = value
		m.filters.ExcludedProjects = nil
	}
}

func (m *Model) toggleExcludedFilter(field filterField, value string) {
	var values *[]string
	switch field {
	case filterAssignee:
		m.filters.Assignee = ""
		values = &m.filters.ExcludedAssignees
	case filterStatus:
		m.filters.Status = ""
		values = &m.filters.ExcludedStatuses
	case filterPriority:
		m.filters.Priority = ""
		values = &m.filters.ExcludedPriorities
	case filterProject:
		m.filters.Project = ""
		values = &m.filters.ExcludedProjects
	default:
		return
	}
	for index, existing := range *values {
		if matchesValue(existing, value) {
			*values = append((*values)[:index], (*values)[index+1:]...)
			return
		}
	}
	*values = append(*values, value)
}

func (m Model) isExcludedFilter(field filterField, value string) bool {
	switch field {
	case filterAssignee:
		return matchesAny(m.filters.ExcludedAssignees, value)
	case filterStatus:
		return matchesAny(m.filters.ExcludedStatuses, value)
	case filterPriority:
		return matchesAny(m.filters.ExcludedPriorities, value)
	case filterProject:
		return matchesAny(m.filters.ExcludedProjects, value)
	default:
		return false
	}
}

func (m *Model) applyActiveFilterPreset() {
	m.filters.Status = ""
	m.filters.ExcludedStatuses = []string{"completed", "canceled"}
}

func filterFieldName(field filterField) string {
	switch field {
	case filterAssignee:
		return "Assignee"
	case filterStatus:
		return "Status"
	case filterPriority:
		return "Priority"
	case filterProject:
		return "Project"
	default:
		return "Filter"
	}
}
