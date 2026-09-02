package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestProjectEditorShowsOnlyTeamProjectsAndCurrentSelection(t *testing.T) {
	dashboard := editorDashboard(t)
	issue := dashboard.Issues[0]
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("P"))
	if m.choiceEditor == nil || m.choiceEditor.kind != editProject || len(m.choiceEditor.options) != len(dashboard.ProjectsForTeam(issue.Team.ID))+1 {
		t.Fatalf("project editor = %#v", m.choiceEditor)
	}
	selected := m.choiceEditor.options[m.choiceEditor.selected].project
	if selected == nil || issue.Project == nil || selected.ID != issue.Project.ID {
		t.Fatalf("selected project = %#v, want %#v", selected, issue.Project)
	}
	view := m.View().Content
	for _, want := range []string{"Edit project", "No project", "Platform quality", "Public launch"} {
		if !strings.Contains(view, want) {
			t.Errorf("project picker missing %q", want)
		}
	}
	if strings.Contains(view, "Product polish") {
		t.Fatal("cross-team project leaked into picker")
	}
}

func TestProjectEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Title = "Canonical response"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("P"))
	m = updateKey(m, specialKey(tea.KeyUp))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.choiceEditor != nil || m.pendingEdit == nil || m.dashboard.Issues[0].Project == nil {
		t.Fatalf("optimistic project state = editor=%#v pending=%#v project=%#v cmd=%v", m.choiceEditor, m.pendingEdit, m.dashboard.Issues[0].Project, cmd != nil)
	}
	optimisticID := m.dashboard.Issues[0].Project.ID
	canonical.Project = &linear.Project{ID: optimisticID, Name: "Canonical project"}
	updater.issue = canonical

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.ProjectID == nil || *updater.update.ProjectID == nil || **updater.update.ProjectID != optimisticID || updater.update.Title != nil || updater.update.StateID != nil || updater.update.Priority != nil || updater.update.AssigneeID != nil {
		t.Fatalf("project mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Project.Name != "Canonical project" || m.dashboard.Issues[0].Title != canonical.Title {
		t.Fatalf("confirmed project state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedClearProjectRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("project forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("P"))
	m = updateKey(m, specialKey(tea.KeyHome))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if m.dashboard.Issues[0].Project != nil {
		t.Fatal("issue project was not optimistically cleared")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Project == nil || original.Project == nil || *issue.Project != *original.Project || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("project rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "project forbidden") {
		t.Fatalf("project error = %v", m.editErr)
	}
}

func TestOpenProjectEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("P"))
	m = updateKey(m, specialKey(tea.KeyEnd))
	selectedID := m.choiceEditor.options[m.choiceEditor.selected].project.ID

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.TeamProjects = append([]linear.TeamProjects(nil), dashboard.TeamProjects...)
	fresh.Issues[0].Description = "Fresh description"
	fresh.Issues[0].Project = nil
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.choiceEditor == nil || m.choiceEditor.options[m.choiceEditor.selected].project.ID != selectedID {
		t.Fatal("refresh discarded the selected project")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Project != nil || issue.Description != "Fresh description" {
		t.Fatalf("refreshed project rollback baseline = %#v", issue)
	}
}

func TestProjectEditorUnchangedSelectionDoesNotMutate(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("P"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.choiceEditor != nil || m.pendingEdit != nil || updater.calls != 0 {
		t.Fatalf("unchanged submit = editor=%#v pending=%#v calls=%d cmd=%v", m.choiceEditor, m.pendingEdit, updater.calls, cmd != nil)
	}
}

func TestProjectEditorNeedsTeamProjectMetadata(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.TeamProjects = nil
	dashboard.Issues[0].Project = nil
	for index := range dashboard.Issues {
		if dashboard.Issues[index].Team.ID == dashboard.Issues[0].Team.ID {
			dashboard.Issues[index].Project = nil
		}
	}
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("P"))
	if m.choiceEditor != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no projects") {
		t.Fatalf("missing projects = editor=%#v error=%v", m.choiceEditor, m.editErr)
	}
}

func TestProjectKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("P"))
		if m.choiceEditor != nil {
			t.Fatal("project picker opened through another modal mode")
		}
	}
}
