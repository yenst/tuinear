package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/issuefilter"
	"github.com/jihmy/tuinear/internal/linear"
)

type filterLoaderStub struct {
	dashboard linear.Dashboard
	saved     map[string]issuefilter.State
	loadErr   error
	saveErr   error
	loads     []string
	saves     []string
	revisions []uint64
}

func (s *filterLoaderStub) FetchDashboard(context.Context) (linear.Dashboard, error) {
	return s.dashboard, nil
}

func (s *filterLoaderStub) LoadIssueFilters(_ context.Context, profileKey string) (issuefilter.State, error) {
	s.loads = append(s.loads, profileKey)
	return s.saved[profileKey].Clone(), s.loadErr
}

func (s *filterLoaderStub) SaveIssueFilters(_ context.Context, profileKey string, filters issuefilter.State, revision uint64) error {
	s.saves = append(s.saves, profileKey)
	s.revisions = append(s.revisions, revision)
	if s.saveErr == nil {
		s.saved[profileKey] = filters.Clone()
	}
	return s.saveErr
}

func loadDashboardWithSavedFilters(t *testing.T, loader *filterLoaderStub) Model {
	t.Helper()
	m := New(loader)
	updated, cmd := m.Update(m.Init()())
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("dashboard load did not request saved filters")
	}
	updated, _ = m.Update(cmd())
	return updated.(Model)
}

func TestSavedFiltersLoadForActiveProfile(t *testing.T) {
	dashboard := editorDashboard(t)
	want := IssueFilters{ExcludedStatuses: []string{"completed"}}
	loader := &filterLoaderStub{
		dashboard: dashboard,
		saved:     map[string]issuefilter.State{"profile:demo-work": want},
	}
	m := loadDashboardWithSavedFilters(t, loader)
	if !reflect.DeepEqual(m.filters, want) || len(m.issues) != len(dashboard.Issues)-1 {
		t.Fatalf("loaded filters = %#v with %d issues", m.filters, len(m.issues))
	}
	if len(loader.loads) != 1 || loader.loads[0] != "profile:demo-work" || !strings.Contains(m.View().Content, "status≠completed") {
		t.Fatalf("filter load = %#v, view missing active filter", loader.loads)
	}
}

func TestAssignedToMeFilterSavesForActiveProfile(t *testing.T) {
	dashboard := editorDashboard(t)
	viewer := linear.User{ID: dashboard.Viewer.ID, Name: dashboard.Viewer.Name, DisplayName: dashboard.Viewer.DisplayName}
	dashboard.Issues[0].Assignee = &viewer
	loader := &filterLoaderStub{dashboard: dashboard, saved: map[string]issuefilter.State{}}
	m := loadDashboardWithSavedFilters(t, loader)
	m = updateKey(m, textKey("f"))
	m.paletteIdx = filterOptionIndex(t, m, "Assignee: Me (Jamie)")
	updated, cmd := m.Update(specialKey(tea.KeyEnter))
	m = updated.(Model)
	if cmd == nil || m.filters.Assignee != dashboard.Viewer.ID || len(m.issues) != 1 {
		t.Fatalf("me filter = %#v, issues=%d, cmd=%v", m.filters, len(m.issues), cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	got := loader.saved["profile:demo-work"]
	if got.Assignee != dashboard.Viewer.ID || len(loader.saves) != 1 || m.filterErr != nil {
		t.Fatalf("saved me filter = %#v, saves=%#v, error=%v", got, loader.saves, m.filterErr)
	}
}

func TestSavedFiltersAreIsolatedWhenAccountChanges(t *testing.T) {
	work := editorDashboard(t)
	personal, err := (linear.DemoClient{}).SwitchAccount(t.Context(), "demo-personal")
	if err != nil {
		t.Fatal(err)
	}
	loader := &filterLoaderStub{
		dashboard: work,
		saved: map[string]issuefilter.State{
			"profile:demo-work":     {Priority: "2"},
			"profile:demo-personal": {ExcludedStatuses: []string{"completed"}},
		},
	}
	m := loadDashboardWithSavedFilters(t, loader)
	updated, cmd := m.Update(dashboardLoadedMsg{dashboard: personal})
	m = updated.(Model)
	if cmd == nil || !m.filters.Empty() {
		t.Fatalf("account switch did not clear before load: %#v, cmd=%v", m.filters, cmd != nil)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !reflect.DeepEqual(m.filters.ExcludedStatuses, []string{"completed"}) || m.filters.Priority != "" {
		t.Fatalf("personal filters = %#v", m.filters)
	}
}

func TestFilterPersistenceFailureIsVisibleAndDoesNotUndoFilter(t *testing.T) {
	dashboard := editorDashboard(t)
	loader := &filterLoaderStub{dashboard: dashboard, saved: map[string]issuefilter.State{}, saveErr: errors.New("disk full")}
	m := loadDashboardWithSavedFilters(t, loader)
	m.filters.Status = "Done"
	m.filterIssues()
	cmd := m.saveIssueFilters()
	updated, _ := m.Update(cmd())
	m = updated.(Model)
	if m.filters.Status != "Done" || m.filterErr == nil || !strings.Contains(m.View().Content, "disk full") {
		t.Fatalf("failed save = filters=%#v error=%v", m.filters, m.filterErr)
	}
}
