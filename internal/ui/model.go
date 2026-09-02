package ui

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/browser"
	"github.com/jihmy/tuinear/internal/linear"
)

type DashboardLoader interface {
	FetchDashboard(context.Context) (linear.Dashboard, error)
}

type CachedDashboardLoader interface {
	LoadCachedDashboard(context.Context) (linear.Dashboard, time.Time, error)
}

type AccountSwitcher interface {
	SwitchAccount(context.Context, string) (linear.Dashboard, error)
}

type IssueUpdater interface {
	UpdateIssue(context.Context, string, linear.IssueUpdate) (linear.Issue, error)
}

type dashboardLoadedMsg struct{ dashboard linear.Dashboard }
type dashboardFailedMsg struct{ err error }
type cachedDashboardLoadedMsg struct {
	dashboard linear.Dashboard
	cachedAt  time.Time
}
type cachedDashboardUnavailableMsg struct{ err error }
type issueBrowserOpenedMsg struct{}
type issueBrowserFailedMsg struct{ err error }

type BrowserOpener func(string) error

type Model struct {
	loader            DashboardLoader
	dashboard         linear.Dashboard
	issues            []linear.Issue
	teamIndex         int
	selected          int
	query             string
	filters           IssueFilters
	searching         bool
	palette           bool
	paletteIdx        int
	width             int
	height            int
	loading           bool
	err               error
	lastLoaded        time.Time
	fromCache         bool
	cachedAt          time.Time
	refreshing        bool
	refreshErr        error
	browserOpen       BrowserOpener
	browserErr        error
	issueUpdater      IssueUpdater
	issueArchiver     IssueArchiver
	editor            *titleEditor
	choiceEditor      *choiceEditor
	labelEditor       *labelEditor
	descriptionEditor *descriptionEditor
	archiveConfirm    *archiveConfirmation
	actionMenu        *actionMenu
	pendingEdit       *pendingIssueEdit
	pendingArchive    *pendingIssueArchive
	editErr           error
}

func New(loader DashboardLoader) Model {
	m := Model{loader: loader, width: 120, height: 34, loading: true, browserOpen: browser.Open}
	if updater, ok := loader.(IssueUpdater); ok {
		m.issueUpdater = updater
	}
	if archiver, ok := loader.(IssueArchiver); ok {
		m.issueArchiver = archiver
	}
	return m
}

func NewWithDashboard(dashboard linear.Dashboard) Model {
	m := Model{width: 120, height: 34, browserOpen: browser.Open}
	m.applyDashboard(dashboard)
	return m
}

func NewWithDashboardAndUpdater(dashboard linear.Dashboard, updater IssueUpdater) Model {
	m := NewWithDashboard(dashboard)
	m.issueUpdater = updater
	if archiver, ok := updater.(IssueArchiver); ok {
		m.issueArchiver = archiver
	}
	return m
}

// NewWithBrowser is intended for callers that need to provide an alternate
// browser launcher, such as tests or an embedding application.
func NewWithBrowser(loader DashboardLoader, opener BrowserOpener) Model {
	m := New(loader)
	m.browserOpen = opener
	return m
}

// NewWithDashboardAndBrowser is the dashboard equivalent of NewWithBrowser.
func NewWithDashboardAndBrowser(dashboard linear.Dashboard, opener BrowserOpener) Model {
	m := NewWithDashboard(dashboard)
	m.browserOpen = opener
	return m
}

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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case dashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		m.rebaseOpenEditors()
		m.rebasePendingIssueEdit()
		m.fromCache = false
		m.cachedAt = time.Time{}
		m.refreshing = false
		m.refreshErr = nil
		return m, nil
	case cachedDashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		m.fromCache = true
		m.cachedAt = msg.cachedAt
		m.refreshing = true
		m.refreshErr = nil
		return m, loadDashboard(m.loader)
	case cachedDashboardUnavailableMsg:
		return m, loadDashboard(m.loader)
	case dashboardFailedMsg:
		preserveDashboard := m.fromCache || m.refreshing
		m.loading = false
		m.refreshing = false
		if preserveDashboard {
			m.err = nil
			m.refreshErr = msg.err
		} else {
			m.err = msg.err
		}
		return m, nil
	case issueBrowserOpenedMsg:
		return m, nil
	case issueBrowserFailedMsg:
		m.browserErr = msg.err
		return m, nil
	case issueUpdatedMsg:
		m.finishIssueEdit(msg.issue)
		return m, nil
	case issueUpdateFailedMsg:
		m.rollbackIssueEdit(msg.issueID, msg.err)
		return m, nil
	case issueArchivedMsg:
		m.finishIssueArchive(msg.issueID)
		return m, nil
	case issueArchiveFailedMsg:
		m.failIssueArchive(msg.issueID, msg.err)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.editor != nil {
			return m, m.updateTitleEditor(msg)
		}
		if m.choiceEditor != nil {
			return m, m.updateChoiceEditor(msg)
		}
		if m.labelEditor != nil {
			return m, m.updateLabelEditor(msg)
		}
		if m.descriptionEditor != nil {
			return m, m.updateDescriptionEditor(msg)
		}
		if m.archiveConfirm != nil {
			return m, m.updateArchiveConfirmation(msg)
		}
		if m.actionMenu != nil {
			return m, m.updateActionMenu(msg)
		}
		if m.pendingArchive != nil {
			return m, nil
		}
		if m.palette {
			m.updatePalette(msg)
			return m, nil
		}
		if m.searching {
			m.updateSearch(msg)
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "r":
			if m.pendingEdit != nil {
				return m, nil
			}
			m.err = nil
			m.refreshErr = nil
			if m.hasDashboard() {
				m.refreshing = true
			} else {
				m.loading = true
			}
			return m, loadDashboard(m.loader)
		case "a":
			if m.pendingEdit != nil {
				return m, nil
			}
			return m, m.cycleAccount(1)
		case "A":
			if m.pendingEdit != nil {
				return m, nil
			}
			return m, m.cycleAccount(-1)
		case "/":
			m.searching = true
		case "f", "ctrl+f":
			m.palette = true
			m.paletteIdx = 0
		case "space", " ":
			return m, m.openSelectedIssue()
		case "enter", "return":
			m.beginIssueActions()
		case "e":
			m.beginTitleEdit()
		case "s":
			m.beginStatusEdit()
		case "p":
			m.beginPriorityEdit()
		case "u":
			m.beginAssigneeEdit()
		case "P":
			m.beginProjectEdit()
		case "l":
			m.beginLabelEdit()
		case "d":
			m.beginDescriptionEdit()
		case "x":
			m.beginArchiveConfirmation()
		case "esc":
			if m.query != "" {
				m.query = ""
				m.filterIssues()
			} else if m.hasFilters() {
				m.filters = IssueFilters{}
				m.filterIssues()
			}
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

func (m *Model) openSelectedIssue() tea.Cmd {
	issue := m.selectedIssue()
	if issue == nil || strings.TrimSpace(issue.URL) == "" {
		return nil
	}
	issueURL, err := validateIssueURL(issue.URL)
	if err != nil {
		m.browserErr = err
		return nil
	}
	if m.browserOpen == nil {
		m.browserErr = fmt.Errorf("open issue in browser: browser launcher is not configured")
		return nil
	}
	m.browserErr = nil
	return openIssueURL(m.browserOpen, issueURL)
}

func openIssueURL(opener BrowserOpener, issueURL string) tea.Cmd {
	return func() tea.Msg {
		if err := opener(issueURL); err != nil {
			return issueBrowserFailedMsg{err: fmt.Errorf("open issue in browser: %w", err)}
		}
		return issueBrowserOpenedMsg{}
	}
}

func validateIssueURL(raw string) (string, error) {
	issueURL := strings.TrimSpace(raw)
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("cannot open issue URL: URL must use http or https")
	}
	return issueURL, nil
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

func (m Model) hasDashboard() bool {
	return m.dashboard.Organization.ID != "" || m.dashboard.Viewer.ID != "" || len(m.dashboard.Teams) > 0 || len(m.dashboard.Issues) > 0
}

func (m *Model) cycleAccount(delta int) tea.Cmd {
	accounts := m.dashboard.Accounts
	if m.loading || len(accounts) < 2 {
		return nil
	}
	switcher, ok := m.loader.(AccountSwitcher)
	if !ok {
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
	return switchAccount(switcher, target)
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
