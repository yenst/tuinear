package cache

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jihmy/tuinear/internal/linear"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func demoDashboard(t *testing.T) linear.Dashboard {
	t.Helper()
	dashboard, err := (linear.DemoClient{}).FetchDashboard(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return dashboard
}

func TestStoreRoundTripUsesNormalizedTables(t *testing.T) {
	store := openTestStore(t)
	want := demoDashboard(t)
	cachedAt := time.Date(2026, 9, 2, 10, 30, 0, 0, time.UTC)
	if err := store.Save(t.Context(), "work", want, cachedAt); err != nil {
		t.Fatal(err)
	}
	got, gotCachedAt, err := store.Load(t.Context(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if !gotCachedAt.Equal(cachedAt) {
		t.Fatalf("cached at = %v, want %v", gotCachedAt, cachedAt)
	}
	if got.Organization != want.Organization || got.Viewer != want.Viewer {
		t.Fatalf("identity did not round trip: %#v", got)
	}
	if len(got.Accounts) != 0 || got.ActiveAccountID != "" {
		t.Fatal("credential-store profile metadata must not be copied into SQLite")
	}
	if len(got.Teams) != len(want.Teams) || len(got.Users) != len(want.Users) || len(got.Issues) != len(want.Issues) {
		t.Fatalf("round trip counts = %d teams/%d users/%d issues", len(got.Teams), len(got.Users), len(got.Issues))
	}
	if gotStates := got.StatesForTeam(want.Teams[0].ID); len(gotStates) != len(want.StatesForTeam(want.Teams[0].ID)) || gotStates[0].ID == "" {
		t.Fatalf("team workflow states did not round trip: %#v", got.TeamStates)
	}
	if gotProjects := got.ProjectsForTeam(want.Teams[0].ID); len(gotProjects) != len(want.ProjectsForTeam(want.Teams[0].ID)) || gotProjects[0].ID == "" {
		t.Fatalf("team projects did not round trip: %#v", got.TeamProjects)
	}
	if gotLabels := got.LabelsForTeam(want.Teams[0].ID); len(gotLabels) != len(want.LabelsForTeam(want.Teams[0].ID)) || len(got.WorkspaceLabels) == 0 || len(got.TeamLabels) == 0 {
		t.Fatalf("editable labels did not round trip: workspace=%#v teams=%#v", got.WorkspaceLabels, got.TeamLabels)
	}
	for index, issue := range want.Issues {
		cached := got.Issues[index]
		if cached.ID != issue.ID || cached.Identifier != issue.Identifier || cached.Title != issue.Title ||
			cached.State != issue.State || cached.Team != issue.Team || !cached.UpdatedAt.Equal(issue.UpdatedAt) {
			t.Fatalf("issue %d did not round trip: %#v", index, cached)
		}
		if (cached.Assignee == nil) != (issue.Assignee == nil) || (cached.Project == nil) != (issue.Project == nil) {
			t.Fatalf("optional issue relations changed for %s", issue.Identifier)
		}
		if len(cached.Labels) != len(issue.Labels) {
			t.Fatalf("labels for %s = %#v", issue.Identifier, cached.Labels)
		}
	}

	for table, minimum := range map[string]int{"teams": 1, "workflow_states": 1, "team_workflow_states": 1, "users": 1, "projects": 1, "team_projects": 1, "issues": 1, "labels": 1, "workspace_labels": 1, "team_labels": 1, "issue_labels": 1} {
		var count int
		if err := store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count < minimum {
			t.Errorf("table %s has %d rows", table, count)
		}
	}
}

func TestStoreIsolatesAccountsAndAtomicallyReplacesSnapshots(t *testing.T) {
	store := openTestStore(t)
	work := demoDashboard(t)
	personal := demoDashboard(t)
	personal.Organization.Name = "Personal"
	personal.Issues = personal.Issues[:1]
	if err := store.Save(t.Context(), "work", work, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(t.Context(), "personal", personal, time.Now()); err != nil {
		t.Fatal(err)
	}
	updated := work
	updated.Issues = updated.Issues[1:]
	if err := store.Save(t.Context(), "work", updated, time.Now()); err != nil {
		t.Fatal(err)
	}
	gotWork, _, err := store.Load(t.Context(), "work")
	if err != nil {
		t.Fatal(err)
	}
	gotPersonal, _, err := store.Load(t.Context(), "personal")
	if err != nil {
		t.Fatal(err)
	}
	if len(gotWork.Issues) != len(updated.Issues) || gotWork.Issues[0].ID != updated.Issues[0].ID {
		t.Fatalf("work cache was not replaced: %#v", gotWork.Issues)
	}
	if gotPersonal.Organization.Name != "Personal" || len(gotPersonal.Issues) != 1 {
		t.Fatalf("personal cache was changed: %#v", gotPersonal)
	}
	if _, _, err := store.Load(t.Context(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing cache error = %v", err)
	}
}

func TestOpenMigratesFreshDatabase(t *testing.T) {
	store := openTestStore(t)
	var version int
	if err := store.db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("schema version = %d, want %d", version, schemaVersion)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("cache permissions = %o, want 600", got)
		}
	}
}

func TestOpenMigratesVersionOneDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range schemaStatements {
		if statement == createTeamWorkflowStatesTable || statement == createTeamProjectsTable || statement == createWorkspaceLabelsTable || statement == createTeamLabelsTable {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("migrated version = %d, %v", version, err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_workflow_states LIMIT 0"); err != nil {
		t.Fatalf("team workflow state table missing: %v", err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_projects LIMIT 0"); err != nil {
		t.Fatalf("team projects table missing: %v", err)
	}
	if _, err := store.db.Exec("SELECT * FROM workspace_labels LIMIT 0"); err != nil {
		t.Fatalf("workspace labels table missing: %v", err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_labels LIMIT 0"); err != nil {
		t.Fatalf("team labels table missing: %v", err)
	}
}

func TestOpenMigratesVersionTwoDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range schemaStatements {
		if statement == createTeamProjectsTable || statement == createWorkspaceLabelsTable || statement == createTeamLabelsTable {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("migrated version = %d, %v", version, err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_projects LIMIT 0"); err != nil {
		t.Fatalf("team projects table missing: %v", err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_labels LIMIT 0"); err != nil {
		t.Fatalf("team labels table missing: %v", err)
	}
}

func TestOpenMigratesVersionThreeDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range schemaStatements {
		if statement == createWorkspaceLabelsTable || statement == createTeamLabelsTable {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("migrated version = %d, %v", version, err)
	}
	if _, err := store.db.Exec("SELECT * FROM workspace_labels LIMIT 0"); err != nil {
		t.Fatalf("workspace labels table missing: %v", err)
	}
	if _, err := store.db.Exec("SELECT * FROM team_labels LIMIT 0"); err != nil {
		t.Fatalf("team labels table missing: %v", err)
	}
}

func TestOpenQuarantinesCorruptOrIncompatibleDatabase(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(string) error
	}{
		{name: "corrupt", setup: func(path string) error { return os.WriteFile(path, []byte("not a sqlite database"), 0o600) }},
		{name: "newer schema", setup: func(path string) error {
			db, err := sql.Open("sqlite", path)
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.Exec("PRAGMA user_version = 99")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache.sqlite3")
			if err := test.setup(path); err != nil {
				t.Fatal(err)
			}
			store, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if store.RecoveredFrom() == "" || !strings.Contains(filepath.Base(store.RecoveredFrom()), ".broken-") {
				t.Fatalf("recovered path = %q", store.RecoveredFrom())
			}
			if _, err := os.Stat(store.RecoveredFrom()); err != nil {
				t.Fatalf("quarantined database is missing: %v", err)
			}
			if _, _, err := store.Load(t.Context(), "work"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("rebuilt cache load error = %v", err)
			}
		})
	}
}
