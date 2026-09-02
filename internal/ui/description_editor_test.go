package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestDescriptionEditorOpensAndCancelsWithoutMutation(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	if m.descriptionEditor == nil || m.descriptionEditor.issueID != dashboard.Issues[0].ID {
		t.Fatalf("description editor = %#v", m.descriptionEditor)
	}
	view := m.View().Content
	if !strings.Contains(view, "Edit description") || !strings.Contains(view, dashboard.Issues[0].Identifier) || !strings.Contains(view, "Ctrl+S saves") {
		t.Fatal("description editor is not visible")
	}
	m = updateKey(m, textKey("esc"))
	if m.descriptionEditor != nil || updater.calls != 0 {
		t.Fatalf("cancel changed state: editor=%#v calls=%d", m.descriptionEditor, updater.calls)
	}
}

func TestDescriptionEditIsMultilineOptimisticAndCanonical(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Title = "Canonical response"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, textKey("# Heading"))
	m = updateKey(m, textKey("enter"))
	m = updateKey(m, textKey("Details"))

	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	want := "# Heading\nDetails"
	if cmd == nil || m.descriptionEditor != nil || m.pendingEdit == nil || m.dashboard.Issues[0].Description != want {
		t.Fatalf("optimistic description = editor=%#v pending=%#v value=%q cmd=%v", m.descriptionEditor, m.pendingEdit, m.dashboard.Issues[0].Description, cmd != nil)
	}
	canonical.Description = "Canonical markdown"
	updater.issue = canonical
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.Description == nil || *updater.update.Description != want || updater.update.Title != nil || updater.update.StateID != nil || updater.update.Priority != nil || updater.update.AssigneeID != nil || updater.update.ProjectID != nil || updater.update.LabelIDs != nil {
		t.Fatalf("description mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Description != canonical.Description || m.dashboard.Issues[0].Title != canonical.Title {
		t.Fatalf("confirmed description state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedClearDescriptionRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("description forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	m = updateKey(m, textKey("ctrl+u"))
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	if m.dashboard.Issues[0].Description != "" {
		t.Fatalf("description was not optimistically cleared: %q", m.dashboard.Issues[0].Description)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Description != original.Description || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("description rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "description forbidden") {
		t.Fatalf("description error = %v", m.editErr)
	}
}

func TestOpenDescriptionEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	m = updateKey(m, textKey("ctrl+u"))
	m = updateKey(m, textKey("Draft survives refresh"))

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Issues[0].Description = "Fresh server description"
	fresh.Issues[0].Priority = 4
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.descriptionEditor == nil || string(m.descriptionEditor.value) != "Draft survives refresh" {
		t.Fatal("refresh discarded the description draft")
	}
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if issue.Description != fresh.Issues[0].Description || issue.Priority != 4 {
		t.Fatalf("refreshed description rollback baseline = %#v", issue)
	}
}

func TestDescriptionEditorUnchangedDoesNotMutate(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	if cmd != nil || m.descriptionEditor != nil || m.pendingEdit != nil || updater.calls != 0 {
		t.Fatalf("unchanged submit = editor=%#v pending=%#v calls=%d cmd=%v", m.descriptionEditor, m.pendingEdit, updater.calls, cmd != nil)
	}
}

func TestDescriptionEditorPreservesMarkdownSource(t *testing.T) {
	dashboard := editorDashboard(t)
	source := "# Heading\n\n- **Keep** this [link](https://linear.app)\n"
	dashboard.Issues[0].Description = source
	updater := &issueUpdaterStub{issue: dashboard.Issues[0]}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("d"))
	if got := string(m.descriptionEditor.value); got != source {
		t.Fatalf("editor source = %q, want %q", got, source)
	}
	m = updateKey(m, textKey("!"))
	updated, cmd := m.Update(textKey("ctrl+s"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("save command is nil")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if got := *updater.update.Description; got != source+"!" {
		t.Fatalf("submitted description = %q, want %q", got, source+"!")
	}
}

func TestDescriptionEditorCursorAndViewport(t *testing.T) {
	value := []rune("one\ntwo\nthree")
	if got := verticalCursor(value, 6, -1); got != 2 {
		t.Fatalf("cursor up = %d, want 2", got)
	}
	if got := verticalCursor(value, 2, 1); got != 6 {
		t.Fatalf("cursor down = %d, want 6", got)
	}
	view := strings.Join(descriptionViewport([]rune("123456789"), 8, 4, 2), "\n")
	if !strings.Contains(view, "│") {
		t.Fatalf("viewport lost cursor: %q", view)
	}
}

func TestDescriptionKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("d"))
		if m.descriptionEditor != nil {
			t.Fatal("description editor opened through another modal mode")
		}
	}
}
