package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type issueActionsStub struct {
	issueUpdaterStub
	archiveErr error
	archives   int
	archivedID string
}

func (s *issueActionsStub) ArchiveIssue(_ context.Context, issueID string) error {
	s.archives++
	s.archivedID = issueID
	return s.archiveErr
}

func TestArchiveConfirmationNamesIssueAndDefaultsToCancel(t *testing.T) {
	dashboard := editorDashboard(t)
	actions := &issueActionsStub{}
	m := NewWithDashboardAndUpdater(dashboard, actions)
	m = updateKey(m, textKey("x"))
	if m.archiveConfirm == nil || m.archiveConfirm.selected != 0 {
		t.Fatalf("archive confirmation = %#v", m.archiveConfirm)
	}
	view := m.View().Content
	for _, want := range []string{"Archive " + dashboard.Issues[0].Identifier + "?", dashboard.Issues[0].Title, "Cancel", "recoverable"} {
		if !strings.Contains(view, want) {
			t.Errorf("archive confirmation missing %q", want)
		}
	}
	m = updateKey(m, textKey("x"))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd != nil || m.archiveConfirm != nil || m.pendingArchive != nil || actions.archives != 0 || len(m.dashboard.Issues) != len(dashboard.Issues) {
		t.Fatalf("default cancel = confirmation=%#v pending=%#v archives=%d cmd=%v", m.archiveConfirm, m.pendingArchive, actions.archives, cmd != nil)
	}
}

func TestArchiveKeyboardFlowRemovesConfirmedIssueOnce(t *testing.T) {
	dashboard := editorDashboard(t)
	actions := &issueActionsStub{}
	m := NewWithDashboardAndUpdater(dashboard, actions)
	target := dashboard.Issues[0]
	m = updateKey(m, textKey("x"))
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.archiveConfirm != nil || m.pendingArchive == nil || len(m.dashboard.Issues) != len(dashboard.Issues) {
		t.Fatalf("archive submit = confirmation=%#v pending=%#v issues=%d cmd=%v", m.archiveConfirm, m.pendingArchive, len(m.dashboard.Issues), cmd != nil)
	}
	m = updateKey(m, textKey("x"))
	if m.archiveConfirm != nil {
		t.Fatal("repeated archive key opened another confirmation while pending")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if actions.archives != 1 || actions.archivedID != target.ID || m.pendingArchive != nil || len(m.dashboard.Issues) != len(dashboard.Issues)-1 {
		t.Fatalf("confirmed archive = calls=%d id=%q pending=%#v issues=%d", actions.archives, actions.archivedID, m.pendingArchive, len(m.dashboard.Issues))
	}
	for _, issue := range m.dashboard.Issues {
		if issue.ID == target.ID {
			t.Fatal("archived issue remains in dashboard")
		}
	}
}

func TestArchiveConfirmationKeepsOriginalTargetWhenSelectionChanges(t *testing.T) {
	dashboard := editorDashboard(t)
	actions := &issueActionsStub{}
	m := NewWithDashboardAndUpdater(dashboard, actions)
	targetID := dashboard.Issues[0].ID
	m = updateKey(m, textKey("x"))
	m.selected = 1
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if actions.archivedID != targetID {
		t.Fatalf("archive targeted %q, want original %q", actions.archivedID, targetID)
	}
}

func TestFailedArchiveKeepsIssueAndShowsError(t *testing.T) {
	dashboard := editorDashboard(t)
	actions := &issueActionsStub{archiveErr: errors.New("archive forbidden")}
	m := NewWithDashboardAndUpdater(dashboard, actions)
	m = updateKey(m, textKey("x"))
	m = updateKey(m, specialKey(tea.KeyDown))
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.pendingArchive != nil || len(m.dashboard.Issues) != len(dashboard.Issues) || m.editErr == nil || !strings.Contains(m.View().Content, "archive forbidden") {
		t.Fatalf("failed archive = pending=%#v issues=%d error=%v", m.pendingArchive, len(m.dashboard.Issues), m.editErr)
	}
}

func TestArchiveConfirmationClosesWhenRefreshRemovesTarget(t *testing.T) {
	dashboard := editorDashboard(t)
	actions := &issueActionsStub{}
	m := NewWithDashboardAndUpdater(dashboard, actions)
	m = updateKey(m, textKey("x"))
	updated, _ := m.Update(dashboardLoadedMsg{dashboard: dashboardWithoutIssue(dashboard, m.archiveConfirm.issueID)})
	m = updated.(Model)
	if m.archiveConfirm != nil || m.editErr == nil || !strings.Contains(m.editErr.Error(), "no longer available") {
		t.Fatalf("stale archive confirmation = %#v error=%v", m.archiveConfirm, m.editErr)
	}
}

func TestActionMenuExposesArchiveOnlyWhenAvailable(t *testing.T) {
	dashboard := editorDashboard(t)
	m := NewWithDashboardAndUpdater(dashboard, &issueActionsStub{})
	m = updateKey(m, specialKey(tea.KeyEnter))
	if m.actionMenu == nil || !m.actionMenu.has(actionArchiveIssue) || !strings.Contains(m.View().Content, "Archive issue") || !strings.Contains(m.View().Content, "[x]") {
		t.Fatalf("archive action menu = %#v", m.actionMenu)
	}
}
