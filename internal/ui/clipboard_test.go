package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func runClipboardCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("clipboard shortcut returned no command")
	}
	msg := cmd()
	updated, followUp := m.Update(msg)
	if followUp != nil {
		_ = followUp()
	}
	return updated.(Model)
}

func TestUppercaseGCopiesSelectedIssueBranch(t *testing.T) {
	dashboard := browserDashboard(t)
	got := ""
	m := NewWithDashboardAndClipboard(dashboard, func(value string) tea.Cmd {
		got = value
		return nil
	})
	m.selected = 1
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', ShiftedCode: 'G', Mod: tea.ModShift}))
	m = runClipboardCmd(t, updated.(Model), cmd)
	if got != dashboard.Issues[1].BranchName {
		t.Fatalf("copied branch = %q, want %q", got, dashboard.Issues[1].BranchName)
	}
	if m.selected != 1 {
		t.Fatalf("uppercase G moved selection to %d", m.selected)
	}
	if !strings.Contains(m.View().Content, "Copied git branch for "+dashboard.Issues[1].Identifier) {
		t.Fatal("successful branch copy was not visible")
	}
}

func TestLowercaseCCopiesSelectedIssueURL(t *testing.T) {
	dashboard := browserDashboard(t)
	got := ""
	m := NewWithDashboardAndClipboard(dashboard, func(value string) tea.Cmd {
		got = value
		return nil
	})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	m = runClipboardCmd(t, updated.(Model), cmd)
	if got != dashboard.Issues[0].URL {
		t.Fatalf("copied URL = %q, want %q", got, dashboard.Issues[0].URL)
	}
	if !strings.Contains(m.View().Content, "Copied issue URL for "+dashboard.Issues[0].Identifier) {
		t.Fatal("successful URL copy was not visible")
	}
}

func TestClipboardShortcutsShowEdgeFailures(t *testing.T) {
	dashboard := browserDashboard(t)
	dashboard.Issues[0].BranchName = ""
	calls := 0
	m := NewWithDashboardAndClipboard(dashboard, func(string) tea.Cmd {
		calls++
		return nil
	})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', ShiftedCode: 'G', Mod: tea.ModShift}))
	m = runClipboardCmd(t, updated.(Model), cmd)
	if calls != 0 || m.clipboardErr == nil || !strings.Contains(m.clipboardErr.Error(), "no git branch") {
		t.Fatalf("missing branch state = calls=%d error=%v", calls, m.clipboardErr)
	}
	if !strings.Contains(m.View().Content, "Clipboard:") {
		t.Fatal("clipboard failure was not visible")
	}

	m.issues = nil
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	m = runClipboardCmd(t, updated.(Model), cmd)
	if calls != 0 || m.clipboardErr == nil || !strings.Contains(m.clipboardErr.Error(), "no issue is selected") {
		t.Fatalf("missing issue state = calls=%d error=%v", calls, m.clipboardErr)
	}
}

func TestEndStillSelectsLastIssue(t *testing.T) {
	dashboard := browserDashboard(t)
	m := NewWithDashboard(dashboard)
	m.selected = 0
	m = updateKey(m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	if m.selected != len(m.issues)-1 {
		t.Fatalf("end selected %d, want %d", m.selected, len(m.issues)-1)
	}
}

func TestCtrlCDoesNotCopyIssueURL(t *testing.T) {
	dashboard := browserDashboard(t)
	called := false
	m := NewWithDashboardAndClipboard(dashboard, func(string) tea.Cmd {
		called = true
		return nil
	})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil || called {
		t.Fatalf("ctrl+c returned cmd=%v and called clipboard=%v", cmd != nil, called)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command returned %T, want tea.QuitMsg", cmd())
	}
	if updated.(Model).clipboardNotice != "" || updated.(Model).clipboardErr != nil {
		t.Fatal("ctrl+c changed clipboard feedback")
	}
}
