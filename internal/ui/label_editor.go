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

type labelEditor struct {
	issueID    string
	identifier string
	original   linear.Issue
	options    []linear.Label
	selected   int
	checked    map[string]bool
}

func (m *Model) beginLabelEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	options := labelChoices(m.editableLabels(issue), issue.Labels)
	if len(options) == 0 {
		m.editErr = fmt.Errorf("no labels are available for %s", issue.Team.Name)
		return
	}
	checked := make(map[string]bool, len(issue.Labels))
	for _, label := range issue.Labels {
		checked[label.ID] = true
	}
	m.labelEditor = &labelEditor{
		issueID: issue.ID, identifier: issue.Identifier, original: issue,
		options: options, checked: checked,
	}
}

func labelChoices(available, current []linear.Label) []linear.Label {
	seen := make(map[string]bool, len(available)+len(current))
	labels := make([]linear.Label, 0, len(available)+len(current))
	for _, source := range [][]linear.Label{available, current} {
		for _, label := range source {
			if strings.TrimSpace(label.ID) == "" || seen[label.ID] {
				continue
			}
			seen[label.ID] = true
			labels = append(labels, label)
		}
	}
	sort.SliceStable(labels, func(i, j int) bool {
		return strings.ToLower(labels[i].Name) < strings.ToLower(labels[j].Name)
	})
	return labels
}

func (m Model) editableLabels(issue linear.Issue) []linear.Label {
	labels := append([]linear.Label(nil), m.dashboard.LabelsForTeam(issue.Team.ID)...)
	if len(labels) == 0 {
		for _, candidate := range m.dashboard.Issues {
			if candidate.Team.ID == issue.Team.ID {
				labels = append(labels, candidate.Labels...)
			}
		}
	}
	return labels
}

func (m *Model) updateLabelEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.labelEditor
	if editor == nil || len(editor.options) == 0 {
		m.labelEditor = nil
		return nil
	}
	switch msg.String() {
	case "esc":
		m.labelEditor = nil
	case "j", "down", "right", "tab":
		editor.selected = (editor.selected + 1) % len(editor.options)
	case "k", "up", "left", "shift+tab":
		editor.selected = (editor.selected - 1 + len(editor.options)) % len(editor.options)
	case "g", "home":
		editor.selected = 0
	case "G", "end":
		editor.selected = len(editor.options) - 1
	case "space", " ":
		id := editor.options[editor.selected].ID
		editor.checked[id] = !editor.checked[id]
	case "enter", "return":
		return m.submitLabelEdit()
	}
	return nil
}

func (m *Model) submitLabelEdit() tea.Cmd {
	if m.labelEditor == nil || m.issueUpdater == nil {
		return nil
	}
	editor := m.labelEditor
	labels := make([]linear.Label, 0, len(editor.options))
	ids := make([]string, 0, len(editor.options))
	for _, label := range editor.options {
		if editor.checked[label.ID] {
			labels = append(labels, label)
			ids = append(ids, label.ID)
		}
	}
	if sameLabelSet(labels, editor.original.Labels) {
		m.labelEditor = nil
		return nil
	}
	before := editor.original
	optimistic := before
	optimistic.Labels = append([]linear.Label(nil), labels...)
	optimistic.UpdatedAt = time.Now()
	m.pendingEdit = &pendingIssueEdit{
		issueID: before.ID, identifier: before.Identifier, before: before,
		optimistic: optimistic, kind: editLabels,
	}
	m.labelEditor = nil
	m.editErr = nil
	m.replaceIssue(optimistic)
	return updateIssueLabels(m.issueUpdater, before.ID, ids)
}

func sameLabelSet(first, second []linear.Label) bool {
	if len(first) != len(second) {
		return false
	}
	ids := make(map[string]bool, len(first))
	for _, label := range first {
		ids[label.ID] = true
	}
	for _, label := range second {
		if !ids[label.ID] {
			return false
		}
	}
	return true
}

func updateIssueLabels(updater IssueUpdater, issueID string, labelIDs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := updater.UpdateIssue(ctx, issueID, linear.IssueUpdate{LabelIDs: &labelIDs})
		if err != nil {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save labels: %w", err)}
		}
		if issue.ID != issueID {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save labels: Linear returned a different issue")}
		}
		return issueUpdatedMsg{issue: issue}
	}
}

func (m *Model) rebaseLabelEditor() {
	editor := m.labelEditor
	if editor == nil {
		return
	}
	issue, ok := m.dashboardIssue(editor.issueID)
	if !ok {
		m.labelEditor = nil
		m.editErr = fmt.Errorf("the issue being edited is no longer available")
		return
	}
	selectedID := ""
	if editor.selected >= 0 && editor.selected < len(editor.options) {
		selectedID = editor.options[editor.selected].ID
	}
	preserved := make([]linear.Label, 0, len(editor.options))
	for _, label := range editor.options {
		if editor.checked[label.ID] {
			preserved = append(preserved, label)
		}
	}
	editor.original = issue
	editor.options = labelChoices(append(m.editableLabels(issue), preserved...), issue.Labels)
	editor.selected = 0
	for index, label := range editor.options {
		if label.ID == selectedID {
			editor.selected = index
			break
		}
	}
}

func (m Model) renderLabelEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.labelEditor == nil {
		return panel("Edit labels", width, height, fitLines(nil, innerHeight))
	}
	editor := m.labelEditor
	lines := []string{
		accentStyle.Bold(true).Render(editor.identifier),
		mutedStyle.Render("Space toggles labels; Enter applies the selection."),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := 0
	if editor.selected >= capacity {
		start = editor.selected - capacity + 1
	}
	end := min(len(editor.options), start+capacity)
	for index := start; index < end; index++ {
		label := editor.options[index]
		marker := "[ ]"
		if editor.checked[label.ID] {
			marker = "[x]"
		}
		prefix := "  "
		if index == editor.selected {
			prefix = "› "
		}
		line := prefix + marker + " " + clip(label.Name, max(1, innerWidth-lipgloss.Width(marker)-4))
		if index == editor.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Edit labels", width, height, fitLines(lines, innerHeight))
}
