package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestEnterOpensIssueActionMenuWithAvailableActions(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, specialKey(tea.KeyEnter))
	if m.actionMenu == nil || len(m.actionMenu.options) != 8 {
		t.Fatalf("action menu = %#v", m.actionMenu)
	}
	view := m.View().Content
	for _, want := range []string{
		"Issue actions", dashboard.Issues[0].Identifier, "Edit title", "Change status",
		"Change priority", "Change assignee", "Change project", "Edit labels", "Edit description", "Open in Linear", "[e]", "[s]", "[p]", "[u]", "[P]", "[l]", "[d]", "[space]",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("action menu missing %q", want)
		}
	}
}

func TestActionMenuSelectionOpensStatusEditor(t *testing.T) {
	m := NewWithDashboardAndUpdater(editorDashboard(t), &issueUpdaterStub{})
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.actionMenu != nil || m.choiceEditor == nil || m.choiceEditor.kind != editStatus {
		t.Fatalf("selected action = menu=%#v choice=%#v cmd=%v", m.actionMenu, m.choiceEditor, cmd != nil)
	}
}

func TestActionMenuQuickKeyOpensPriorityEditor(t *testing.T) {
	m := NewWithDashboardAndUpdater(editorDashboard(t), &issueUpdaterStub{})
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, textKey("p"))
	if m.actionMenu != nil || m.choiceEditor == nil || m.choiceEditor.kind != editPriority {
		t.Fatalf("priority quick action = menu=%#v choice=%#v", m.actionMenu, m.choiceEditor)
	}
}

func TestReadOnlyActionMenuShowsOnlyBrowserAction(t *testing.T) {
	dashboard := editorDashboard(t)
	opened := ""
	m := NewWithDashboardAndBrowser(dashboard, func(rawURL string) error {
		opened = rawURL
		return nil
	})
	m = updateKey(m, specialKey(tea.KeyEnter))
	if m.actionMenu == nil || len(m.actionMenu.options) != 1 || m.actionMenu.options[0].action != actionOpenBrowser {
		t.Fatalf("read-only action menu = %#v", m.actionMenu)
	}
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.actionMenu != nil {
		t.Fatalf("browser action = menu=%#v cmd=%v", m.actionMenu, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if opened != dashboard.Issues[0].URL || m.browserErr != nil {
		t.Fatalf("opened URL = %q, error=%v", opened, m.browserErr)
	}
}

func TestActionMenuClosesWhenRefreshRemovesIssue(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
	m = updateKey(m, specialKey(tea.KeyEnter))
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: dashboardWithoutIssue(dashboard, m.actionMenu.issueID)})
	m = updated.(Model)
	if m.actionMenu != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no longer available") {
		t.Fatalf("removed issue menu = %#v, error=%v", m.actionMenu, m.editErr)
	}
}

func TestActionMenuEnterIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := editorDashboard(t)
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndUpdater(dashboard, &issueUpdaterStub{})
		configure(&m)
		m = updateKey(m, specialKey(tea.KeyEnter))
		if m.actionMenu != nil {
			t.Fatal("action menu opened through another modal mode")
		}
	}
}

func dashboardWithoutIssue(dashboard linear.Dashboard, issueID string) linear.Dashboard {
	dashboard.Issues = append([]linear.Issue(nil), dashboard.Issues...)
	filtered := dashboard.Issues[:0]
	for _, issue := range dashboard.Issues {
		if issue.ID != issueID {
			filtered = append(filtered, issue)
		}
	}
	dashboard.Issues = filtered
	return dashboard
}
