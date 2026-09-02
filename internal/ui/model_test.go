package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func TestSelectionStaysInBounds(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m.moveSelection(-10)
	if m.selected != 0 {
		t.Fatalf("selected = %d, want 0", m.selected)
	}
	m.moveSelection(100)
	if m.selected != len(m.issues)-1 {
		t.Fatalf("selected = %d, want %d", m.selected, len(m.issues)-1)
	}
}

func TestTeamFilterResetsSelection(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m.selected = 3
	m.cycleTeam(1)
	if m.selected != 0 {
		t.Fatalf("selected = %d, want 0", m.selected)
	}
	for _, issue := range m.issues {
		if issue.Team.ID != dashboard.Teams[0].ID {
			t.Fatalf("ticket %s belongs to wrong team", issue.Identifier)
		}
	}
}

func TestSnapshotContainsCoreInformation(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	view := Snapshot(dashboard, 120, 32)
	for _, expected := range []string{"TUINEAR", "PLAT-42", "Persist the issue cache", "Aisha", "All issues"} {
		if !strings.Contains(view, expected) {
			t.Errorf("snapshot missing %q", expected)
		}
	}
	if got := lipgloss.Width(view); got != 120 {
		t.Errorf("snapshot width = %d, want 120", got)
	}
	if got := lipgloss.Height(view); got != 32 {
		t.Errorf("snapshot height = %d, want 32", got)
	}
}

func TestWrapTextRespectsWidth(t *testing.T) {
	for _, line := range wrapText("one two three four five", 8) {
		if len([]rune(line)) > 8 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}
