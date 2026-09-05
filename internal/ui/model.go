package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yenst/tuinear/internal/browser"
	"github.com/yenst/tuinear/internal/linear"
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

type CachedAccountSwitcher interface {
	SwitchAccountCached(context.Context, string) (linear.Dashboard, time.Time, error)
}

type IssueUpdater interface {
	UpdateIssue(context.Context, string, linear.IssueUpdate) (linear.Issue, error)
}

type IssueCreator interface {
	CreateIssue(context.Context, linear.IssueCreate) (linear.Issue, error)
}

type dashboardLoadedMsg struct{ dashboard linear.Dashboard }
type dashboardFailedMsg struct{ err error }
type cachedDashboardLoadedMsg struct {
	dashboard linear.Dashboard
	cachedAt  time.Time
}
type cachedDashboardUnavailableMsg struct{ err error }
type accountDashboardLoadedMsg struct {
	dashboard linear.Dashboard
	cachedAt  time.Time
}
type issueBrowserOpenedMsg struct{}
type issueBrowserFailedMsg struct{ err error }

type issueCopyKind string

const (
	issueCopyBranch issueCopyKind = "git branch"
	issueCopyURL    issueCopyKind = "issue URL"
)

type issueCopiedMsg struct {
	kind       issueCopyKind
	identifier string
	value      string
}
type issueCopyFailedMsg struct{ err error }

type BrowserOpener func(string) error
type ClipboardSetter func(string) tea.Cmd

type Model struct {
	loader                   DashboardLoader
	dashboard                linear.Dashboard
	issues                   []linear.Issue
	teamIndex                int
	selected                 int
	query                    string
	filters                  IssueFilters
	searching                bool
	palette                  bool
	paletteIdx               int
	help                     bool
	width                    int
	height                   int
	loading                  bool
	err                      error
	lastLoaded               time.Time
	fromCache                bool
	cachedAt                 time.Time
	refreshing               bool
	refreshErr               error
	browserOpen              BrowserOpener
	browserErr               error
	clipboardSet             ClipboardSetter
	clipboardErr             error
	clipboardNotice          string
	issueUpdater             IssueUpdater
	issueCreator             IssueCreator
	issueArchiver            IssueArchiver
	editor                   *titleEditor
	createEditor             *createIssueEditor
	choiceEditor             *choiceEditor
	labelEditor              *labelEditor
	descriptionEditor        *descriptionEditor
	archiveConfirm           *archiveConfirmation
	actionMenu               *actionMenu
	pendingEdit              *pendingIssueEdit
	pendingCreate            *pendingIssueCreate
	pendingArchive           *pendingIssueArchive
	editErr                  error
	filterStore              IssueFilterStore
	filterProfileKey         string
	filterPreferencesLoading bool
	filterPreferencesLoaded  bool
	filterRevision           uint64
	filterErr                error
}

func New(loader DashboardLoader) Model {
	m := Model{loader: loader, width: 120, height: 34, loading: true, browserOpen: browser.Open, clipboardSet: tea.SetClipboard}
	if updater, ok := loader.(IssueUpdater); ok {
		m.issueUpdater = updater
	}
	if creator, ok := loader.(IssueCreator); ok {
		m.issueCreator = creator
	}
	if archiver, ok := loader.(IssueArchiver); ok {
		m.issueArchiver = archiver
	}
	if store, ok := loader.(IssueFilterStore); ok {
		m.filterStore = store
	}
	return m
}

func NewWithDashboard(dashboard linear.Dashboard) Model {
	m := Model{width: 120, height: 34, browserOpen: browser.Open, clipboardSet: tea.SetClipboard}
	m.applyDashboard(dashboard)
	return m
}

func NewWithDashboardAndUpdater(dashboard linear.Dashboard, updater IssueUpdater) Model {
	m := NewWithDashboard(dashboard)
	m.issueUpdater = updater
	if archiver, ok := updater.(IssueArchiver); ok {
		m.issueArchiver = archiver
	}
	if creator, ok := updater.(IssueCreator); ok {
		m.issueCreator = creator
	}
	return m
}

func NewWithDashboardAndCreator(dashboard linear.Dashboard, creator IssueCreator) Model {
	m := NewWithDashboard(dashboard)
	m.issueCreator = creator
	return m
}

// NewWithBrowser is intended for callers that need to provide an alternate
// browser launcher, such as tests or an embedding application.
func NewWithBrowser(loader DashboardLoader, opener BrowserOpener) Model {
	m := New(loader)
	m.browserOpen = opener
	return m
}

// NewWithClipboard is intended for callers that need to provide an alternate
// clipboard implementation, such as tests or an embedding application.
func NewWithClipboard(loader DashboardLoader, setter ClipboardSetter) Model {
	m := New(loader)
	m.clipboardSet = setter
	return m
}

// NewWithDashboardAndClipboard is the dashboard equivalent of NewWithClipboard.
func NewWithDashboardAndClipboard(dashboard linear.Dashboard, setter ClipboardSetter) Model {
	m := NewWithDashboard(dashboard)
	m.clipboardSet = setter
	return m
}

// NewWithDashboardAndBrowser is the dashboard equivalent of NewWithBrowser.
func NewWithDashboardAndBrowser(dashboard linear.Dashboard, opener BrowserOpener) Model {
	m := NewWithDashboard(dashboard)
	m.browserOpen = opener
	return m
}

func (m Model) hasDashboard() bool {
	return m.dashboard.Organization.ID != "" || m.dashboard.Viewer.ID != "" || len(m.dashboard.Teams) > 0 || len(m.dashboard.Issues) > 0
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
	view.WindowTitle = "Tuinear"
	return view
}

func Snapshot(dashboard linear.Dashboard, width, height int) string {
	m := NewWithDashboard(dashboard)
	m.width = width
	m.height = height
	return m.render()
}
