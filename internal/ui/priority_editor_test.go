package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestPriorityEditorShowsLinearPrioritiesAndSelectsCurrent(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("p"))
	if m.choiceEditor == nil || m.choiceEditor.kind != editPriority || len(m.choiceEditor.options) != 5 {
		t.Fatalf("priority editor = %#v", m.choiceEditor)
	}
	if got := m.choiceEditor.options[m.choiceEditor.selected].priority; got != dashboard.Issues[0].Priority {
		t.Fatalf("selected priority = %d, want %d", got, dashboard.Issues[0].Priority)
	}
	view := m.View().Content
	for _, want := range []string{"Edit priority", "No priority", "Urgent", "High", "Medium", "Low"} {
		if !strings.Contains(view, want) {
			t.Errorf("priority picker missing %q", want)
		}
	}
}

func TestPriorityEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Priority = 1
	canonical.Title = "Canonical response"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("p"))
	m = updateKey(m, specialKey(tea.KeyUp))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.choiceEditor != nil || m.pendingEdit == nil || m.dashboard.Issues[0].Priority != 1 {
		t.Fatalf("optimistic priority state = editor=%#v pending=%#v priority=%d cmd=%v", m.choiceEditor, m.pendingEdit, m.dashboard.Issues[0].Priority, cmd != nil)
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.Priority == nil || *updater.update.Priority != 1 || updater.update.Title != nil || updater.update.StateID != nil {
		t.Fatalf("priority mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Priority != 1 || m.dashboard.Issues[0].Title != canonical.Title {
		t.Fatalf("confirmed priority state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedPriorityEditRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("priority forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("p"))
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if m.dashboard.Issues[0].Priority == original.Priority {
		t.Fatal("priority did not update optimistically")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Priority != original.Priority || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("priority rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "priority forbidden") {
		t.Fatalf("priority error = %v", m.editErr)
	}
}

func TestOpenPriorityEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("p"))
	m = updateKey(m, specialKey(tea.KeyDown))

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Issues[0].Description = "Fresh description"
	fresh.Issues[0].Priority = 4
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.choiceEditor == nil || m.choiceEditor.options[m.choiceEditor.selected].priority != 3 {
		t.Fatal("refresh discarded the selected priority choice")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Priority != 4 || issue.Description != "Fresh description" {
		t.Fatalf("refreshed priority rollback baseline = %#v", issue)
	}
}

func TestPriorityEditorUnchangedSelectionDoesNotMutate(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("p"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.choiceEditor != nil || m.pendingEdit != nil || updater.calls != 0 {
		t.Fatalf("unchanged submit = editor=%#v pending=%#v calls=%d cmd=%v", m.choiceEditor, m.pendingEdit, updater.calls, cmd != nil)
	}
}

func TestPriorityKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("p"))
		if m.choiceEditor != nil {
			t.Fatal("priority picker opened through another modal mode")
		}
	}
}
