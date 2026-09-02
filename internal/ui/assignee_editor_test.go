package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestAssigneeEditorShowsWorkspaceMembersAndCurrentSelection(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("u"))
	if m.choiceEditor == nil || m.choiceEditor.kind != editAssignee || len(m.choiceEditor.options) != len(dashboard.Users)+1 {
		t.Fatalf("assignee editor = %#v", m.choiceEditor)
	}
	selected := m.choiceEditor.options[m.choiceEditor.selected].assignee
	if selected == nil || selected.ID != dashboard.Issues[0].Assignee.ID {
		t.Fatalf("selected assignee = %#v", selected)
	}
	view := m.View().Content
	for _, want := range []string{"Edit assignee", "Unassigned", "Aisha", "Jamie", "Marcus"} {
		if !strings.Contains(view, want) {
			t.Errorf("assignee picker missing %q", want)
		}
	}
}

func TestAssigneeEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Assignee = &dashboard.Users[0]
	canonical.Title = "Canonical response"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("u"))
	m = updateKey(m, specialKey(tea.KeyDown))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.choiceEditor != nil || m.pendingEdit == nil || m.dashboard.Issues[0].Assignee == nil {
		t.Fatalf("optimistic assignee state = editor=%#v pending=%#v assignee=%#v cmd=%v", m.choiceEditor, m.pendingEdit, m.dashboard.Issues[0].Assignee, cmd != nil)
	}
	optimisticID := m.dashboard.Issues[0].Assignee.ID

	canonical.Assignee = &linear.User{ID: optimisticID, Name: "Canonical user", DisplayName: "Canonical"}
	updater.issue = canonical
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.AssigneeID == nil || *updater.update.AssigneeID == nil || **updater.update.AssigneeID != optimisticID || updater.update.Title != nil || updater.update.StateID != nil || updater.update.Priority != nil {
		t.Fatalf("assignee mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Assignee.Label() != "Canonical" || m.dashboard.Issues[0].Title != canonical.Title {
		t.Fatalf("confirmed assignee state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedUnassignRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("assignee forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("u"))
	m = updateKey(m, specialKey(tea.KeyHome))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if m.dashboard.Issues[0].Assignee != nil {
		t.Fatal("issue was not optimistically unassigned")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Assignee == nil || original.Assignee == nil || *issue.Assignee != *original.Assignee || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("assignee rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "assignee forbidden") {
		t.Fatalf("assignee error = %v", m.editErr)
	}
}

func TestOpenAssigneeEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("u"))
	m = updateKey(m, specialKey(tea.KeyEnd))
	selectedID := m.choiceEditor.options[m.choiceEditor.selected].assignee.ID

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Users = append([]linear.User(nil), dashboard.Users...)
	fresh.Issues[0].Description = "Fresh description"
	fresh.Issues[0].Assignee = &fresh.Users[0]
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.choiceEditor == nil || m.choiceEditor.options[m.choiceEditor.selected].assignee.ID != selectedID {
		t.Fatal("refresh discarded the selected assignee")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Assignee == nil || issue.Assignee.ID != fresh.Users[0].ID || issue.Description != "Fresh description" {
		t.Fatalf("refreshed assignee rollback baseline = %#v", issue)
	}
}

func TestAssigneeEditorUnchangedSelectionDoesNotMutate(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("u"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.choiceEditor != nil || m.pendingEdit != nil || updater.calls != 0 {
		t.Fatalf("unchanged submit = editor=%#v pending=%#v calls=%d cmd=%v", m.choiceEditor, m.pendingEdit, updater.calls, cmd != nil)
	}
}

func TestAssigneeEditorRequiresWorkspaceMemberMetadata(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.Users = nil
	dashboard.Issues[0].Assignee = nil
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("u"))
	if m.choiceEditor != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no workspace members") {
		t.Fatalf("missing members = editor=%#v error=%v", m.choiceEditor, m.editErr)
	}
}

func TestAssigneeKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("u"))
		if m.choiceEditor != nil {
			t.Fatal("assignee picker opened through another modal mode")
		}
	}
}
