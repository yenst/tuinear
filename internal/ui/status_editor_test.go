package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

func TestStatusEditorUsesOnlySelectedIssuesTeamStates(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("s"))
	if m.choiceEditor == nil || len(m.choiceEditor.options) != 4 {
		t.Fatalf("status editor = %#v", m.choiceEditor)
	}
	for _, choice := range m.choiceEditor.options {
		if !strings.HasPrefix(choice.state.ID, "platform-") {
			t.Fatalf("cross-team state leaked into picker: %#v", choice.state)
		}
	}
	view := m.View().Content
	for _, want := range []string{"Edit status", "Backlog", "Todo", "In Progress", "Done"} {
		if !strings.Contains(view, want) {
			t.Errorf("status picker missing %q", want)
		}
	}
}

func TestStatusEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.State = dashboard.StatesForTeam(canonical.Team.ID)[3]
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("s"))
	m = updateKey(m, specialKey(tea.KeyDown))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.choiceEditor != nil || m.pendingEdit == nil {
		t.Fatalf("submit state = editor=%#v pending=%#v cmd=%v", m.choiceEditor, m.pendingEdit, cmd != nil)
	}
	if got := m.dashboard.Issues[0].State; got.ID != "platform-done" {
		t.Fatalf("optimistic state = %#v", got)
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.StateID == nil || *updater.update.StateID != "platform-done" || updater.update.Title != nil || updater.update.Priority != nil {
		t.Fatalf("status mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].State.ID != canonical.State.ID {
		t.Fatalf("confirmed state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedStatusEditRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("status forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("s"))
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.State != original.State || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("status rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "status forbidden") {
		t.Fatalf("status error = %v", m.editErr)
	}
}

func TestOpenStatusEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("s"))
	m = updateKey(m, specialKey(tea.KeyDown))

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Issues[0].Description = "Fresh description"
	fresh.Issues[0].State = fresh.StatesForTeam(fresh.Issues[0].Team.ID)[1]
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.choiceEditor == nil || m.choiceEditor.options[m.choiceEditor.selected].state.ID != "platform-done" {
		t.Fatal("refresh discarded the selected status choice")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.State.ID != "platform-todo" || issue.Description != "Fresh description" {
		t.Fatalf("refreshed rollback baseline = %#v", issue)
	}
}

func TestStatusEditorNeedsStableWorkflowStateIDs(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.TeamStates = nil
	for index := range dashboard.Issues {
		dashboard.Issues[index].State.ID = ""
	}
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("s"))
	if m.choiceEditor != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no editable statuses") {
		t.Fatalf("missing state metadata = editor=%#v error=%v", m.choiceEditor, m.editErr)
	}
}

func TestStatusKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("s"))
		if m.choiceEditor != nil {
			t.Fatal("status picker opened through another modal mode")
		}
	}
}
