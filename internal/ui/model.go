package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

type DashboardLoader interface {
	FetchDashboard(context.Context) (linear.Dashboard, error)
}

type dashboardLoadedMsg struct{ dashboard linear.Dashboard }
type dashboardFailedMsg struct{ err error }

type Model struct {
	loader     DashboardLoader
	dashboard  linear.Dashboard
	issues     []linear.Issue
	teamIndex  int
	selected   int
	width      int
	height     int
	loading    bool
	err        error
	lastLoaded time.Time
}

func New(loader DashboardLoader) Model {
	return Model{loader: loader, width: 120, height: 34, loading: true}
}

func NewWithDashboard(dashboard linear.Dashboard) Model {
	m := Model{width: 120, height: 34}
	m.applyDashboard(dashboard)
	return m
}

func (m Model) Init() tea.Cmd {
	return loadDashboard(m.loader)
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case dashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		return m, nil
	case dashboardFailedMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, loadDashboard(m.loader)
		case "j", "down":
			m.moveSelection(1)
		case "k", "up":
			m.moveSelection(-1)
		case "g", "home":
			m.selected = 0
		case "G", "end":
			if len(m.issues) > 0 {
				m.selected = len(m.issues) - 1
			}
		case "tab", "]":
			m.cycleTeam(1)
		case "shift+tab", "[":
			m.cycleTeam(-1)
		}
	}
	return m, nil
}

func (m *Model) applyDashboard(dashboard linear.Dashboard) {
	sort.SliceStable(dashboard.Teams, func(i, j int) bool {
		return dashboard.Teams[i].Name < dashboard.Teams[j].Name
	})
	m.dashboard = dashboard
	m.loading = false
	m.err = nil
	m.lastLoaded = time.Now()
	if m.teamIndex > len(dashboard.Teams) {
		m.teamIndex = 0
	}
	m.filterIssues()
}

func (m *Model) moveSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.issues) {
		m.selected = len(m.issues) - 1
	}
}

func (m *Model) cycleTeam(delta int) {
	count := len(m.dashboard.Teams) + 1
	if count <= 1 {
		return
	}
	m.teamIndex = (m.teamIndex + delta + count) % count
	m.selected = 0
	m.filterIssues()
}

func (m *Model) filterIssues() {
	m.issues = m.issues[:0]
	if m.teamIndex == 0 {
		m.issues = append(m.issues, m.dashboard.Issues...)
	} else {
		teamID := m.dashboard.Teams[m.teamIndex-1].ID
		for _, issue := range m.dashboard.Issues {
			if issue.Team.ID == teamID {
				m.issues = append(m.issues, issue)
			}
		}
	}
	if len(m.issues) == 0 {
		m.selected = 0
	} else if m.selected >= len(m.issues) {
		m.selected = len(m.issues) - 1
	}
}

func (m Model) activeTeamName() string {
	if m.teamIndex == 0 || m.teamIndex > len(m.dashboard.Teams) {
		return "All issues"
	}
	return m.dashboard.Teams[m.teamIndex-1].Name
}

func (m Model) selectedIssue() *linear.Issue {
	if m.selected < 0 || m.selected >= len(m.issues) {
		return nil
	}
	issue := m.issues[m.selected]
	return &issue
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func Snapshot(dashboard linear.Dashboard, width, height int) string {
	m := NewWithDashboard(dashboard)
	m.width = width
	m.height = height
	return m.render()
}
