package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
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
