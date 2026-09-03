package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

func TestLabelEditorShowsWorkspaceAndTeamLabels(t *testing.T) {
	dashboard := editorDashboard(t)
	issue := dashboard.Issues[0]
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("l"))
	if m.labelEditor == nil || len(m.labelEditor.options) != len(dashboard.LabelsForTeam(issue.Team.ID)) {
		t.Fatalf("label editor = %#v", m.labelEditor)
	}
	if !m.labelEditor.checked[issue.Labels[0].ID] {
		t.Fatalf("current labels are not selected: %#v", m.labelEditor.checked)
	}
	view := m.View().Content
	for _, want := range []string{"Edit labels", "platform", "quality", "testing", "[x]"} {
		if !strings.Contains(view, want) {
			t.Errorf("label editor missing %q", want)
		}
	}
	if strings.Contains(view, "product") {
		t.Fatal("cross-team label leaked into editor")
	}
}

func TestLabelEditIsOptimisticAndUsesCanonicalResponse(t *testing.T) {
	dashboard := editorDashboard(t)
	canonical := dashboard.Issues[0]
	canonical.Title = "Canonical response"
	updater := &issueUpdaterStub{issue: canonical}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("l"))
	selected := m.labelEditor.options[0]
	m = updateKey(m, textKey("space"))

	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.labelEditor != nil || m.pendingEdit == nil || !hasLabel(m.dashboard.Issues[0].Labels, selected.ID) {
		t.Fatalf("optimistic labels = editor=%#v pending=%#v labels=%#v cmd=%v", m.labelEditor, m.pendingEdit, m.dashboard.Issues[0].Labels, cmd != nil)
	}
	canonical.Labels = append([]linear.Label(nil), m.dashboard.Issues[0].Labels...)
	canonical.Labels[0].Name = "Canonical label"
	updater.issue = canonical

	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if updater.calls != 1 || updater.update.LabelIDs == nil || !hasString(*updater.update.LabelIDs, selected.ID) || updater.update.Title != nil || updater.update.StateID != nil || updater.update.Priority != nil || updater.update.AssigneeID != nil || updater.update.ProjectID != nil {
		t.Fatalf("label mutation = calls=%d update=%#v", updater.calls, updater.update)
	}
	if m.pendingEdit != nil || m.editErr != nil || m.dashboard.Issues[0].Title != canonical.Title || m.dashboard.Issues[0].Labels[0].Name != "Canonical label" {
		t.Fatalf("confirmed label state = pending=%#v error=%v issue=%#v", m.pendingEdit, m.editErr, m.dashboard.Issues[0])
	}
}

func TestFailedClearLabelsRollsBackExactly(t *testing.T) {
	dashboard := editorDashboard(t)
	original := dashboard.Issues[0]
	updater := &issueUpdaterStub{err: errors.New("labels forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("l"))
	for !m.labelEditor.checked[m.labelEditor.options[m.labelEditor.selected].ID] {
		m = updateKey(m, specialKey(tea.KeyDown))
	}
	m = updateKey(m, textKey("space"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if len(m.dashboard.Issues[0].Labels) != 0 {
		t.Fatalf("labels were not optimistically cleared: %#v", m.dashboard.Issues[0].Labels)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if !sameLabelSet(issue.Labels, original.Labels) || !issue.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("label rollback = %#v, want %#v", issue, original)
	}
	if m.editErr == nil || !strings.Contains(m.View().Content, "labels forbidden") {
		t.Fatalf("label error = %v", m.editErr)
	}
}

func TestOpenLabelEditorRebasesOverBackgroundRefresh(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{err: errors.New("save failed")}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("l"))
	m = updateKey(m, specialKey(tea.KeyEnd))
	selectedID := m.labelEditor.options[m.labelEditor.selected].ID
	m = updateKey(m, textKey("space"))

	fresh := dashboard
	fresh.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	fresh.Issues[0].Description = "Fresh description"
	fresh.Issues[0].Labels = []linear.Label{dashboard.LabelsForTeam(fresh.Issues[0].Team.ID)[0]}
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: fresh})
	m = updated.(Model)
	if m.labelEditor == nil || m.labelEditor.options[m.labelEditor.selected].ID != selectedID || !m.labelEditor.checked[selectedID] {
		t.Fatal("refresh discarded the selected label state")
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	issue := m.dashboard.Issues[0]
	if !sameLabelSet(issue.Labels, fresh.Issues[0].Labels) || issue.Description != "Fresh description" {
		t.Fatalf("refreshed label rollback baseline = %#v", issue)
	}
}

func TestLabelEditorUnchangedSelectionDoesNotMutate(t *testing.T) {
	dashboard := editorDashboard(t)
	updater := &issueUpdaterStub{}
	m := NewWithDashboardAndUpdater(dashboard, updater)
	m = updateKey(m, textKey("l"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.labelEditor != nil || m.pendingEdit != nil || updater.calls != 0 {
		t.Fatalf("unchanged submit = editor=%#v pending=%#v calls=%d cmd=%v", m.labelEditor, m.pendingEdit, updater.calls, cmd != nil)
	}
}

func TestLabelEditorNeedsMetadata(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.WorkspaceLabels = nil
	dashboard.TeamLabels = nil
	for index := range dashboard.Issues {
		if dashboard.Issues[index].Team.ID == dashboard.Issues[0].Team.ID {
			dashboard.Issues[index].Labels = nil
		}
	}
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, textKey("l"))
	if m.labelEditor != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no labels") {
		t.Fatalf("missing labels = editor=%#v error=%v", m.labelEditor, m.editErr)
	}
}

func TestLabelKeyIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, textKey("l"))
		if m.labelEditor != nil {
			t.Fatal("label editor opened through another modal mode")
		}
	}
}

func hasLabel(labels []linear.Label, id string) bool {
	for _, label := range labels {
		if label.ID == id {
			return true
		}
	}
	return false
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
