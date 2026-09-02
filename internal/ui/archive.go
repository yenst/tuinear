package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type archiveConfirmation struct {
	issueID    string
	identifier string
	title      string
	selected   int
}

type IssueArchiver interface {
	ArchiveIssue(context.Context, string) error
}

type issueArchivedMsg struct{ issueID string }

type issueArchiveFailedMsg struct {
	issueID string
	err     error
}

type pendingIssueArchive struct {
	issueID    string
	identifier string
}

func (m *Model) beginArchiveConfirmation() {
	if m.pendingArchive != nil {
		m.editErr = fmt.Errorf("wait for %s to finish archiving", m.pendingArchive.identifier)
		return
	}
	if m.pendingEdit != nil {
		m.editErr = fmt.Errorf("wait for %s to finish saving", m.pendingEdit.identifier)
		return
	}
	issue := m.selectedIssue()
	if issue == nil {
		return
	}
	if m.issueArchiver == nil {
		m.editErr = fmt.Errorf("issue archiving is not available for this data source")
		return
	}
	m.archiveConfirm = &archiveConfirmation{
		issueID: issue.ID, identifier: issue.Identifier, title: issue.Title,
	}
	m.editErr = nil
	m.browserErr = nil
}

func (m *Model) updateArchiveConfirmation(msg tea.KeyPressMsg) tea.Cmd {
	confirmation := m.archiveConfirm
	if confirmation == nil {
		return nil
	}
	switch msg.String() {
	case "esc", "q", "n":
		m.archiveConfirm = nil
	case "j", "down", "right", "tab":
		confirmation.selected = 1
	case "k", "up", "left", "shift+tab":
		confirmation.selected = 0
	case "enter", "return":
		if confirmation.selected == 0 {
			m.archiveConfirm = nil
			return nil
		}
		return m.submitIssueArchive()
	}
	return nil
}

func (m *Model) submitIssueArchive() tea.Cmd {
	if m.archiveConfirm == nil || m.issueArchiver == nil {
		return nil
	}
	confirmation := m.archiveConfirm
	issue, ok := m.dashboardIssue(confirmation.issueID)
	if !ok || issue.Identifier != confirmation.identifier {
		m.archiveConfirm = nil
		m.editErr = fmt.Errorf("the issue selected for archiving is no longer available")
		return nil
	}
	m.pendingArchive = &pendingIssueArchive{issueID: issue.ID, identifier: issue.Identifier}
	m.archiveConfirm = nil
	m.editErr = nil
	return archiveIssue(m.issueArchiver, issue.ID, issue.Identifier)
}

func archiveIssue(archiver IssueArchiver, issueID, identifier string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := archiver.ArchiveIssue(ctx, issueID); err != nil {
			return issueArchiveFailedMsg{issueID: issueID, err: fmt.Errorf("archive %s: %w", identifier, err)}
		}
		return issueArchivedMsg{issueID: issueID}
	}
}

func (m *Model) finishIssueArchive(issueID string) {
	if m.pendingArchive == nil || m.pendingArchive.issueID != issueID {
		return
	}
	issues := m.dashboard.Issues[:0]
	for _, issue := range m.dashboard.Issues {
		if issue.ID != issueID {
			issues = append(issues, issue)
		}
	}
	m.dashboard.Issues = issues
	m.pendingArchive = nil
	m.editErr = nil
	m.fromCache = false
	m.cachedAt = time.Time{}
	m.filterIssues()
}

func (m *Model) failIssueArchive(issueID string, err error) {
	if m.pendingArchive == nil || m.pendingArchive.issueID != issueID {
		return
	}
	m.pendingArchive = nil
	m.editErr = err
}

func (m Model) renderArchiveConfirmation(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.archiveConfirm == nil {
		return panel("Archive issue", width, height, fitLines(nil, innerHeight))
	}
	confirmation := m.archiveConfirm
	lines := []string{
		lipgloss.NewStyle().Foreground(theme.yellow).Bold(true).Render("Archive " + confirmation.identifier + "?"),
		lipgloss.NewStyle().Foreground(theme.text).Render(clip(confirmation.title, innerWidth)),
		"",
		mutedStyle.Render("This removes the issue from active views."),
		mutedStyle.Render("It remains recoverable from Linear's archive."),
		"",
	}
	for index, label := range []string{"Cancel", "Archive issue"} {
		prefix := "  "
		if index == confirmation.selected {
			prefix = "› "
		}
		line := prefix + label
		if index == confirmation.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Archive issue", width, height, fitLines(lines, innerHeight))
}
