package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("cache path is empty")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("protect cache directory: %w", err)
	}
	store, err := open(path)
	if err == nil {
		return store, nil
	}
	recovered, recoverErr := quarantine(path)
	if recoverErr != nil {
		return nil, fmt.Errorf("open cache: %w (recovery failed: %v)", err, recoverErr)
	}
	if recovered == "" {
		return nil, fmt.Errorf("open cache: %w", err)
	}
	store, retryErr := open(path)
	if retryErr != nil {
		return nil, fmt.Errorf("rebuild cache after moving %s: %w", recovered, retryErr)
	}
	store.recoveredFrom = recovered
	return store, nil
}

func open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		db.Close()
		if err != nil {
			return nil, fmt.Errorf("check cache integrity: %w", err)
		}
		return nil, fmt.Errorf("check cache integrity: %s", integrity)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("protect cache database: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read cache schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("cache schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == 0 {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin cache migration: %w", err)
		}
		defer tx.Rollback()
		for _, statement := range schemaStatements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("create cache schema: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("set cache schema version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit cache migration: %w", err)
		}
	}
	if version == 1 {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin cache migration: %w", err)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, createTeamWorkflowStatesTable); err != nil {
			return fmt.Errorf("add team workflow states: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("set cache schema version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit cache migration: %w", err)
		}
	}
	for _, query := range schemaValidationQueries {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("validate cache schema: %w", err)
		}
		rows.Close()
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE snapshots (
        account_key TEXT PRIMARY KEY,
        cached_at_ns INTEGER NOT NULL,
        organization_id TEXT NOT NULL,
        organization_name TEXT NOT NULL,
        organization_url_key TEXT NOT NULL,
        viewer_id TEXT NOT NULL,
        viewer_name TEXT NOT NULL,
        viewer_display_name TEXT NOT NULL,
        viewer_email TEXT NOT NULL
    )`,
	`CREATE TABLE teams (
        account_key TEXT NOT NULL,
        id TEXT NOT NULL,
        team_key TEXT NOT NULL,
        name TEXT NOT NULL,
        position INTEGER NOT NULL,
        PRIMARY KEY (account_key, id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	`CREATE TABLE workflow_states (
        account_key TEXT NOT NULL,
        cache_id TEXT NOT NULL,
        id TEXT NOT NULL,
        name TEXT NOT NULL,
        type TEXT NOT NULL,
        color TEXT NOT NULL,
        PRIMARY KEY (account_key, cache_id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	createTeamWorkflowStatesTable,
	`CREATE TABLE users (
        account_key TEXT NOT NULL,
        id TEXT NOT NULL,
        name TEXT NOT NULL,
        display_name TEXT NOT NULL,
        PRIMARY KEY (account_key, id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	`CREATE TABLE projects (
        account_key TEXT NOT NULL,
        id TEXT NOT NULL,
        name TEXT NOT NULL,
        PRIMARY KEY (account_key, id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	`CREATE TABLE issues (
        account_key TEXT NOT NULL,
        id TEXT NOT NULL,
        identifier TEXT NOT NULL,
        title TEXT NOT NULL,
        description TEXT NOT NULL,
        priority INTEGER NOT NULL,
        url TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        state_id TEXT NOT NULL,
        assignee_id TEXT,
        team_id TEXT NOT NULL,
        project_id TEXT,
        position INTEGER NOT NULL,
        PRIMARY KEY (account_key, id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	`CREATE TABLE labels (
        account_key TEXT NOT NULL,
        id TEXT NOT NULL,
        name TEXT NOT NULL,
        color TEXT NOT NULL,
        PRIMARY KEY (account_key, id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
	`CREATE TABLE issue_labels (
        account_key TEXT NOT NULL,
        issue_id TEXT NOT NULL,
        label_id TEXT NOT NULL,
        position INTEGER NOT NULL,
        PRIMARY KEY (account_key, issue_id, label_id),
        FOREIGN KEY (account_key) REFERENCES snapshots(account_key) ON DELETE CASCADE
    )`,
}

var schemaValidationQueries = []string{
	"SELECT account_key, cached_at_ns, organization_id, organization_name, organization_url_key, viewer_id, viewer_name, viewer_display_name, viewer_email FROM snapshots LIMIT 0",
	"SELECT account_key, id, team_key, name, position FROM teams LIMIT 0",
	"SELECT account_key, cache_id, id, name, type, color FROM workflow_states LIMIT 0",
	"SELECT account_key, team_id, state_cache_id, position FROM team_workflow_states LIMIT 0",
	"SELECT account_key, id, name, display_name FROM users LIMIT 0",
	"SELECT account_key, id, name FROM projects LIMIT 0",
	"SELECT account_key, id, identifier, title, description, priority, url, created_at, updated_at, state_id, assignee_id, team_id, project_id, position FROM issues LIMIT 0",
	"SELECT account_key, id, name, color FROM labels LIMIT 0",
	"SELECT account_key, issue_id, label_id, position FROM issue_labels LIMIT 0",
}

const createTeamWorkflowStatesTable = `CREATE TABLE team_workflow_states (
        account_key TEXT NOT NULL,
        team_id TEXT NOT NULL,
        state_cache_id TEXT NOT NULL,
        position INTEGER NOT NULL,
        PRIMARY KEY (account_key, team_id, state_cache_id),
        FOREIGN KEY (account_key, team_id) REFERENCES teams(account_key, id) ON DELETE CASCADE,
        FOREIGN KEY (account_key, state_cache_id) REFERENCES workflow_states(account_key, cache_id) ON DELETE CASCADE
    )`

func quarantine(path string) (string, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	backup := path + ".broken-" + time.Now().UTC().Format("20060102T150405.000000000")
	if err := os.Rename(path, backup); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); err == nil {
			_ = os.Rename(path+suffix, backup+suffix)
		}
	}
	return backup, nil
}
