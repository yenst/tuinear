package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

type statusEditor struct {
	issueID    string
	identifier string
	original   linear.Issue
	options    []linear.WorkflowState
	selected   int
}

func (m *Model) beginStatusEdit() {
	if m.pendingEdit != nil {
		m.editErr = fmt.Errorf("wait for %s to finish saving", m.pendingEdit.identifier)
		return
	}
	issue := m.selectedIssue()
	if issue == nil {
		return
	}
	if m.issueUpdater == nil {
		m.editErr = fmt.Errorf("issue editing is not available for this data source")
		return
	}
	options := m.editableStates(*issue)
	if len(options) == 0 {
		m.editErr = fmt.Errorf("no editable statuses are available for %s", issue.Team.Name)
		return
	}
	selected := 0
	for index, state := range options {
		if state.ID == issue.State.ID || (issue.State.ID == "" && state.Name == issue.State.Name) {
			selected = index
			break
		}
	}
	m.statusEditor = &statusEditor{
		issueID: issue.ID, identifier: issue.Identifier, original: *issue,
		options: options, selected: selected,
	}
	m.editErr = nil
	m.browserErr = nil
}

func (m Model) editableStates(issue linear.Issue) []linear.WorkflowState {
	states := m.dashboard.StatesForTeam(issue.Team.ID)
	fromDashboard := len(states) > 0
	if !fromDashboard {
		for _, candidate := range m.dashboard.Issues {
			if candidate.Team.ID == issue.Team.ID {
				states = append(states, candidate.State)
			}
		}
	}
	seen := make(map[string]bool, len(states))
	options := make([]linear.WorkflowState, 0, len(states))
	for _, state := range states {
		if strings.TrimSpace(state.ID) == "" || seen[state.ID] {
			continue
		}
		seen[state.ID] = true
		options = append(options, state)
	}
	if strings.TrimSpace(issue.State.ID) != "" && !seen[issue.State.ID] {
		options = append(options, issue.State)
	}
	if !fromDashboard {
		sort.SliceStable(options, func(i, j int) bool { return options[i].Name < options[j].Name })
	}
	return options
}

func (m *Model) updateStatusEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.statusEditor
	if editor == nil || len(editor.options) == 0 {
		m.statusEditor = nil
		return nil
	}
	switch msg.String() {
	case "esc":
		m.statusEditor = nil
	case "j", "down", "right", "tab":
		editor.selected = (editor.selected + 1) % len(editor.options)
	case "k", "up", "left", "shift+tab":
		editor.selected = (editor.selected - 1 + len(editor.options)) % len(editor.options)
	case "g", "home":
		editor.selected = 0
	case "G", "end":
		editor.selected = len(editor.options) - 1
	case "enter", "return":
		return m.submitStatusEdit()
	}
	return nil
}

func (m *Model) submitStatusEdit() tea.Cmd {
	if m.statusEditor == nil || m.issueUpdater == nil || len(m.statusEditor.options) == 0 {
		return nil
	}
	editor := m.statusEditor
	state := editor.options[editor.selected]
	if state.ID == editor.original.State.ID || (editor.original.State.ID == "" && state.Name == editor.original.State.Name) {
		m.statusEditor = nil
		return nil
	}
	before := editor.original
	optimistic := before
	optimistic.State = state
	optimistic.UpdatedAt = time.Now()
	m.pendingEdit = &pendingIssueEdit{
		issueID: before.ID, identifier: before.Identifier, before: before,
		optimistic: optimistic, kind: editStatus,
	}
	m.statusEditor = nil
	m.editErr = nil
	m.replaceIssue(optimistic)
	return updateIssueStatus(m.issueUpdater, before.ID, state.ID)
}

func updateIssueStatus(updater IssueUpdater, issueID, stateID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := updater.UpdateIssue(ctx, issueID, linear.IssueUpdate{StateID: &stateID})
		if err != nil {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save status: %w", err)}
		}
		if issue.ID != issueID {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save status: Linear returned a different issue")}
		}
		return issueUpdatedMsg{issue: issue}
	}
}

func (m Model) renderStatusEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.statusEditor == nil {
		return panel("Edit status", width, height, fitLines(nil, innerHeight))
	}
	editor := m.statusEditor
	lines := []string{
		accentStyle.Bold(true).Render(editor.identifier),
		mutedStyle.Render("Choose a team status, then press Enter."),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := 0
	if editor.selected >= capacity {
		start = editor.selected - capacity + 1
	}
	end := min(len(editor.options), start+capacity)
	for index := start; index < end; index++ {
		state := editor.options[index]
		prefix := "  "
		if index == editor.selected {
			prefix = "› "
		}
		line := prefix + statusGlyph(state.Type) + " " + clip(state.Name, max(1, innerWidth-4))
		if index == editor.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Edit status", width, height, fitLines(lines, innerHeight))
}
