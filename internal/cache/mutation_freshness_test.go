package cache

import (
	"testing"
	"time"

	"github.com/yenst/tuinear/internal/linear"
)

func TestIssueMutationsPreserveDashboardCacheAge(t *testing.T) {
	for _, operation := range []string{"update", "create", "archive"} {
		t.Run(operation, func(t *testing.T) {
			store := openTestStore(t)
			dashboard := demoDashboard(t)
			cachedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
			if err := store.Save(t.Context(), "work", dashboard, cachedAt); err != nil {
				t.Fatal(err)
			}
			issue := dashboard.Issues[0]
			issue.Title = "Confirmed change"
			created := issue
			created.ID = "new-issue"
			remote := &remoteStub{updated: issue, created: created}
			loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
			loader.now = func() time.Time { return cachedAt.Add(24 * time.Hour) }
			var err error
			switch operation {
			case "update":
				_, err = loader.UpdateIssue(t.Context(), issue.ID, linear.IssueUpdate{Title: &issue.Title})
			case "create":
				_, err = loader.CreateIssue(t.Context(), linear.IssueCreate{TeamID: issue.Team.ID, Title: created.Title})
			case "archive":
				err = loader.ArchiveIssue(t.Context(), issue.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, gotAt, err := store.Load(t.Context(), "work")
			if err != nil {
				t.Fatal(err)
			}
			if !gotAt.Equal(cachedAt) {
				t.Fatalf("single issue %s changed dashboard freshness from %v to %v", operation, cachedAt, gotAt)
			}
		})
	}
}
