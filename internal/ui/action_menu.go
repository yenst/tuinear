package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

type issueAction uint8

const (
	actionEditTitle issueAction = iota
	actionEditStatus
	actionEditPriority
	actionEditAssignee
	actionEditProject
	actionEditLabels
	actionEditDescription
	actionOpenBrowser
	actionArchiveIssue
)

type actionOption struct {
	action   issueAction
	shortcut string
	label    string
	glyph    string
}

type actionMenu struct {
	issueID    string
	identifier string
	options    []actionOption
	selected   int
}

func (m *Model) beginIssueActions() {
	issue := m.selectedIssue()
	if issue == nil {
		return
	}
	options := make([]actionOption, 0, 9)
	if m.issueUpdater != nil {
		options = append(options,
			actionOption{action: actionEditTitle, shortcut: "e", label: "Edit title", glyph: "✎"},
			actionOption{action: actionEditStatus, shortcut: "s", label: "Change status", glyph: statusGlyph(issue.State.Type)},
			actionOption{action: actionEditPriority, shortcut: "p", label: "Change priority", glyph: priorityGlyph(issue.Priority)},
			actionOption{action: actionEditAssignee, shortcut: "u", label: "Change assignee", glyph: "@"},
			actionOption{action: actionEditProject, shortcut: "P", label: "Change project", glyph: "◇"},
			actionOption{action: actionEditLabels, shortcut: "l", label: "Edit labels", glyph: "#"},
			actionOption{action: actionEditDescription, shortcut: "d", label: "Edit description", glyph: "¶"},
		)
	}
	if strings.TrimSpace(issue.URL) != "" {
		options = append(options, actionOption{action: actionOpenBrowser, shortcut: "space", label: "Open in Linear", glyph: "↗"})
	}
	if m.issueArchiver != nil {
		options = append(options, actionOption{action: actionArchiveIssue, shortcut: "x", label: "Archive issue", glyph: "⌫"})
	}
	if len(options) == 0 {
		m.editErr = fmt.Errorf("no actions are available for %s", issue.Identifier)
		return
	}
	m.actionMenu = &actionMenu{
		issueID: issue.ID, identifier: issue.Identifier, options: options,
	}
	m.editErr = nil
	m.browserErr = nil
}

func (m *Model) updateActionMenu(msg tea.KeyPressMsg) tea.Cmd {
	menu := m.actionMenu
	if menu == nil || len(menu.options) == 0 {
		m.actionMenu = nil
		return nil
	}
	switch msg.String() {
	case "esc":
		m.actionMenu = nil
	case "j", "down", "right", "tab":
		menu.selected = (menu.selected + 1) % len(menu.options)
	case "k", "up", "left", "shift+tab":
		menu.selected = (menu.selected - 1 + len(menu.options)) % len(menu.options)
	case "g", "home":
		menu.selected = 0
	case "G", "end":
		menu.selected = len(menu.options) - 1
	case "enter", "return":
		return m.runIssueAction(menu.options[menu.selected].action)
	case "e":
		return m.runIssueAction(actionEditTitle)
	case "s":
		return m.runIssueAction(actionEditStatus)
	case "p":
		return m.runIssueAction(actionEditPriority)
	case "u":
		return m.runIssueAction(actionEditAssignee)
	case "P":
		return m.runIssueAction(actionEditProject)
	case "l":
		return m.runIssueAction(actionEditLabels)
	case "d":
		return m.runIssueAction(actionEditDescription)
	case "space", " ":
		return m.runIssueAction(actionOpenBrowser)
	case "x":
		return m.runIssueAction(actionArchiveIssue)
	}
	return nil
}

func (m *Model) runIssueAction(action issueAction) tea.Cmd {
	if m.actionMenu == nil || !m.actionMenu.has(action) {
		return nil
	}
	m.actionMenu = nil
	switch action {
	case actionEditTitle:
		m.beginTitleEdit()
	case actionEditStatus:
		m.beginStatusEdit()
	case actionEditPriority:
		m.beginPriorityEdit()
	case actionEditAssignee:
		m.beginAssigneeEdit()
	case actionEditProject:
		m.beginProjectEdit()
	case actionEditLabels:
		m.beginLabelEdit()
	case actionEditDescription:
		m.beginDescriptionEdit()
	case actionOpenBrowser:
		return m.openSelectedIssue()
	case actionArchiveIssue:
		m.beginArchiveConfirmation()
	}
	return nil
}

func (menu actionMenu) has(action issueAction) bool {
	for _, option := range menu.options {
		if option.action == action {
			return true
		}
	}
	return false
}

func (m Model) renderActionMenu(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.actionMenu == nil {
		return panel("Issue actions", width, height, fitLines(nil, innerHeight))
	}
	menu := m.actionMenu
	issue, ok := m.dashboardIssue(menu.issueID)
	if !ok {
		return panel("Issue actions", width, height, fitLines([]string{mutedStyle.Render("Issue unavailable.")}, innerHeight))
	}
	lines := []string{
		accentStyle.Bold(true).Render(menu.identifier),
		lipgloss.NewStyle().Foreground(theme.text).Render(clip(issue.Title, innerWidth)),
		mutedStyle.Render("Choose an action, then press Enter."),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := 0
	if menu.selected >= capacity {
		start = menu.selected - capacity + 1
	}
	end := min(len(menu.options), start+capacity)
	for index := start; index < end; index++ {
		option := menu.options[index]
		prefix := "  "
		if index == menu.selected {
			prefix = "› "
		}
		detail := actionDetail(option.action, issue)
		shortcut := "[" + option.shortcut + "]"
		left := prefix + option.glyph + " " + option.label
		available := max(1, innerWidth-lipgloss.Width(left)-lipgloss.Width(shortcut)-3)
		line := left + "  " + clip(detail, available)
		gap := max(1, innerWidth-lipgloss.Width(line)-lipgloss.Width(shortcut))
		line += strings.Repeat(" ", gap) + shortcut
		if index == menu.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Issue actions", width, height, fitLines(lines, innerHeight))
}

func actionDetail(action issueAction, issue linear.Issue) string {
	switch action {
	case actionEditTitle:
		return issue.Title
	case actionEditStatus:
		return issue.State.Name
	case actionEditPriority:
		return issue.PriorityLabel()
	case actionEditAssignee:
		if issue.Assignee == nil {
			return "Unassigned"
		}
		return issue.Assignee.Label()
	case actionEditProject:
		if issue.Project == nil {
			return "No project"
		}
		return issue.Project.Name
	case actionEditLabels:
		if len(issue.Labels) == 0 {
			return "No labels"
		}
		names := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			names = append(names, label.Name)
		}
		return strings.Join(names, ", ")
	case actionEditDescription:
		description := strings.TrimSpace(issue.Description)
		if description == "" {
			return "No description"
		}
		return strings.ReplaceAll(description, "\n", " ")
	case actionOpenBrowser:
		return "Default browser"
	case actionArchiveIssue:
		return "Recoverable from Linear's archive"
	default:
		return ""
	}
}

func priorityGlyph(priority int) string {
	switch priority {
	case 1:
		return "!!!"
	case 2:
		return "!!"
	case 3:
		return "!"
	case 4:
		return "↓"
	default:
		return "—"
	}
}
