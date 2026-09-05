package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

type createIssueField uint8

const (
	createTitle createIssueField = iota
	createDescription
	createStatus
	createPriority
	createAssignee
	createProject
	createLabels
	createSubmit
)

type createIssueMode uint8

const (
	createBrowse createIssueMode = iota
	createEditTitle
	createEditDescription
	createEditChoice
	createEditLabels
)

type createIssueEditor struct {
	team              linear.Team
	selected          createIssueField
	mode              createIssueMode
	title             []rune
	titleCursor       int
	description       []rune
	descriptionCursor int
	state             linear.WorkflowState
	priority          int
	assignee          *linear.User
	project           *linear.Project
	labels            []linear.Label
	choiceOptions     []issueChoice
	choiceSelected    int
	labelOptions      []linear.Label
	labelSelected     int
	labelChecked      map[string]bool
	err               error
}

type pendingIssueCreate struct {
	temporaryID string
	draft       createIssueEditor
	optimistic  linear.Issue
}

type issueCreatedMsg struct{ issue linear.Issue }
type issueCreateFailedMsg struct {
	temporaryID string
	err         error
}

func (m *Model) beginIssueCreate() {
	if m.pendingEdit != nil || m.pendingArchive != nil || m.pendingCreate != nil {
		m.editErr = fmt.Errorf("wait for the current ticket action to finish")
		return
	}
	if m.issueCreator == nil {
		m.editErr = fmt.Errorf("issue creation is not available for this data source")
		return
	}
	if m.teamIndex == 0 || m.teamIndex > len(m.dashboard.Teams) {
		m.editErr = fmt.Errorf("select a team before creating a ticket")
		return
	}
	team := m.dashboard.Teams[m.teamIndex-1]
	m.createEditor = &createIssueEditor{
		team: team, state: defaultState(m.dashboard.StatesForTeam(team.ID)), mode: createEditTitle,
	}
	m.editErr = nil
	m.browserErr = nil
}

func (m *Model) submitIssueCreate() tea.Cmd {
	if m.createEditor == nil || m.issueCreator == nil {
		return nil
	}
	title := strings.TrimSpace(string(m.createEditor.title))
	if title == "" {
		m.createEditor.err = fmt.Errorf("title cannot be empty")
		m.createEditor.selected = createTitle
		return nil
	}
	editor := m.createEditor
	team := editor.team
	now := time.Now()
	temporaryID := fmt.Sprintf("tuinear-create-%d", now.UnixNano())
	optimistic := linear.Issue{
		ID: temporaryID, Identifier: "NEW", Title: title,
		Description: string(editor.description), Priority: editor.priority,
		Team: team, State: editor.state, Assignee: cloneUser(editor.assignee),
		Project: cloneProject(editor.project), Labels: append([]linear.Label(nil), editor.labels...),
		CreatedAt: now, UpdatedAt: now,
	}
	draft := *editor
	draft.mode = createBrowse
	draft.err = nil
	m.pendingCreate = &pendingIssueCreate{
		temporaryID: temporaryID, draft: draft, optimistic: optimistic,
	}
	m.createEditor = nil
	m.editErr = nil
	m.prependIssue(optimistic)
	return createIssue(m.issueCreator, temporaryID, issueCreateInput(editor, title))
}

func issueCreateInput(editor *createIssueEditor, title string) linear.IssueCreate {
	priority := editor.priority
	create := linear.IssueCreate{
		TeamID: editor.team.ID, Title: title, Description: string(editor.description),
		Priority: &priority,
	}
	create.StateID = editor.state.ID
	if editor.assignee != nil {
		create.AssigneeID = editor.assignee.ID
	}
	if editor.project != nil {
		create.ProjectID = editor.project.ID
	}
	for _, label := range editor.labels {
		create.LabelIDs = append(create.LabelIDs, label.ID)
	}
	return create
}

func cloneUser(user *linear.User) *linear.User {
	if user == nil {
		return nil
	}
	cloned := *user
	return &cloned
}

func cloneProject(project *linear.Project) *linear.Project {
	if project == nil {
		return nil
	}
	cloned := *project
	return &cloned
}

func defaultState(states []linear.WorkflowState) linear.WorkflowState {
	for _, wanted := range []string{"triage", "backlog", "unstarted"} {
		for _, state := range states {
			if state.Type == wanted {
				return state
			}
		}
	}
	if len(states) > 0 {
		return states[0]
	}
	return linear.WorkflowState{}
}

func createIssue(creator IssueCreator, temporaryID string, create linear.IssueCreate) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := creator.CreateIssue(ctx, create)
		if err != nil {
			return issueCreateFailedMsg{temporaryID: temporaryID, err: fmt.Errorf("create ticket: %w", err)}
		}
		if issue.ID == "" || issue.Team.ID != create.TeamID {
			return issueCreateFailedMsg{temporaryID: temporaryID, err: fmt.Errorf("create ticket: Linear returned an invalid issue")}
		}
		return issueCreatedMsg{issue: issue}
	}
}

func (m *Model) finishIssueCreate(issue linear.Issue) {
	if m.pendingCreate == nil {
		return
	}
	temporaryID := m.pendingCreate.temporaryID
	m.removeIssue(temporaryID)
	found := false
	for index := range m.dashboard.Issues {
		if m.dashboard.Issues[index].ID == issue.ID {
			m.dashboard.Issues[index] = issue
			found = true
			break
		}
	}
	if !found {
		m.dashboard.Issues = append([]linear.Issue{issue}, m.dashboard.Issues...)
	}
	m.pendingCreate = nil
	m.editErr = nil
	m.filterIssues()
	for index := range m.issues {
		if m.issues[index].ID == issue.ID {
			m.selected = index
			break
		}
	}
}

func (m *Model) failIssueCreate(temporaryID string, err error) {
	if m.pendingCreate == nil || m.pendingCreate.temporaryID != temporaryID {
		return
	}
	draft := m.pendingCreate
	m.removeIssue(temporaryID)
	m.pendingCreate = nil
	m.editErr = err
	restored := draft.draft
	restored.err = err
	m.createEditor = &restored
}

func (m *Model) rebasePendingIssueCreate() {
	if m.pendingCreate == nil {
		return
	}
	for _, issue := range m.dashboard.Issues {
		if issue.ID == m.pendingCreate.temporaryID {
			return
		}
	}
	m.prependIssue(m.pendingCreate.optimistic)
}

func (m *Model) prependIssue(issue linear.Issue) {
	for _, current := range m.dashboard.Issues {
		if current.ID == issue.ID {
			return
		}
	}
	m.dashboard.Issues = append([]linear.Issue{issue}, m.dashboard.Issues...)
	m.filterIssues()
	for index := range m.issues {
		if m.issues[index].ID == issue.ID {
			m.selected = index
			break
		}
	}
}

func (m *Model) removeIssue(issueID string) {
	issues := m.dashboard.Issues[:0]
	for _, issue := range m.dashboard.Issues {
		if issue.ID != issueID {
			issues = append(issues, issue)
		}
	}
	m.dashboard.Issues = issues
	m.filterIssues()
}
