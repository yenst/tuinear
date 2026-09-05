package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/linear"
)

func (m Model) Init() tea.Cmd {
	if loader, ok := m.loader.(CachedDashboardLoader); ok {
		return loadCachedDashboard(loader)
	}
	return loadDashboard(m.loader)
}

func loadCachedDashboard(loader CachedDashboardLoader) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		dashboard, cachedAt, err := loader.LoadCachedDashboard(ctx)
		if err != nil {
			return cachedDashboardUnavailableMsg{err: err}
		}
		return cachedDashboardLoadedMsg{dashboard: dashboard, cachedAt: cachedAt}
	}
}

func loadDashboard(loader DashboardLoader) tea.Cmd {
	return func() tea.Msg {
		if loader == nil {
			return dashboardFailedMsg{err: fmt.Errorf("dashboard loader is not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		dashboard, err := loader.FetchDashboard(ctx)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		return dashboardLoadedMsg{dashboard: dashboard}
	}
}

func (m *Model) cycleAccount(delta int) tea.Cmd {
	accounts := m.dashboard.Accounts
	if m.loading || m.refreshing || len(accounts) < 2 {
		return nil
	}
	switcher, canSwitch := m.loader.(AccountSwitcher)
	cachedSwitcher, canSwitchCached := m.loader.(CachedAccountSwitcher)
	if !canSwitch && !canSwitchCached {
		return nil
	}
	current := -1
	for index, account := range accounts {
		if account.ID == m.dashboard.ActiveAccountID {
			current = index
			break
		}
	}
	next := 0
	if current >= 0 {
		next = (current + delta + len(accounts)) % len(accounts)
	}
	target := accounts[next].ID
	if target == "" || target == m.dashboard.ActiveAccountID {
		return nil
	}
	m.loading = true
	m.err = nil
	m.refreshErr = nil
	if canSwitchCached {
		return switchAccountCached(cachedSwitcher, target)
	}
	return switchAccount(switcher, target)
}

func switchAccountCached(switcher CachedAccountSwitcher, accountID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		dashboard, cachedAt, err := switcher.SwitchAccountCached(ctx, accountID)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		return accountDashboardLoadedMsg{dashboard: dashboard, cachedAt: cachedAt}
	}
}

func switchAccount(switcher AccountSwitcher, accountID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		dashboard, err := switcher.SwitchAccount(ctx, accountID)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		return dashboardLoadedMsg{dashboard: dashboard}
	}
}

func (m *Model) applyDashboard(dashboard linear.Dashboard) {
	selectedID := ""
	if issue := m.selectedIssue(); issue != nil {
		selectedID = issue.ID
	}
	teamID := ""
	if m.teamIndex > 0 && m.teamIndex <= len(m.dashboard.Teams) {
		teamID = m.dashboard.Teams[m.teamIndex-1].ID
	}
	accountChanged := m.dashboard.ActiveAccountID != "" &&
		dashboard.ActiveAccountID != "" &&
		m.dashboard.ActiveAccountID != dashboard.ActiveAccountID
	sort.SliceStable(dashboard.Teams, func(i, j int) bool {
		return dashboard.Teams[i].Name < dashboard.Teams[j].Name
	})
	m.dashboard = dashboard
	m.loading = false
	m.err = nil
	m.lastLoaded = time.Now()
	if accountChanged {
		m.teamIndex = 0
		m.selected = 0
	} else if teamID != "" {
		m.teamIndex = 0
		for index, team := range dashboard.Teams {
			if team.ID == teamID {
				m.teamIndex = index + 1
				break
			}
		}
	}
	if m.teamIndex > len(dashboard.Teams) {
		m.teamIndex = 0
	}
	m.filterIssues()
	if !accountChanged && selectedID != "" {
		for index, issue := range m.issues {
			if issue.ID == selectedID {
				m.selected = index
				break
			}
		}
	}
}
