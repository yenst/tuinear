package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yenst/tuinear/internal/linear"
)

type cachedLoaderStub struct {
	cached        linear.Dashboard
	remote        linear.Dashboard
	cachedAt      time.Time
	cacheErr      error
	remoteErr     error
	remoteFetches int
}

type cachedAccountLoaderStub struct {
	work       linear.Dashboard
	personal   linear.Dashboard
	cachedAt   time.Time
	selectedID string
	fetches    int
}

func (s *cachedAccountLoaderStub) FetchDashboard(context.Context) (linear.Dashboard, error) {
	s.fetches++
	return s.personal, nil
}

func (s *cachedAccountLoaderStub) SwitchAccountCached(_ context.Context, accountID string) (linear.Dashboard, time.Time, error) {
	s.selectedID = accountID
	return s.personal, s.cachedAt, nil
}

func (s *cachedLoaderStub) LoadCachedDashboard(context.Context) (linear.Dashboard, time.Time, error) {
	return s.cached, s.cachedAt, s.cacheErr
}

func (s *cachedLoaderStub) FetchDashboard(context.Context) (linear.Dashboard, error) {
	s.remoteFetches++
	return s.remote, s.remoteErr
}

func textKey(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text})
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func updateKey(m Model, key tea.KeyPressMsg) Model {
	updated, _ := m.Update(key)
	return updated.(Model)
}

func demoIssues(t *testing.T) []linear.Issue {
	t.Helper()
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return dashboard.Issues
}

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
	for _, expected := range []string{"TUINEAR", "Acme / Jamie", "Accounts", "Personal / Jamie", "PLAT-42", "Persist the issue cache", "Aisha", "All issues"} {
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

func TestCycleAccountRefreshesDashboardInPlace(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := New(linear.DemoClient{})
	m.applyDashboard(dashboard)
	cmd := m.cycleAccount(1)
	if cmd == nil {
		t.Fatal("cycleAccount returned no command")
	}
	msg := cmd()
	updated, _ := m.Update(msg)
	got := updated.(Model)
	if got.dashboard.ActiveAccountID != "demo-personal" || got.dashboard.Organization.Name != "Personal" {
		t.Fatalf("active dashboard = %#v", got.dashboard)
	}
	if got.teamIndex != 0 || got.selected != 0 {
		t.Fatalf("selection was not reset: team=%d issue=%d", got.teamIndex, got.selected)
	}
}

func TestShiftAUsesCachedAccountThenRefreshes(t *testing.T) {
	work, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	personal, err := (linear.DemoClient{}).SwitchAccount(t.Context(), "demo-personal")
	if err != nil {
		t.Fatal(err)
	}
	loader := &cachedAccountLoaderStub{
		work: work, personal: personal, cachedAt: time.Now().Add(-time.Minute),
	}
	m := New(loader)
	m.applyDashboard(work)
	shiftA := tea.KeyPressMsg(tea.Key{Code: 'a', ShiftedCode: 'A', Mod: tea.ModShift})
	updated, switchCmd := m.Update(shiftA)
	m = updated.(Model)
	if switchCmd == nil || !m.loading {
		t.Fatal("shift+a did not start account switching")
	}
	updated, refreshCmd := m.Update(switchCmd())
	m = updated.(Model)
	if loader.selectedID != "demo-personal" || m.dashboard.ActiveAccountID != "demo-personal" {
		t.Fatalf("selected account = %q, dashboard = %q", loader.selectedID, m.dashboard.ActiveAccountID)
	}
	if !m.fromCache || !m.refreshing || refreshCmd == nil || loader.fetches != 0 {
		t.Fatalf("cached switch state = cached=%v refreshing=%v cmd=%v fetches=%d", m.fromCache, m.refreshing, refreshCmd != nil, loader.fetches)
	}
	updated, _ = m.Update(refreshCmd())
	m = updated.(Model)
	if m.fromCache || m.refreshing || loader.fetches != 1 {
		t.Fatalf("refreshed switch state = cached=%v refreshing=%v fetches=%d", m.fromCache, m.refreshing, loader.fetches)
	}
}

func TestHelpOverlayOpensWithEnhancedShiftKeyAndCloses(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m.width, m.height = 100, 28
	shiftH := tea.KeyPressMsg(tea.Key{Code: 'h', ShiftedCode: 'H', Mod: tea.ModShift})
	m = updateKey(m, shiftH)
	if !m.help {
		t.Fatal("shift+h did not open help")
	}
	view := m.View()
	for _, expected := range []string{"Tuinear keybindings", "Navigate", "Ticket actions", "saved filters", "close this help"} {
		if !strings.Contains(view.Content, expected) {
			t.Errorf("help overlay missing %q", expected)
		}
	}
	if lipgloss.Width(view.Content) != 100 || lipgloss.Height(view.Content) != 28 {
		t.Fatalf("help view dimensions = %dx%d", lipgloss.Width(view.Content), lipgloss.Height(view.Content))
	}
	if view.WindowTitle != "Tuinear" {
		t.Fatalf("window title = %q", view.WindowTitle)
	}
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.help {
		t.Fatal("escape did not close help")
	}
}

func TestWrapTextRespectsWidth(t *testing.T) {
	for _, line := range wrapText("one two three four five", 8) {
		if len([]rune(line)) > 8 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestLongAccountLabelDoesNotResizeScreen(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	dashboard.Organization.Name = strings.Repeat("Very Long Workspace ", 5)
	dashboard.Viewer.DisplayName = strings.Repeat("Long User ", 5)
	view := Snapshot(dashboard, 44, 16)
	if got := lipgloss.Width(view); got != 44 {
		t.Errorf("snapshot width = %d, want 44", got)
	}
	if got := lipgloss.Height(view); got != 16 {
		t.Errorf("snapshot height = %d, want 16", got)
	}
}

func TestAccountLayoutFitsCommonTerminalWidths(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{72, 80, 96} {
		view := Snapshot(dashboard, width, 24)
		if got := lipgloss.Width(view); got != width {
			t.Errorf("snapshot width = %d, want %d", got, width)
		}
		if got := lipgloss.Height(view); got != 24 {
			t.Errorf("snapshot height at width %d = %d, want 24", width, got)
		}
		if !strings.Contains(view, "Accounts") || !strings.Contains(view, "Personal") {
			t.Errorf("account list is not visible at width %d", width)
		}
	}
}

func TestSearchChromeDoesNotWrap(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	for _, width := range []int{44, 72, 80, 95, 120} {
		if got := lipgloss.Height(m.renderSearchBar(width)); got != 1 {
			t.Errorf("search bar height at width %d = %d, want 1", width, got)
		}
		if got := lipgloss.Height(m.renderFooter(width)); got != 1 {
			t.Errorf("footer height at width %d = %d, want 1", width, got)
		}
	}
}

func TestFilterIssuesTable(t *testing.T) {
	issues := demoIssues(t)
	tests := []struct {
		name    string
		query   string
		filters IssueFilters
		want    []string
	}{
		{name: "identifier search", query: "plat-37", want: []string{"PLAT-37"}},
		{name: "title search", query: "keyboard-first", want: []string{"PROD-18"}},
		{name: "assignee", filters: IssueFilters{Assignee: "Aisha"}, want: []string{"PLAT-42", "PROD-11"}},
		{name: "unassigned", filters: IssueFilters{Assignee: "Unassigned"}, want: []string{"PLAT-37"}},
		{name: "status", filters: IssueFilters{Status: "completed"}, want: []string{"PROD-11"}},
		{name: "priority", filters: IssueFilters{Priority: "Urgent"}, want: []string{"PROD-18"}},
		{name: "project", filters: IssueFilters{Project: "Public launch"}, want: []string{"PLAT-42", "PROD-18", "PROD-11"}},
		{name: "composed", query: "cache", filters: IssueFilters{Assignee: "Aisha", Status: "started", Priority: "2", Project: "project-launch"}, want: []string{"PLAT-42"}},
		{name: "empty retains no issue", query: "does-not-exist", filters: IssueFilters{Status: "started"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterIssues(issues, tt.filters, tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d issues, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i].Identifier != want {
					t.Errorf("issue %d = %s, want %s", i, got[i].Identifier, want)
				}
			}
		})
	}
}

func TestFilterIssuesHandlesLargeCachedSet(t *testing.T) {
	issues := demoIssues(t)
	large := make([]linear.Issue, 0, 10000)
	for i := 0; i < 2500; i++ {
		for _, issue := range issues {
			issue.ID = fmt.Sprintf("cached-%d-%s", i, issue.ID)
			large = append(large, issue)
		}
	}
	got := FilterIssues(large, IssueFilters{Status: "started"}, "cache")
	if len(got) != 2500 {
		t.Fatalf("large search returned %d issues, want 2500", len(got))
	}
}

func TestSearchInteractionIsIncrementalAndEscapable(t *testing.T) {
	m := NewWithDashboard((func() linear.Dashboard {
		dashboard, _ := (linear.DemoClient{}).FetchDashboard(t.Context())
		return dashboard
	})())
	m = updateKey(m, textKey("/"))
	if !m.searching {
		t.Fatal("slash did not open search")
	}
	m = updateKey(m, textKey("cache"))
	if m.query != "cache" || len(m.issues) != 1 || m.issues[0].Identifier != "PLAT-42" {
		t.Fatalf("search state = query %q issues %#v", m.query, m.issues)
	}
	view := m.View().Content
	if !strings.Contains(view, "Search: cache") || !strings.Contains(view, "PLAT-42") {
		t.Fatalf("search state is not visible in view: %s", view)
	}
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.query != "" || !m.searching {
		t.Fatalf("first escape should clear query and retain search mode: %#v", m)
	}
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.searching {
		t.Fatal("second escape should close search")
	}
	m = updateKey(m, textKey("/"))
	m = updateKey(m, textKey("plat"))
	m = updateKey(m, specialKey(tea.KeyEnter))
	m = updateKey(m, specialKey(tea.KeyEscape))
	if m.query != "" {
		t.Fatal("escape should clear a committed query")
	}
}

func TestSearchAcceptsShiftedCharacters(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m = updateKey(m, textKey("/"))
	m = updateKey(m, tea.KeyPressMsg(tea.Key{Code: 'P', Text: "P", Mod: tea.ModShift}))
	if m.query != "P" {
		t.Fatalf("shifted search input = %q, want P", m.query)
	}
}

func TestFilterPaletteIsDiscoverableAndAppliesSelection(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m = updateKey(m, textKey("f"))
	if !m.palette {
		t.Fatal("f did not open filter palette")
	}
	view := m.View().Content
	for _, expected := range []string{"Filter palette", "Assignee: Any", "Status: Any", "Priority: Any", "Project: Any"} {
		if !strings.Contains(view, expected) {
			t.Errorf("palette missing %q", expected)
		}
	}
	want := "Status: In Progress"
	options := m.filterOptions()
	index := -1
	for i, option := range options {
		if option.label == want {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("palette missing %q", want)
	}
	for i := 0; i < index; i++ {
		m = updateKey(m, textKey("j"))
	}
	m = updateKey(m, specialKey(tea.KeyEnter))
	if m.palette || m.filters.Status != "In Progress" || len(m.issues) != 1 {
		t.Fatalf("palette selection = palette=%v filters=%#v issues=%d", m.palette, m.filters, len(m.issues))
	}
	if !strings.Contains(m.View().Content, "status=In Progress") {
		t.Fatal("active filter is not visible")
	}
}

func TestFilterPaletteKeepsSelectionVisibleWhenItScrolls(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for index := range 30 {
		issue := dashboard.Issues[0]
		issue.ID = fmt.Sprintf("project-issue-%d", index)
		issue.Project = &linear.Project{ID: fmt.Sprintf("project-%d", index), Name: fmt.Sprintf("Project %02d", index)}
		dashboard.Issues = append(dashboard.Issues, issue)
	}
	m := NewWithDashboard(dashboard)
	m.width, m.height, m.palette = 80, 16, true
	m.paletteIdx = len(m.filterOptions()) - 1
	selected := m.filterOptions()[m.paletteIdx].label
	if view := m.View().Content; !strings.Contains(view, selected) {
		t.Fatalf("selected scrolling option %q is not visible", selected)
	}
}

func TestControlCQuitsFromModalModes(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboard(dashboard)
		configure(&m)
		_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
		if cmd == nil {
			t.Fatal("ctrl+c did not return a quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("ctrl+c command returned %T, want tea.QuitMsg", msg)
		}
	}
}

func TestWarmStartupRendersCacheBeforeRefreshing(t *testing.T) {
	cached, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	remote := cached
	cached.Organization.Name = "Cached workspace"
	remote.Organization.Name = "Fresh workspace"
	loader := &cachedLoaderStub{cached: cached, remote: remote, cachedAt: time.Now().Add(-5 * time.Minute)}
	m := New(loader)

	cacheMsg := m.Init()()
	if loader.remoteFetches != 0 {
		t.Fatal("warm startup contacted the network before rendering cache")
	}
	updated, refresh := m.Update(cacheMsg)
	m = updated.(Model)
	if m.loading || !m.fromCache || !m.refreshing || m.dashboard.Organization.Name != "Cached workspace" {
		t.Fatalf("cached startup state = %#v", m)
	}
	if refresh == nil || !strings.Contains(m.View().Content, "cached 5m") {
		t.Fatal("cached view did not start a visible background refresh")
	}

	updated, _ = m.Update(refresh())
	m = updated.(Model)
	if m.fromCache || m.refreshing || m.dashboard.Organization.Name != "Fresh workspace" || loader.remoteFetches != 1 {
		t.Fatalf("fresh synchronization state = %#v, fetches=%d", m, loader.remoteFetches)
	}
}

func TestOfflineRefreshKeepsLastKnownGoodDashboard(t *testing.T) {
	cached, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	loader := &cachedLoaderStub{
		cached: cached, cachedAt: time.Now().Add(-2 * time.Hour), remoteErr: errors.New("network unavailable"),
	}
	m := New(loader)
	updated, refresh := m.Update(m.Init()())
	m = updated.(Model)
	updated, _ = m.Update(refresh())
	m = updated.(Model)
	if m.err != nil || m.refreshErr == nil || !m.fromCache || len(m.issues) == 0 {
		t.Fatalf("offline cached state = %#v", m)
	}
	view := m.View().Content
	if !strings.Contains(view, "offline · cached 2h") || !strings.Contains(view, "PLAT-42") ||
		!strings.Contains(view, "Refresh: network unavailable") {
		t.Fatal("offline cache is not visibly retained")
	}
}

func TestManualRefreshShowsProgressAndAppliesFreshDashboard(t *testing.T) {
	current, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fresh := current
	fresh.Organization.Name = "Fresh workspace"
	loader := &cachedLoaderStub{remote: fresh}
	m := New(loader)
	m.applyDashboard(current)

	updated, refreshCmd := m.Update(textKey("r"))
	m = updated.(Model)
	if refreshCmd == nil || !m.refreshing || !strings.Contains(m.View().Content, "refreshing") {
		t.Fatalf("manual refresh did not visibly start: refreshing=%v cmd=%v", m.refreshing, refreshCmd != nil)
	}
	updated, _ = m.Update(refreshCmd())
	m = updated.(Model)
	if m.refreshing || m.refreshErr != nil || m.dashboard.Organization.Name != "Fresh workspace" || loader.remoteFetches != 1 {
		t.Fatalf("manual refresh result = %#v, fetches=%d", m, loader.remoteFetches)
	}
}

func TestRefreshPreservesSelectedIssueAndTeamByID(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m.cycleTeam(1)
	m.selected = 1
	wantIssueID := m.selectedIssue().ID
	wantTeamID := m.dashboard.Teams[m.teamIndex-1].ID

	refreshed := dashboard
	refreshed.Teams = append(refreshed.Teams, linear.Team{ID: "new-team", Key: "NEW", Name: "AAA"})
	for left, right := 0, len(refreshed.Issues)-1; left < right; left, right = left+1, right-1 {
		refreshed.Issues[left], refreshed.Issues[right] = refreshed.Issues[right], refreshed.Issues[left]
	}
	m.applyDashboard(refreshed)
	if got := m.dashboard.Teams[m.teamIndex-1].ID; got != wantTeamID {
		t.Fatalf("selected team = %s, want %s", got, wantTeamID)
	}
	if issue := m.selectedIssue(); issue == nil || issue.ID != wantIssueID {
		t.Fatalf("selected issue = %#v, want ID %s", issue, wantIssueID)
	}
}

func TestFilteredLayoutRemainsResponsive(t *testing.T) {
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m := NewWithDashboard(dashboard)
	m.query = "no-match"
	m.filters = IssueFilters{Status: "started", Project: "Public launch"}
	m.filterIssues()
	for _, size := range [][2]int{{44, 16}, {72, 24}, {120, 32}} {
		view := Snapshot(dashboard, size[0], size[1])
		if got := lipgloss.Width(view); got != size[0] {
			t.Errorf("width=%d, want %d", got, size[0])
		}
		if got := lipgloss.Height(view); got != size[1] {
			t.Errorf("height=%d, want %d", got, size[1])
		}
	}
	view := m.View().Content
	if !strings.Contains(view, "Search: no-match") || !strings.Contains(view, "No tickets") {
		t.Fatal("empty filtered state should preserve query and explain empty result")
	}
}
