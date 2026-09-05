package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case dashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		m.syncIssueFilterProfile()
		m.rebaseOpenEditors()
		m.rebasePendingIssueEdit()
		m.rebasePendingIssueCreate()
		m.fromCache = false
		m.cachedAt = time.Time{}
		m.refreshing = false
		m.refreshErr = nil
		return m, m.loadIssueFilters()
	case cachedDashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		m.syncIssueFilterProfile()
		m.rebasePendingIssueCreate()
		m.fromCache = true
		m.cachedAt = msg.cachedAt
		m.refreshing = true
		m.refreshErr = nil
		filterCmd := m.loadIssueFilters()
		if filterCmd != nil {
			return m, tea.Batch(loadDashboard(m.loader), filterCmd)
		}
		return m, loadDashboard(m.loader)
	case cachedDashboardUnavailableMsg:
		return m, loadDashboard(m.loader)
	case accountDashboardLoadedMsg:
		m.applyDashboard(msg.dashboard)
		m.syncIssueFilterProfile()
		m.rebaseOpenEditors()
		m.rebasePendingIssueEdit()
		m.rebasePendingIssueCreate()
		m.cachedAt = msg.cachedAt
		m.fromCache = !msg.cachedAt.IsZero()
		m.refreshing = m.fromCache
		m.refreshErr = nil
		filterCmd := m.loadIssueFilters()
		if !m.fromCache {
			return m, filterCmd
		}
		if filterCmd != nil {
			return m, tea.Batch(loadDashboard(m.loader), filterCmd)
		}
		return m, loadDashboard(m.loader)
	case dashboardFailedMsg:
		preserveDashboard := m.hasDashboard()
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
	case issueCopiedMsg:
		m.clipboardErr = nil
		m.clipboardNotice = fmt.Sprintf("Copied %s for %s", msg.kind, msg.identifier)
		if m.clipboardSet == nil {
			return m, nil
		}
		return m, m.clipboardSet(msg.value)
	case issueCopyFailedMsg:
		m.clipboardNotice = ""
		m.clipboardErr = msg.err
		return m, nil
	case issueUpdatedMsg:
		m.finishIssueEdit(msg.issue)
		return m, nil
	case issueUpdateFailedMsg:
		m.rollbackIssueEdit(msg.issueID, msg.err)
		return m, nil
	case issueCreatedMsg:
		m.finishIssueCreate(msg.issue)
		return m, nil
	case issueCreateFailedMsg:
		m.failIssueCreate(msg.temporaryID, msg.err)
		return m, nil
	case issueArchivedMsg:
		m.finishIssueArchive(msg.issueID)
		return m, nil
	case issueArchiveFailedMsg:
		m.failIssueArchive(msg.issueID, msg.err)
		return m, nil
	case issueFiltersLoadedMsg:
		m.finishIssueFilterLoad(msg)
		return m, nil
	case issueFiltersSavedMsg:
		m.finishIssueFilterSave(msg)
		return m, nil
	case tea.KeyPressMsg:
		cmd := m.updateKey(msg)
		return m, cmd
	}
	return m, nil
}
