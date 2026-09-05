package ui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestConfirmedMutationsPreserveCachedDashboardStatus(t *testing.T) {
	for _, operation := range []string{"update", "create", "archive"} {
		t.Run(operation, func(t *testing.T) {
			m := NewWithDashboard(editorDashboard(t))
			m.fromCache = true
			m.cachedAt = time.Now().Add(-24 * time.Hour)
			m.refreshErr = errors.New("dashboard refresh failed")
			cachedAt := m.cachedAt
			issue := m.dashboard.Issues[0]
			var msg tea.Msg
			switch operation {
			case "update":
				m.pendingEdit = &pendingIssueEdit{issueID: issue.ID, before: issue}
				issue.Title = "Confirmed title"
				msg = issueUpdatedMsg{issue: issue}
			case "create":
				m.pendingCreate = &pendingIssueCreate{temporaryID: "temporary"}
				issue.ID = "created"
				msg = issueCreatedMsg{issue: issue}
			case "archive":
				m.pendingArchive = &pendingIssueArchive{issueID: issue.ID}
				msg = issueArchivedMsg{issueID: issue.ID}
			}
			updated, _ := m.Update(msg)
			m = updated.(Model)
			if !m.fromCache || !m.cachedAt.Equal(cachedAt) || m.refreshErr == nil {
				t.Fatalf("%s hid stale dashboard state: cached=%v at=%v error=%v", operation, m.fromCache, m.cachedAt, m.refreshErr)
			}
		})
	}
}
