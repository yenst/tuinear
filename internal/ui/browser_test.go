package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

func browserDashboard(t *testing.T) linear.Dashboard {
	t.Helper()
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return dashboard
}

func spaceKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Text: " "})
}

func TestSpaceOpensSelectedIssueURL(t *testing.T) {
	dashboard := browserDashboard(t)
	var gotURL string
	m := NewWithDashboardAndBrowser(dashboard, func(issueURL string) error {
		gotURL = issueURL
		return nil
	})

	updated, cmd := m.Update(spaceKey())
	if cmd == nil {
		t.Fatal("space returned no browser command")
	}
	updated, followUp := updated.(Model).Update(cmd())
	if followUp != nil {
		t.Fatal("successful browser open returned an unexpected follow-up command")
	}
	if gotURL != dashboard.Issues[0].URL {
		t.Fatalf("opened URL = %q, want %q", gotURL, dashboard.Issues[0].URL)
	}
	if model := updated.(Model); model.browserErr != nil {
		t.Fatalf("successful browser open left an error: %v", model.browserErr)
	}
}

func TestSpaceWithEmptySelectionOrURLIsNoOp(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*Model)
		clearURL bool
	}{
		{name: "no selection", prepare: func(m *Model) { m.issues = nil }},
		{name: "blank URL", clearURL: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboard := browserDashboard(t)
			if tt.clearURL {
				dashboard.Issues[0].URL = "  "
			}
			called := false
			m := NewWithDashboardAndBrowser(dashboard, func(string) error {
				called = true
				return nil
			})
			if tt.prepare != nil {
				tt.prepare(&m)
			}
			updated, cmd := m.Update(spaceKey())
			if cmd != nil || called {
				t.Fatalf("space should be a no-op: cmd=%v called=%v", cmd != nil, called)
			}
			if updated.(Model).browserErr != nil {
				t.Fatal("no-op should not show a browser error")
			}
		})
	}
}

func TestSpaceRejectsInvalidIssueURLWithoutLaunching(t *testing.T) {
	dashboard := browserDashboard(t)
	dashboard.Issues[0].URL = "javascript:alert(1)"
	called := false
	m := NewWithDashboardAndBrowser(dashboard, func(string) error {
		called = true
		return nil
	})
	updated, cmd := m.Update(spaceKey())
	if cmd != nil || called {
		t.Fatalf("invalid URL should not launch: cmd=%v called=%v", cmd != nil, called)
	}
	model := updated.(Model)
	if model.browserErr == nil || !strings.Contains(model.browserErr.Error(), "http or https") {
		t.Fatalf("invalid URL error = %v", model.browserErr)
	}
	if !strings.Contains(model.View().Content, "http or https") {
		t.Fatal("invalid URL error is not visible in the dashboard")
	}
}

func TestSpaceShowsBrowserLaunchFailureAndKeepsDashboard(t *testing.T) {
	dashboard := browserDashboard(t)
	wantErr := errors.New("browser unavailable")
	m := NewWithDashboardAndBrowser(dashboard, func(string) error { return wantErr })
	updated, cmd := m.Update(spaceKey())
	if cmd == nil {
		t.Fatal("space returned no browser command")
	}
	updated, _ = updated.(Model).Update(cmd())
	model := updated.(Model)
	if model.browserErr == nil || !strings.Contains(model.browserErr.Error(), wantErr.Error()) {
		t.Fatalf("browser error = %v", model.browserErr)
	}
	if len(model.issues) != len(dashboard.Issues) || model.selectedIssue() == nil {
		t.Fatal("browser failure replaced the current dashboard")
	}
	if !strings.Contains(model.View().Content, wantErr.Error()) {
		t.Fatal("browser failure is not visible in the dashboard")
	}
}

func TestBrowserFailureDoesNotHideActiveSearch(t *testing.T) {
	dashboard := browserDashboard(t)
	m := NewWithDashboardAndBrowser(dashboard, func(string) error {
		return errors.New("browser unavailable")
	})
	m.width = 44
	m.height = 20
	m.query = "login"

	updated, cmd := m.Update(spaceKey())
	if cmd == nil {
		t.Fatal("space returned no browser command")
	}
	updated, _ = updated.(Model).Update(cmd())
	view := updated.(Model).View().Content
	if !strings.Contains(view, "Search: login") {
		t.Fatal("browser failure hid the active search")
	}
	if !strings.Contains(view, "Browser:") {
		t.Fatal("browser failure is not visible at narrow width")
	}
}

func TestSpaceIsIsolatedFromSearchAndFilterPalette(t *testing.T) {
	dashboard := browserDashboard(t)
	called := 0
	launcher := func(string) error {
		called++
		return nil
	}
	for _, configure := range []func(*Model){
		func(m *Model) { m.searching = true },
		func(m *Model) { m.palette = true },
	} {
		m := NewWithDashboardAndBrowser(dashboard, launcher)
		configure(&m)
		updated, cmd := m.Update(spaceKey())
		if cmd != nil || called != 0 {
			t.Fatalf("space launched inside modal mode: cmd=%v calls=%d", cmd != nil, called)
		}
		if updated.(Model).browserErr != nil {
			t.Fatal("modal space unexpectedly changed browser state")
		}
	}
}
