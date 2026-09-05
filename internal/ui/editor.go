package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yenst/tuinear/internal/linear"
)

type titleEditor struct {
	issueID    string
	identifier string
	original   linear.Issue
	value      []rune
	cursor     int
	err        error
}

type issueEditKind uint8

const (
	editTitle issueEditKind = iota
	editStatus
	editPriority
	editAssignee
	editProject
	editLabels
	editDescription
)

type pendingIssueEdit struct {
	issueID    string
	identifier string
	before     linear.Issue
	optimistic linear.Issue
	kind       issueEditKind
}

type issueUpdatedMsg struct{ issue linear.Issue }
type issueUpdateFailedMsg struct {
	issueID string
	err     error
}

func (m *Model) beginTitleEdit() {
	if m.pendingEdit != nil || m.pendingArchive != nil {
		if m.pendingArchive != nil {
			m.editErr = fmt.Errorf("wait for %s to finish archiving", m.pendingArchive.identifier)
			return
		}
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
	value := []rune(issue.Title)
	m.editor = &titleEditor{
		issueID: issue.ID, identifier: issue.Identifier, original: *issue,
		value: value, cursor: len(value),
	}
	m.editErr = nil
	m.browserErr = nil
}

func (m *Model) updateTitleEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.editor
	if editor == nil {
		return nil
	}
	switch msg.String() {
	case "esc":
		m.editor = nil
	case "enter", "return":
		return m.submitTitleEdit()
	case "left", "ctrl+b":
		editor.cursor = max(0, editor.cursor-1)
	case "right", "ctrl+f":
		editor.cursor = min(len(editor.value), editor.cursor+1)
	case "home", "ctrl+a":
		editor.cursor = 0
	case "end", "ctrl+e":
		editor.cursor = len(editor.value)
	case "backspace", "ctrl+h":
		if editor.cursor > 0 {
			editor.value = append(editor.value[:editor.cursor-1], editor.value[editor.cursor:]...)
			editor.cursor--
			editor.err = nil
		}
	case "delete":
		if editor.cursor < len(editor.value) {
			editor.value = append(editor.value[:editor.cursor], editor.value[editor.cursor+1:]...)
			editor.err = nil
		}
	case "ctrl+u":
		editor.value = nil
		editor.cursor = 0
		editor.err = nil
	default:
		text := msg.Text
		if text == "" && msg.Code >= 32 && msg.Code != utf8.RuneError {
			text = string(msg.Code)
		}
		if text != "" && (msg.Mod == 0 || msg.Mod == tea.ModShift) {
			insert := []rune(strings.ReplaceAll(text, "\n", " "))
			editor.value = append(editor.value, make([]rune, len(insert))...)
			copy(editor.value[editor.cursor+len(insert):], editor.value[editor.cursor:])
			copy(editor.value[editor.cursor:], insert)
			editor.cursor += len(insert)
			editor.err = nil
		}
	}
	return nil
}

func (m *Model) submitTitleEdit() tea.Cmd {
	if m.editor == nil || m.issueUpdater == nil {
		return nil
	}
	title := strings.TrimSpace(string(m.editor.value))
	if title == "" {
		m.editor.err = fmt.Errorf("title cannot be empty")
		return nil
	}
	if title == m.editor.original.Title {
		m.editor = nil
		return nil
	}
	before := m.editor.original
	optimistic := before
	optimistic.Title = title
	optimistic.UpdatedAt = time.Now()
	m.pendingEdit = &pendingIssueEdit{
		issueID: before.ID, identifier: before.Identifier, before: before,
		optimistic: optimistic, kind: editTitle,
	}
	m.editor = nil
	m.editErr = nil
	m.replaceIssue(optimistic)
	return updateIssueTitle(m.issueUpdater, before.ID, title)
}

func updateIssueTitle(updater IssueUpdater, issueID, title string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := updater.UpdateIssue(ctx, issueID, linear.IssueUpdate{Title: &title})
		if err != nil {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save title: %w", err)}
		}
		if issue.ID != issueID {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save title: Linear returned a different issue")}
		}
		return issueUpdatedMsg{issue: issue}
	}
}

func (m *Model) finishIssueEdit(issue linear.Issue) {
	if m.pendingEdit == nil || m.pendingEdit.issueID != issue.ID {
		return
	}
	m.replaceIssue(issue)
	m.pendingEdit = nil
	m.editErr = nil
}

func (m *Model) rollbackIssueEdit(issueID string, err error) {
	if m.pendingEdit == nil || m.pendingEdit.issueID != issueID {
		return
	}
	m.replaceIssue(m.pendingEdit.before)
	m.pendingEdit = nil
	m.editErr = err
}

// rebasePendingIssueEdit keeps an optimistic field visible if a cached-startup
// refresh completes while the mutation is still running. The fresh server
// issue becomes the rollback baseline, so a later mutation failure cannot
// restore stale cached fields.
func (m *Model) rebasePendingIssueEdit() {
	if m.pendingEdit == nil {
		return
	}
	for _, issue := range m.dashboard.Issues {
		if issue.ID == m.pendingEdit.issueID {
			m.pendingEdit.before = issue
			switch m.pendingEdit.kind {
			case editTitle:
				issue.Title = m.pendingEdit.optimistic.Title
			case editStatus:
				issue.State = m.pendingEdit.optimistic.State
			case editPriority:
				issue.Priority = m.pendingEdit.optimistic.Priority
			case editAssignee:
				issue.Assignee = m.pendingEdit.optimistic.Assignee
			case editProject:
				issue.Project = m.pendingEdit.optimistic.Project
			case editLabels:
				issue.Labels = append([]linear.Label(nil), m.pendingEdit.optimistic.Labels...)
			case editDescription:
				issue.Description = m.pendingEdit.optimistic.Description
			}
			issue.UpdatedAt = time.Now()
			m.replaceIssue(issue)
			return
		}
	}
}

func (m *Model) rebaseOpenEditors() {
	if m.archiveConfirm != nil {
		if _, ok := m.dashboardIssue(m.archiveConfirm.issueID); !ok {
			m.archiveConfirm = nil
			m.editErr = fmt.Errorf("the issue archive confirmation is no longer available")
		}
	}
	if m.actionMenu != nil {
		if _, ok := m.dashboardIssue(m.actionMenu.issueID); !ok {
			m.actionMenu = nil
			m.editErr = fmt.Errorf("the issue action menu is no longer available")
		}
	}
	if m.editor != nil {
		issue, ok := m.dashboardIssue(m.editor.issueID)
		if !ok {
			m.editor = nil
			m.editErr = fmt.Errorf("the issue being edited is no longer available")
		} else {
			m.editor.original = issue
		}
	}
	if m.choiceEditor != nil {
		issue, ok := m.dashboardIssue(m.choiceEditor.issueID)
		if !ok {
			m.choiceEditor = nil
			m.editErr = fmt.Errorf("the issue being edited is no longer available")
			return
		}
		selected := issueChoice{}
		if len(m.choiceEditor.options) > 0 {
			selected = m.choiceEditor.options[m.choiceEditor.selected]
		}
		m.choiceEditor.original = issue
		switch m.choiceEditor.kind {
		case editStatus:
			m.choiceEditor.options = statusChoices(m.editableStates(issue))
		case editAssignee:
			m.choiceEditor.options = assigneeChoices(m.dashboard.Users, issue.Assignee)
		case editProject:
			m.choiceEditor.options = projectChoices(m.editableProjects(issue), issue.Project)
		}
		m.choiceEditor.selected = 0
		for index, option := range m.choiceEditor.options {
			if option.sameValue(selected, m.choiceEditor.kind) {
				m.choiceEditor.selected = index
				break
			}
		}
	}
	if m.labelEditor != nil {
		m.rebaseLabelEditor()
	}
	if m.descriptionEditor != nil {
		issue, ok := m.dashboardIssue(m.descriptionEditor.issueID)
		if !ok {
			m.descriptionEditor = nil
			m.editErr = fmt.Errorf("the issue being edited is no longer available")
		} else {
			m.descriptionEditor.original = issue
		}
	}
}

func (m Model) dashboardIssue(issueID string) (linear.Issue, bool) {
	for _, issue := range m.dashboard.Issues {
		if issue.ID == issueID {
			return issue, true
		}
	}
	return linear.Issue{}, false
}

func (m *Model) replaceIssue(replacement linear.Issue) {
	selectedID := replacement.ID
	for index := range m.dashboard.Issues {
		if m.dashboard.Issues[index].ID == replacement.ID {
			m.dashboard.Issues[index] = replacement
			break
		}
	}
	m.filterIssues()
	for index := range m.issues {
		if m.issues[index].ID == selectedID {
			m.selected = index
			break
		}
	}
}

func (m Model) renderTitleEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.editor == nil {
		return panel("Edit title", width, height, fitLines(nil, innerHeight))
	}
	lines := []string{
		accentStyle.Bold(true).Render(m.editor.identifier),
		"",
		mutedStyle.Render("Title"),
		"> " + editorWindow(m.editor.value, m.editor.cursor, max(1, innerWidth-2)),
		"",
		mutedStyle.Render("Enter saves · Esc cancels · Ctrl+U clears"),
		mutedStyle.Render("←/→ moves the cursor · Home/End jumps"),
	}
	if m.editor.err != nil {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(theme.red).Render("Error: "+m.editor.err.Error()))
	}
	return panel("Edit title", width, height, fitLines(lines, innerHeight))
}

func editorWindow(value []rune, cursor, width int) string {
	cursor = clamp(cursor, 0, len(value))
	width = max(1, width)
	start := 0
	if cursor >= width {
		start = cursor - width + 1
	}
	end := min(len(value), start+width-1)
	visible := append([]rune(nil), value[start:end]...)
	position := cursor - start
	visible = append(visible, 0)
	copy(visible[position+1:], visible[position:])
	visible[position] = '│'
	return string(visible)
}
