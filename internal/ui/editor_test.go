package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

type issueUpdaterStub struct {
	issue   linear.Issue
	err     error
	calls   int
	issueID string
	update  linear.IssueUpdate
}

func (s *issueUpdaterStub) UpdateIssue(_ context.Context, issueID string, update linear.IssueUpdate) (linear.Issue, error) {
	s.calls++
	s.issueID = issueID
	s.update = update
	return s.issue, s.err
}

func editorDashboard(t *testing.T) linear.Dashboard {
	t.Helper()
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return dashboard
}

func TestTitleEditorOpensAndCancelsWithoutMutation(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("e"))
	if m.editor == nil || m.editor.issueID != dashboard.Issues[0].ID {
		t.Fatalf("editor = %#v", m.editor)
	}
	view := m.View().Content
	if !strings.Contains(view, "Edit title") || !strings.Contains(view, dashboard.Issues[0].Identifier) {
		t.Fatal("title editor is not visible")
	}
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.editor != nil || updater.calls != 0 || m.dashboard.Issues[0].Title != dashboard.Issues[0].Title {
		t.Fatalf("cancel changed state: editor=%#v calls=%d", m.editor, updater.calls)
	}
}

func TestTitleEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Title = "Canonical title from Linear"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("e"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, textKey("Optimistic title"))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.editor != nil || m.pendingEdit == nil {
		t.Fatalf("submit state = editor=%#v pending=%#v cmd=%v", m.editor, m.pendingEdit, cmd != nil)
	}
	if got := m.dashboard.Issues[0].Title; got != "Optimistic title" {
		t.Fatalf("optimistic title = %q", got)
	}
	m = updateKey(m, textKey("e"))
	if m.editor != nil || m.editErr == nil {
		t.Fatal("a second edit was allowed while saving")
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.issueID != dashboard.Issues[0].ID || updater.update.Title == nil || *updater.update.Title != "Optimistic title" {
		t.Fatalf("mutation = calls=%d id=%q update=%#v", updater.calls, updater.issueID, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Title != canonical.Title {
		t.Fatalf("confirmed state = pending=%#v error=%v title=%q", m.pendingEdit, m.editErr, m.dashboard.Issues[0].Title)
	}
}

func TestFailedTitleEditRollsBackExactlyAndStaysVisible(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("permission denied")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("e"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, textKey("Will fail"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if got := m.dashboard.Issues[0]; got.Title != original.Title || !got.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("rollback issue = %#v, want %#v", got, original)
	}
	if m.pendingEdit != nil || m.editErr == nil || !strings.Contains(m.View().Content, "permission denied") {
		t.Fatalf("failed state = pending=%#v error=%v", m.pendingEdit, m.editErr)
	}
}

func TestPendingTitleEditRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("e"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, textKey("Optimistic title"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Issues[0].Title = "Fresh server title"
	fresh.Issues[0].Description = "Fresh server description"
	updated, _ = m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.dashboard.Issues[0].Title != "Optimistic title" {
		t.Fatal("background refresh hid the optimistic title")
	}

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Title != "Fresh server title" || issue.Description != "Fresh server description" {
		t.Fatalf("rollback did not use fresh baseline: %#v", issue)
	}
}

func TestTitleEditorRejectsEmptyTitleAndSupportsUnicodeCursor(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("e"))
	m = updateKey(m, textKey("ctrl+u"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.editor == nil || m.editor.err == nil || updater.calls != 0 {
		t.Fatalf("empty submit = cmd=%v editor=%#v calls=%d", cmd != nil, m.editor, updater.calls)
	}
	m = updateKey(m, textKey("é界"))
	m = updateKey(m, specialKey(tea.KeyLeft))
	if got := editorWindow(m.editor.value, m.editor.cursor, 10); got != "é│界" {
		t.Fatalf("unicode cursor window = %q", got)
	}
}

func TestTitleEditorAcceptsShiftedCharacters(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("e"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, tea.KeyPressMsg(tea.Key{Code: 'D', Text: "D", Mod: tea.ModShift}))
	if got := string(m.editor.value); got != "D" {
		t.Fatalf("shifted input = %q, want D", got)
	}
}

func TestEditKeyIsIsolatedFromSearchAndFilters(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, updater)
		configure(&m)
		m = updateKey(m, textKey("e"))
		if m.editor != nil || updater.calls != 0 {
			t.Fatal("edit opened through another modal mode")
		}
	}
}
