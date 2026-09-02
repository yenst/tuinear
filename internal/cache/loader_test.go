package cache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jihmy/tuinear/internal/issuefilter"
	"github.com/jihmy/tuinear/internal/linear"
)

type remoteStub struct {
	dashboard  linear.Dashboard
	err        error
	fetches    int
	updated    linear.Issue
	updateErr  error
	updates    int
	archiveErr error
	archives   int
	archivedID string
}

func (s *remoteStub) ArchiveIssue(_ context.Context, issueID string) error {
	s.archives++
	s.archivedID = issueID
	return s.archiveErr
}

func (s *remoteStub) FetchDashboard(context.Context) (linear.Dashboard, error) {
	s.fetches++
	return s.dashboard, s.err
}

func (s *remoteStub) UpdateIssue(_ context.Context, _ string, _ linear.IssueUpdate) (linear.Issue, error) {
	s.updates++
	return s.updated, s.updateErr
}

func TestLoaderReadsCacheBeforeRemoteAndSynchronizes(t *testing.T) {
	store := openTestStore(t)
	cached := demoDashboard(t)
	cached.Organization.Name = "Cached"
	cachedAt := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	if err := store.Save(t.Context(), "work", cached, cachedAt); err != nil {
		t.Fatal(err)
	}
	remoteDashboard := demoDashboard(t)
	remoteDashboard.Organization.Name = "Fresh"
	remote := &remoteStub{dashboard: remoteDashboard}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	loader.now = func() time.Time { return cachedAt.Add(time.Hour) }

	got, gotAt, err := loader.LoadCachedDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Organization.Name != "Cached" || !gotAt.Equal(cachedAt) || remote.fetches != 0 {
		t.Fatalf("cached load = %#v at %v, remote fetches %d", got.Organization, gotAt, remote.fetches)
	}
	got, err = loader.FetchDashboard(t.Context())
	if err != nil || got.Organization.Name != "Fresh" || remote.fetches != 1 {
		t.Fatalf("remote refresh = %#v, %v, fetches %d", got.Organization, err, remote.fetches)
	}
	stored, storedAt, err := store.Load(t.Context(), "work")
	if err != nil || stored.Organization.Name != "Fresh" || !storedAt.Equal(cachedAt.Add(time.Hour)) {
		t.Fatalf("synchronized cache = %#v at %v, %v", stored.Organization, storedAt, err)
	}
}

func TestLoaderReturnsRemoteDataWhenCacheWriteFails(t *testing.T) {
	store := openTestStore(t)
	remote := &remoteStub{dashboard: demoDashboard(t)}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := loader.FetchDashboard(t.Context())
	if err != nil || len(got.Issues) == 0 {
		t.Fatalf("remote data should survive a cache write failure: %#v, %v", got, err)
	}
}

func TestAPIKeyCacheKeyIsStableWithoutContainingCredential(t *testing.T) {
	first := APIKeyCacheKey(" secret-token ")
	second := APIKeyCacheKey("secret-token")
	if first != second || strings.Contains(first, "secret-token") {
		t.Fatalf("unsafe or unstable API-key cache key %q / %q", first, second)
	}
	if first == APIKeyCacheKey("other-token") {
		t.Fatal("different API keys share a cache namespace")
	}
}

func TestLoaderPropagatesRemoteFailureWithoutReplacingCache(t *testing.T) {
	store := openTestStore(t)
	cached := demoDashboard(t)
	if err := store.Save(t.Context(), "work", cached, time.Now()); err != nil {
		t.Fatal(err)
	}
	remote := &remoteStub{err: errors.New("offline")}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	if _, err := loader.FetchDashboard(t.Context()); err == nil {
		t.Fatal("remote failure was hidden")
	}
	got, _, err := store.Load(t.Context(), "work")
	if err != nil || len(got.Issues) != len(cached.Issues) {
		t.Fatalf("last-known-good cache changed: %d issues, %v", len(got.Issues), err)
	}
}

func TestLoaderCachesConfirmedIssueUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	updated.Title = "Confirmed by Linear"
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	title := updated.Title
	got, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{Title: &title})
	if err != nil || got.Title != updated.Title || remote.updates != 1 {
		t.Fatalf("UpdateIssue = %#v, %v, updates=%d", got, err, remote.updates)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Title != updated.Title {
		t.Fatalf("cached update = %q, %v", cached.Issues[0].Title, err)
	}
}

func TestLoaderCachesConfirmedStatusUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	states := dashboard.StatesForTeam(updated.Team.ID)
	updated.State = states[len(states)-1]
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	stateID := updated.State.ID
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{StateID: &stateID}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].State.ID != updated.State.ID {
		t.Fatalf("cached status = %#v, %v", cached.Issues[0].State, err)
	}
}

func TestLoaderCachesConfirmedPriorityUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	updated.Priority = 1
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	priority := updated.Priority
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{Priority: &priority}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Priority != updated.Priority {
		t.Fatalf("cached priority = %d, %v", cached.Issues[0].Priority, err)
	}
}

func TestLoaderCachesConfirmedAssigneeUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	updated.Assignee = &dashboard.Users[0]
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	assigneeID := updated.Assignee.ID
	selected := &assigneeID
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{AssigneeID: &selected}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Assignee == nil || cached.Issues[0].Assignee.ID != updated.Assignee.ID {
		t.Fatalf("cached assignee = %#v, %v", cached.Issues[0].Assignee, err)
	}
}

func TestLoaderCachesConfirmedProjectUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	projects := dashboard.ProjectsForTeam(updated.Team.ID)
	updated.Project = &projects[len(projects)-1]
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	projectID := updated.Project.ID
	selected := &projectID
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{ProjectID: &selected}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Project == nil || cached.Issues[0].Project.ID != updated.Project.ID {
		t.Fatalf("cached project = %#v, %v", cached.Issues[0].Project, err)
	}
}

func TestLoaderCachesConfirmedLabelUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	updated.Labels = dashboard.LabelsForTeam(updated.Team.ID)
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	labelIDs := make([]string, 0, len(updated.Labels))
	for _, label := range updated.Labels {
		labelIDs = append(labelIDs, label.ID)
	}
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{LabelIDs: &labelIDs}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || len(cached.Issues[0].Labels) != len(updated.Labels) {
		t.Fatalf("cached labels = %#v, %v", cached.Issues[0].Labels, err)
	}
}

func TestLoaderCachesConfirmedDescriptionUpdate(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := dashboard.Issues[0]
	updated.Description = "# Confirmed\n\nCached markdown"
	remote := &remoteStub{updated: updated}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	description := updated.Description
	if _, err := loader.UpdateIssue(t.Context(), updated.ID, linear.IssueUpdate{Description: &description}); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Description != updated.Description {
		t.Fatalf("cached description = %q, %v", cached.Issues[0].Description, err)
	}
}

func TestLoaderDoesNotChangeCacheWhenIssueUpdateFails(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	original := dashboard.Issues[0].Title
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	remote := &remoteStub{updateErr: errors.New("permission denied")}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	title := "Must not be cached"
	if _, err := loader.UpdateIssue(t.Context(), dashboard.Issues[0].ID, linear.IssueUpdate{Title: &title}); err == nil {
		t.Fatal("remote update failure was hidden")
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || cached.Issues[0].Title != original {
		t.Fatalf("failed update changed cache to %q, %v", cached.Issues[0].Title, err)
	}
}

func TestLoaderRemovesOnlyConfirmedArchivedIssueFromCache(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	remote := &remoteStub{}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	archivedID := dashboard.Issues[0].ID
	if err := loader.ArchiveIssue(t.Context(), archivedID); err != nil {
		t.Fatal(err)
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || len(cached.Issues) != len(dashboard.Issues)-1 || remote.archives != 1 || remote.archivedID != archivedID {
		t.Fatalf("archive cache = %d issues, %v; remote=%d/%q", len(cached.Issues), err, remote.archives, remote.archivedID)
	}
	for _, issue := range cached.Issues {
		if issue.ID == archivedID {
			t.Fatal("confirmed archived issue remains cached")
		}
	}
}

func TestLoaderKeepsCacheWhenArchiveFails(t *testing.T) {
	store := openTestStore(t)
	dashboard := demoDashboard(t)
	if err := store.Save(t.Context(), "work", dashboard, time.Now()); err != nil {
		t.Fatal(err)
	}
	remote := &remoteStub{archiveErr: errors.New("archive forbidden")}
	loader := NewLoader(store, remote, func() (string, error) { return "work", nil })
	if err := loader.ArchiveIssue(t.Context(), dashboard.Issues[0].ID); err == nil {
		t.Fatal("archive failure was hidden")
	}
	cached, _, err := store.Load(t.Context(), "work")
	if err != nil || len(cached.Issues) != len(dashboard.Issues) {
		t.Fatalf("failed archive changed cache: %d issues, %v", len(cached.Issues), err)
	}
}

func TestLoaderIgnoresStaleFilterSaveRevision(t *testing.T) {
	store := openTestStore(t)
	loader := NewLoader(store, &remoteStub{}, func() (string, error) { return "work", nil })
	newer := issuefilter.State{ExcludedStatuses: []string{"completed", "canceled"}}
	if err := loader.SaveIssueFilters(t.Context(), "profile:work", newer, 2); err != nil {
		t.Fatal(err)
	}
	if err := loader.SaveIssueFilters(t.Context(), "profile:work", issuefilter.State{Status: "Todo"}, 1); err != nil {
		t.Fatal(err)
	}
	got, err := loader.LoadIssueFilters(t.Context(), "profile:work")
	if err != nil || got.Status != "" || len(got.ExcludedStatuses) != 2 {
		t.Fatalf("filters after stale save = %#v, %v", got, err)
	}
}
