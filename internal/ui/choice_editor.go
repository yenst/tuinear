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

type issueChoice struct {
	label    string
	glyph    string
	state    linear.WorkflowState
	priority int
	assignee *linear.User
	project  *linear.Project
}

func (choice issueChoice) sameValue(other issueChoice, kind issueEditKind) bool {
	switch kind {
	case editStatus:
		return choice.state.ID == other.state.ID
	case editAssignee:
		return assigneeID(choice.assignee) == assigneeID(other.assignee)
	case editProject:
		return projectID(choice.project) == projectID(other.project)
	default:
		return choice.priority == other.priority
	}
}

type choiceEditor struct {
	kind        issueEditKind
	field       string
	instruction string
	issueID     string
	identifier  string
	original    linear.Issue
	options     []issueChoice
	selected    int
}

func (m *Model) beginStatusEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	options := statusChoices(m.editableStates(issue))
	if len(options) == 0 {
		m.editErr = fmt.Errorf("no editable statuses are available for %s", issue.Team.Name)
		return
	}
	m.choiceEditor = &choiceEditor{
		kind: editStatus, field: "status", instruction: "Choose a team status, then press Enter.",
		issueID: issue.ID, identifier: issue.Identifier, original: issue, options: options,
	}
	for index, option := range options {
		if option.state.ID == issue.State.ID || (issue.State.ID == "" && option.state.Name == issue.State.Name) {
			m.choiceEditor.selected = index
			break
		}
	}
}

func (m *Model) beginPriorityEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	options := priorityChoices()
	m.choiceEditor = &choiceEditor{
		kind: editPriority, field: "priority", instruction: "Choose a priority, then press Enter.",
		issueID: issue.ID, identifier: issue.Identifier, original: issue, options: options,
	}
	for index, option := range options {
		if option.priority == issue.Priority {
			m.choiceEditor.selected = index
			break
		}
	}
}

func (m *Model) beginAssigneeEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	options := assigneeChoices(m.dashboard.Users, issue.Assignee)
	if len(options) == 1 && issue.Assignee == nil {
		m.editErr = fmt.Errorf("no workspace members are available")
		return
	}
	m.choiceEditor = &choiceEditor{
		kind: editAssignee, field: "assignee", instruction: "Choose a workspace member, then press Enter.",
		issueID: issue.ID, identifier: issue.Identifier, original: issue, options: options,
	}
	currentID := assigneeID(issue.Assignee)
	for index, option := range options {
		if assigneeID(option.assignee) == currentID {
			m.choiceEditor.selected = index
			break
		}
	}
}

func (m *Model) beginProjectEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	options := projectChoices(m.editableProjects(issue), issue.Project)
	if len(options) == 1 && issue.Project == nil {
		m.editErr = fmt.Errorf("no projects are available for %s", issue.Team.Name)
		return
	}
	m.choiceEditor = &choiceEditor{
		kind: editProject, field: "project", instruction: "Choose a team project, then press Enter.",
		issueID: issue.ID, identifier: issue.Identifier, original: issue, options: options,
	}
	currentID := projectID(issue.Project)
	for index, option := range options {
		if projectID(option.project) == currentID {
			m.choiceEditor.selected = index
			break
		}
	}
}

func (m *Model) beginChoiceEdit() (linear.Issue, bool) {
	if m.pendingArchive != nil {
		m.editErr = fmt.Errorf("wait for %s to finish archiving", m.pendingArchive.identifier)
		return linear.Issue{}, false
	}
	if m.pendingEdit != nil {
		m.editErr = fmt.Errorf("wait for %s to finish saving", m.pendingEdit.identifier)
		return linear.Issue{}, false
	}
	issue := m.selectedIssue()
	if issue == nil {
		return linear.Issue{}, false
	}
	if m.issueUpdater == nil {
		m.editErr = fmt.Errorf("issue editing is not available for this data source")
		return linear.Issue{}, false
	}
	m.editErr = nil
	m.browserErr = nil
	return *issue, true
}

func statusChoices(states []linear.WorkflowState) []issueChoice {
	options := make([]issueChoice, 0, len(states))
	for _, state := range states {
		options = append(options, issueChoice{label: state.Name, glyph: statusGlyph(state.Type), state: state})
	}
	return options
}

func priorityChoices() []issueChoice {
	return []issueChoice{
		{label: "No priority", glyph: priorityGlyph(0), priority: 0},
		{label: "Urgent", glyph: priorityGlyph(1), priority: 1},
		{label: "High", glyph: priorityGlyph(2), priority: 2},
		{label: "Medium", glyph: priorityGlyph(3), priority: 3},
		{label: "Low", glyph: priorityGlyph(4), priority: 4},
	}
}

func assigneeChoices(users []linear.User, current *linear.User) []issueChoice {
	seen := make(map[string]bool, len(users)+1)
	values := make([]linear.User, 0, len(users)+1)
	for _, user := range users {
		if strings.TrimSpace(user.ID) == "" || seen[user.ID] {
			continue
		}
		seen[user.ID] = true
		values = append(values, user)
	}
	if current != nil && strings.TrimSpace(current.ID) != "" && !seen[current.ID] {
		values = append(values, *current)
	}
	sort.SliceStable(values, func(i, j int) bool {
		return strings.ToLower(values[i].Label()) < strings.ToLower(values[j].Label())
	})
	options := []issueChoice{{label: "Unassigned", glyph: "○"}}
	for index := range values {
		options = append(options, issueChoice{label: values[index].Label(), glyph: "@", assignee: &values[index]})
	}
	return options
}

func assigneeID(user *linear.User) string {
	if user == nil {
		return ""
	}
	return user.ID
}

func projectChoices(projects []linear.Project, current *linear.Project) []issueChoice {
	seen := make(map[string]bool, len(projects)+1)
	values := make([]linear.Project, 0, len(projects)+1)
	for _, project := range projects {
		if strings.TrimSpace(project.ID) == "" || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		values = append(values, project)
	}
	if current != nil && strings.TrimSpace(current.ID) != "" && !seen[current.ID] {
		values = append(values, *current)
	}
	sort.SliceStable(values, func(i, j int) bool {
		return strings.ToLower(values[i].Name) < strings.ToLower(values[j].Name)
	})
	options := []issueChoice{{label: "No project", glyph: "○"}}
	for index := range values {
		options = append(options, issueChoice{label: values[index].Name, glyph: "◇", project: &values[index]})
	}
	return options
}

func projectID(project *linear.Project) string {
	if project == nil {
		return ""
	}
	return project.ID
}

func (m Model) editableProjects(issue linear.Issue) []linear.Project {
	projects := append([]linear.Project(nil), m.dashboard.ProjectsForTeam(issue.Team.ID)...)
	if len(projects) == 0 {
		for _, candidate := range m.dashboard.Issues {
			if candidate.Team.ID == issue.Team.ID && candidate.Project != nil {
				projects = append(projects, *candidate.Project)
			}
		}
	}
	return projects
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

func (m *Model) updateChoiceEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.choiceEditor
	if editor == nil || len(editor.options) == 0 {
		m.choiceEditor = nil
		return nil
	}
	switch msg.String() {
	case "esc":
		m.choiceEditor = nil
	case "j", "down", "right", "tab":
		editor.selected = (editor.selected + 1) % len(editor.options)
	case "k", "up", "left", "shift+tab":
		editor.selected = (editor.selected - 1 + len(editor.options)) % len(editor.options)
	case "g", "home":
		editor.selected = 0
	case "G", "end":
		editor.selected = len(editor.options) - 1
	case "enter", "return":
		return m.submitChoiceEdit()
	}
	return nil
}

func (m *Model) submitChoiceEdit() tea.Cmd {
	if m.choiceEditor == nil || m.issueUpdater == nil || len(m.choiceEditor.options) == 0 {
		return nil
	}
	editor := m.choiceEditor
	choice := editor.options[editor.selected]
	before := editor.original
	optimistic := before
	var update linear.IssueUpdate
	var action string
	switch editor.kind {
	case editStatus:
		if choice.state.ID == before.State.ID || (before.State.ID == "" && choice.state.Name == before.State.Name) {
			m.choiceEditor = nil
			return nil
		}
		optimistic.State = choice.state
		update.StateID = &choice.state.ID
		action = "status"
	case editPriority:
		if choice.priority == before.Priority {
			m.choiceEditor = nil
			return nil
		}
		optimistic.Priority = choice.priority
		update.Priority = &choice.priority
		action = "priority"
	case editAssignee:
		if assigneeID(choice.assignee) == assigneeID(before.Assignee) {
			m.choiceEditor = nil
			return nil
		}
		var selectedID *string
		if choice.assignee != nil {
			assignee := *choice.assignee
			optimistic.Assignee = &assignee
			id := assignee.ID
			selectedID = &id
		} else {
			optimistic.Assignee = nil
		}
		update.AssigneeID = &selectedID
		action = "assignee"
	case editProject:
		if projectID(choice.project) == projectID(before.Project) {
			m.choiceEditor = nil
			return nil
		}
		var selectedID *string
		if choice.project != nil {
			project := *choice.project
			optimistic.Project = &project
			id := project.ID
			selectedID = &id
		} else {
			optimistic.Project = nil
		}
		update.ProjectID = &selectedID
		action = "project"
	default:
		return nil
	}
	optimistic.UpdatedAt = time.Now()
	m.pendingEdit = &pendingIssueEdit{
		issueID: before.ID, identifier: before.Identifier, before: before,
		optimistic: optimistic, kind: editor.kind,
	}
	m.choiceEditor = nil
	m.editErr = nil
	m.replaceIssue(optimistic)
	return updateIssueChoice(m.issueUpdater, before.ID, update, action)
}

func updateIssueChoice(updater IssueUpdater, issueID string, update linear.IssueUpdate, action string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := updater.UpdateIssue(ctx, issueID, update)
		if err != nil {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save %s: %w", action, err)}
		}
		if issue.ID != issueID {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save %s: Linear returned a different issue", action)}
		}
		return issueUpdatedMsg{issue: issue}
	}
}

func (m Model) renderChoiceEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.choiceEditor == nil {
		return panel("Edit issue", width, height, fitLines(nil, innerHeight))
	}
	editor := m.choiceEditor
	lines := []string{
		accentStyle.Bold(true).Render(editor.identifier),
		mutedStyle.Render(editor.instruction),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := 0
	if editor.selected >= capacity {
		start = editor.selected - capacity + 1
	}
	end := min(len(editor.options), start+capacity)
	for index := start; index < end; index++ {
		choice := editor.options[index]
		prefix := "  "
		if index == editor.selected {
			prefix = "› "
		}
		line := prefix + choice.glyph + " " + clip(choice.label, max(1, innerWidth-lipgloss.Width(choice.glyph)-3))
		if index == editor.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Edit "+editor.field, width, height, fitLines(lines, innerHeight))
}
