package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

func TestDashboardRequestsDoNotOverlap(t *testing.T) {
	for _, initialKey := range []string{"r", "a"} {
		t.Run(initialKey, func(t *testing.T) {
			m := New(linear.DemoClient{})
			m.applyDashboard(editorDashboard(t))
			updated, cmd := m.Update(textKey(initialKey))
			m = updated.(Model)
			if cmd == nil {
				t.Fatal("initial dashboard request did not start")
			}
			for _, key := range []string{"r", "a", "A"} {
				_, next := m.Update(textKey(key))
				if next != nil {
					t.Errorf("%q started a request while %q was pending", key, initialKey)
				}
			}
			updated, _ = m.Update(cmd())
			_, next := updated.(Model).Update(textKey("r"))
			if next == nil {
				t.Fatal("refresh stayed disabled after request completed")
			}
		})
	}
}

func TestAccountSwitchBlocksActionsOnPreviousDashboard(t *testing.T) {
	for _, key := range []string{"e", "s", "p", "u", "P", "l", "d", "x", "n", "enter", "f"} {
		t.Run(key, func(t *testing.T) {
			m := New(linear.DemoClient{})
			m.applyDashboard(editorDashboard(t))
			m = updateKey(m, textKey("tab"))
			updated, switchCmd := m.Update(textKey("a"))
			m = updated.(Model)
			if switchCmd == nil || !m.loading {
				t.Fatal("account switch did not start")
			}
			updated, cmd := m.Update(textKey(key))
			m = updated.(Model)
			if cmd != nil || m.editor != nil || m.choiceEditor != nil || m.labelEditor != nil ||
				m.descriptionEditor != nil || m.archiveConfirm != nil || m.createEditor != nil || m.actionMenu != nil || m.palette {
				t.Fatal("action opened against the previous account during switching")
			}
		})
	}
}

func TestFailedAccountSwitchKeepsPreviousDashboardUsable(t *testing.T) {
	m := New(linear.DemoClient{})
	m.applyDashboard(editorDashboard(t))
	previousID := m.dashboard.ActiveAccountID
	m = updateKey(m, textKey("a"))
	updated, _ := m.Update(dashboardFailedMsg{err: errors.New("account unavailable")})
	m = updated.(Model)
	if m.loading || m.err != nil || m.refreshErr == nil || m.dashboard.ActiveAccountID != previousID {
		t.Fatalf("failed switch lost usable dashboard: loading=%v err=%v refresh=%v", m.loading, m.err, m.refreshErr)
	}
}

func TestFilterPaletteHandlesOptionsRemovedByRefresh(t *testing.T) {
	for _, key := range []string{"enter", "!"} {
		t.Run(key, func(t *testing.T) {
			m := NewWithDashboard(editorDashboard(t))
			m = updateKey(m, textKey("f"))
			m = updateKey(m, textKey("end"))
			updated, _ := m.Update(dashboardLoadedMsg{dashboard: linear.Dashboard{}})
			m = updated.(Model)
			m = updateKey(m, textKey(key))
			if m.paletteIdx < 0 || m.paletteIdx >= len(m.filterOptions()) {
				t.Fatalf("palette selection out of bounds: %d", m.paletteIdx)
			}
		})
	}
}

func TestQuitRemainsAvailableWhileLoading(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		_, cmd := New(linear.DemoClient{}).Update(textKey(key))
		if cmd == nil {
			t.Fatalf("%q did not quit during loading", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%q returned a non-quit command", key)
		}
	}
}
