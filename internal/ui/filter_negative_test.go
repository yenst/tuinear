package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFilterIssuesSupportsMultipleNegativeStatuses(t *testing.T) {
	issues := demoIssues(t)
	issues[1].State.Name = "On hold"
	issues[1].State.Type = "started"
	got := FilterIssues(issues, IssueFilters{ExcludedStatuses: []string{"completed", "On hold"}}, "")
	if len(got) != 2 || got[0].Identifier != "PLAT-42" || got[1].Identifier != "PLAT-37" {
		t.Fatalf("negative status filter = %#v", got)
	}
}

func TestFilterPaletteBangTogglesNotWithoutClosing(t *testing.T) {
	m := NewWithDashboard(editorDashboard(t))
	m = updateKey(m, textKey("f"))
	doneIndex := filterOptionIndex(t, m, "Status: Done")
	m.paletteIdx = doneIndex
	m = updateKey(m, textKey("!"))
	if !m.palette || !matchesAny(m.filters.ExcludedStatuses, "Done") || len(m.issues) != 3 {
		t.Fatalf("NOT Done = palette=%v filters=%#v issues=%d", m.palette, m.filters, len(m.issues))
	}
	if !strings.Contains(m.View().Content, "Status: NOT Done") || !strings.Contains(m.View().Content, "status≠Done") {
		t.Fatal("negative filter is not visible")
	}
	doneIndex = filterOptionIndex(t, m, "Status: NOT Done")
	m.paletteIdx = doneIndex
	m = updateKey(m, textKey("!"))
	if len(m.filters.ExcludedStatuses) != 0 || len(m.issues) != 4 {
		t.Fatalf("toggled-off NOT Done = %#v, issues=%d", m.filters, len(m.issues))
	}
}

func TestActiveStatusPresetExcludesCompletedAndCanceled(t *testing.T) {
	m := NewWithDashboard(editorDashboard(t))
	m = updateKey(m, textKey("f"))
	m.paletteIdx = filterOptionIndex(t, m, "Status: Active (NOT completed/canceled)")
	m = updateKey(m, specialKey(tea.KeyEnter))
	if m.palette || !matchesAny(m.filters.ExcludedStatuses, "completed") || !matchesAny(m.filters.ExcludedStatuses, "canceled") || len(m.issues) != 3 {
		t.Fatalf("active preset = palette=%v filters=%#v issues=%d", m.palette, m.filters, len(m.issues))
	}
}

func filterOptionIndex(t *testing.T, m Model, label string) int {
	t.Helper()
	for index, option := range m.filterOptions() {
		if option.label == label {
			return index
		}
	}
	t.Fatalf("filter palette missing %q", label)
	return -1
}
