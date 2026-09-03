package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

type issueCreatorStub struct {
	issue  linear.Issue
	err    error
	calls  int
	create linear.IssueCreate
}

func (s *issueCreatorStub) CreateIssue(_ context.Context, create linear.IssueCreate) (linear.Issue, error) {
	s.calls++
	s.create = create
	return s.issue, s.err
}

func TestIssueCreateRequiresExplicitTeamAndValidTitle(t *testing.T) {
	dashboard := editorDashboard(t)
	creator := &issueCreatorStub{}
	m := NewWithDashboardAndCreator(dashboard, creator)
	m = updateKey(m, textKey("n"))
	if m.createEditor != nil || m.editErr == nil || !strings.Contains(m.View().Content, "select a team") {
		t.Fatalf("all-team creation state = editor=%#v error=%v", m.createEditor, m.editErr)
	}
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))
	if m.createEditor == nil || m.createEditor.team.ID != dashboard.Teams[0].ID {
		t.Fatalf("create editor = %#v", m.createEditor)
	}
	if !strings.Contains(m.View().Content, dashboard.Teams[0].Name) {
		t.Fatal("create editor does not identify the selected team")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.createEditor == nil || m.createEditor.err != nil || creator.calls != 0 {
		t.Fatalf("empty title field = cmd=%v editor=%#v calls=%d", cmd != nil, m.createEditor, creator.calls)
	}
	updated, cmd = m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	if cmd != nil || m.createEditor == nil || m.createEditor.err == nil || m.createEditor.selected != createTitle || creator.calls != 0 {
		t.Fatalf("empty title submit = cmd=%v editor=%#v calls=%d", cmd != nil, m.createEditor, creator.calls)
	}
}

func TestIssueCreateIsOptimisticSurvivesRefreshAndUsesCanonicalIssue(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := linear.Issue{
		ID: "created-1", Identifier: "PLAT-99", Title: "Canonical title",
		Team: dashboard.Teams[0], State: dashboard.StatesForTeam(dashboard.Teams[0].ID)[1],
	}
	creator := &issueCreatorStub{issue: canonical}
	m := NewWithDashboardAndCreator(dashboard, creator)
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))
	m = updateKey(m, textKey("  Draft ticket  "))
	m = updateKey(m, specialKey(tea.KeyEnter))
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	if cmd == nil || m.pendingCreate == nil || len(m.dashboard.Issues) != len(dashboard.Issues)+1 {
		t.Fatalf("optimistic create = cmd=%v pending=%#v issues=%d", cmd != nil, m.pendingCreate, len(m.dashboard.Issues))
	}
	temporaryID := m.pendingCreate.temporaryID
	if m.dashboard.Issues[0].ID != temporaryID || m.dashboard.Issues[0].Title != "Draft ticket" {
		t.Fatalf("optimistic issue = %#v", m.dashboard.Issues[0])
	}

	updated, _ = m.Update(dashboardLoadedMsg{dashboard: dashboard})
	m = updated.(Model)
	placeholders := 0
	for _, issue := range m.dashboard.Issues {
		if issue.ID == temporaryID {
			placeholders++
		}
	}
	if placeholders != 1 {
		t.Fatalf("placeholder count after refresh = %d", placeholders)
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if creator.calls != 1 || creator.create.TeamID != dashboard.Teams[0].ID || creator.create.Title != "Draft ticket" {
		t.Fatalf("create call = calls=%d input=%#v", creator.calls, creator.create)
	}
	if m.pendingCreate != nil || m.createEditor != nil {
		t.Fatalf("confirmed state = pending=%#v editor=%#v", m.pendingCreate, m.createEditor)
	}
	foundCanonical, foundTemporary := false, false
	for _, issue := range m.dashboard.Issues {
		foundCanonical = foundCanonical || issue.ID == canonical.ID
		foundTemporary = foundTemporary || issue.ID == temporaryID
	}
	if !foundCanonical || foundTemporary {
		t.Fatalf("canonical reconciliation = found canonical %v temporary %v", foundCanonical, foundTemporary)
	}
}

func TestFailedIssueCreateRemovesPlaceholderAndRestoresDraft(t *testing.T) {
	dashboard := editorDashboard(t)
	creator := &issueCreatorStub{err: errors.New("creation forbidden")}
	m := NewWithDashboardAndCreator(dashboard, creator)
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))
	m = updateKey(m, textKey("Retry me"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	m.createEditor.description = []rune("Preserve **all** of this")
	m.createEditor.priority = 2
	m.createEditor.assignee = &dashboard.Users[0]
	projects := dashboard.ProjectsForTeam(dashboard.Teams[0].ID)
	labels := dashboard.LabelsForTeam(dashboard.Teams[0].ID)
	if len(projects) == 0 || len(labels) < 2 {
		t.Fatal("demo dashboard does not have enough creation metadata")
	}
	m.createEditor.project = &projects[0]
	m.createEditor.labels = append([]linear.Label(nil), labels[:2]...)
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	temporaryID := m.pendingCreate.temporaryID
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.pendingCreate != nil || m.createEditor == nil || string(m.createEditor.title) != "Retry me" {
		t.Fatalf("retry state = pending=%#v editor=%#v", m.pendingCreate, m.createEditor)
	}
	if string(m.createEditor.description) != "Preserve **all** of this" || m.createEditor.priority != 2 ||
		m.createEditor.assignee == nil || m.createEditor.assignee.ID != dashboard.Users[0].ID ||
		m.createEditor.project == nil || m.createEditor.project.ID != projects[0].ID || len(m.createEditor.labels) != 2 {
		t.Fatalf("complete retry draft was not restored: %#v", m.createEditor)
	}
	for _, issue := range m.dashboard.Issues {
		if issue.ID == temporaryID {
			t.Fatal("failed placeholder remained in dashboard")
		}
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "creation forbidden") {
		t.Fatalf("creation error = %v", m.editErr)
	}
}

func TestIssueCreateFormSubmitsEveryEditableField(t *testing.T) {
	dashboard := editorDashboard(t)
	team := dashboard.Teams[0]
	states := dashboard.StatesForTeam(team.ID)
	projects := dashboard.ProjectsForTeam(team.ID)
	labels := dashboard.LabelsForTeam(team.ID)
	if len(states) < 2 || len(dashboard.Users) == 0 || len(projects) == 0 || len(labels) < 2 {
		t.Fatal("demo dashboard does not have enough creation metadata")
	}
	canonical := linear.Issue{ID: "created-all-fields", Identifier: team.Key + "-99", Team: team}
	creator := &issueCreatorStub{issue: canonical}
	m := NewWithDashboardAndCreator(dashboard, creator)
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))

	// Title.
	m = updateKey(m, textKey("Complete creation form"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	// Description.
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("# Context"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("Details"))
	m = updateKey(m, textKey("ctrl+s"))
	// Status: move away from the default.
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("j"))
	wantState := m.createEditor.choiceOptions[m.createEditor.choiceSelected].state
	m = updateKey(m, specialKey(tea.KeyEnter))
	// Priority: urgent.
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("j"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	// Assignee: first user.
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("j"))
	wantAssignee := m.createEditor.choiceOptions[m.createEditor.choiceSelected].assignee
	m = updateKey(m, specialKey(tea.KeyEnter))
	// Project: first team project.
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("j"))
	wantProject := m.createEditor.choiceOptions[m.createEditor.choiceSelected].project
	m = updateKey(m, specialKey(tea.KeyEnter))
	// Labels: first two.
	m = updateKey(m, specialKey(tea.KeyEnter))
	wantLabelIDs := []string{m.createEditor.labelOptions[0].ID, m.createEditor.labelOptions[1].ID}
	m = updateKey(m, textKey("space"))
	m = updateKey(m, textKey("j"))
	m = updateKey(m, textKey("space"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	view := m.View().Content
	for _, value := range []string{"Complete creation form", "# Context", wantState.Name, "Urgent", wantAssignee.Label(), wantProject.Name, "Create ticket"} {
		if !strings.Contains(view, value) {
			t.Fatalf("creation form does not show %q", value)
		}
	}

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.pendingCreate == nil {
		t.Fatalf("complete draft was not submitted: cmd=%v pending=%#v", cmd != nil, m.pendingCreate)
	}
	optimistic := m.pendingCreate.optimistic
	if optimistic.Description != "# Context\nDetails" || optimistic.State.ID != wantState.ID || optimistic.Priority != 1 ||
		optimistic.Assignee == nil || optimistic.Assignee.ID != wantAssignee.ID || optimistic.Project == nil || optimistic.Project.ID != wantProject.ID ||
		len(optimistic.Labels) != 2 {
		t.Fatalf("optimistic issue = %#v", optimistic)
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	create := creator.create
	if create.Description != "# Context\nDetails" || create.StateID != wantState.ID || create.Priority == nil || *create.Priority != 1 ||
		create.AssigneeID != wantAssignee.ID || create.ProjectID != wantProject.ID || strings.Join(create.LabelIDs, ",") != strings.Join(wantLabelIDs, ",") {
		t.Fatalf("complete create payload = %#v", create)
	}
}

func TestIssueCreateFormEscapeReturnsThenCancels(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndCreator(dashboard, &issueCreatorStub{})
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))
	m = updateKey(m, textKey("Preserved draft"))
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.createEditor == nil || m.createEditor.mode != createBrowse || string(m.createEditor.title) != "Preserved draft" {
		t.Fatalf("first escape did not return to form: %#v", m.createEditor)
	}
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.createEditor != nil {
		t.Fatal("second escape did not cancel creation")
	}
}

func TestIssueCreateFormReportsUnavailableTeamMetadata(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.TeamStates = nil
	dashboard.TeamLabels = nil
	dashboard.WorkspaceLabels = nil
	dashboard.Issues = nil
	m := NewWithDashboardAndCreator(dashboard, &issueCreatorStub{})
	m = updateKey(m, textKey("tab"))
	m = updateKey(m, textKey("n"))
	m.createEditor.mode = createBrowse

	for _, field := range []createIssueField{createStatus, createLabels} {
		m.createEditor.selected = field
		m = updateKey(m, specialKey(tea.KeyEnter))
		if m.createEditor.mode != createBrowse || m.createEditor.err == nil {
			t.Fatalf("unavailable field %d = mode %d error %v", field, m.createEditor.mode, m.createEditor.err)
		}
	}
}
