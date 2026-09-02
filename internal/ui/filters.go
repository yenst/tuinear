package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

// IssueFilters is the set of composable local filters. Empty values mean any.
// Assignee and Project accept either their stable ID or their displayed name.
type IssueFilters struct {
	Assignee string
	Status   string
	Priority string
	Project  string
}

type filterField uint8

const (
	filterAssignee filterField = iota
	filterStatus
	filterPriority
	filterProject
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
		if filters.Assignee != "" {
			if issue.Assignee == nil {
				if !matchesValue(filters.Assignee, "__unassigned__", "Unassigned") {
					continue
				}
			} else if !matchesValue(filters.Assignee, issue.Assignee.ID, issue.Assignee.Label()) {
				continue
			}
		}
		if filters.Status != "" && !matchesValue(filters.Status, issue.State.Type, issue.State.Name) {
			continue
		}
		if filters.Priority != "" && !matchesValue(filters.Priority, fmt.Sprintf("%d", issue.Priority), issue.PriorityLabel()) {
			continue
		}
		if filters.Project != "" {
			if issue.Project == nil || !matchesValue(filters.Project, issue.Project.ID, issue.Project.Name) {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	return filtered
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
	return m.filters != (IssueFilters{})
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

func (m *Model) updatePalette(msg tea.KeyPressMsg) {
	options := m.filterOptions()
	if len(options) == 0 {
		m.palette = false
		return
	}
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
	case "enter", "return":
		option := options[m.paletteIdx]
		if option.field == filterClear {
			m.filters = IssueFilters{}
		} else {
			switch option.field {
			case filterAssignee:
				m.filters.Assignee = option.value
			case filterStatus:
				m.filters.Status = option.value
			case filterPriority:
				m.filters.Priority = option.value
			case filterProject:
				m.filters.Project = option.value
			}
		}
		m.filterIssues()
		m.palette = false
	}
}

func (m Model) filterOptions() []filterOption {
	options := []filterOption{{label: "Assignee: Any", field: filterAssignee}}
	options = append(options, m.valuesFor(filterAssignee)...)
	options = append(options, filterOption{label: "Status: Any", field: filterStatus})
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
	seen := map[string]bool{}
	values := make([]value, 0)
	add := func(label, val string) {
		if label == "" || seen[strings.ToLower(label)] {
			return
		}
		seen[strings.ToLower(label)] = true
		values = append(values, value{label: label, value: val})
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
			}
		}
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].label < values[j].label })
	options := make([]filterOption, 0, len(values))
	for _, item := range values {
		options = append(options, filterOption{label: filterFieldName(field) + ": " + item.label, value: item.value, field: field})
	}
	return options
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
