package cache

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jihmy/tuinear/internal/linear"
)

var ErrNotFound = errors.New("cached dashboard not found")

type Store struct {
	db            *sql.DB
	path          string
	recoveredFrom string
}

func DefaultPath() (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(directory, "tuinear", "cache.sqlite3"), nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) RecoveredFrom() string {
	if s == nil {
		return ""
	}
	return s.recoveredFrom
}

func (s *Store) Save(ctx context.Context, accountKey string, dashboard linear.Dashboard, cachedAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return errors.New("cache account key is empty")
	}
	if cachedAt.IsZero() {
		cachedAt = time.Now()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cache update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM snapshots WHERE account_key = ?", accountKey); err != nil {
		return fmt.Errorf("replace cached dashboard: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO snapshots VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountKey, cachedAt.UnixNano(), dashboard.Organization.ID, dashboard.Organization.Name,
		dashboard.Organization.URLKey, dashboard.Viewer.ID, dashboard.Viewer.Name,
		dashboard.Viewer.DisplayName, dashboard.Viewer.Email); err != nil {
		return fmt.Errorf("store cached dashboard: %w", err)
	}

	teams := append([]linear.Team(nil), dashboard.Teams...)
	teamSeen := make(map[string]bool, len(teams))
	for _, team := range teams {
		teamSeen[team.ID] = true
	}
	for _, issue := range dashboard.Issues {
		if !teamSeen[issue.Team.ID] {
			teams = append(teams, issue.Team)
			teamSeen[issue.Team.ID] = true
		}
	}
	for position, team := range teams {
		if _, err := tx.ExecContext(ctx, `INSERT INTO teams VALUES (?, ?, ?, ?, ?)`, accountKey, team.ID, team.Key, team.Name, position); err != nil {
			return fmt.Errorf("store cached teams: %w", err)
		}
	}

	states := map[string]linear.WorkflowState{}
	users := map[string]linear.User{}
	projects := map[string]linear.Project{}
	labels := map[string]linear.Label{}
	for _, group := range dashboard.TeamStates {
		for _, state := range group.States {
			states[workflowStateKey(state)] = state
		}
	}
	for _, user := range dashboard.Users {
		if user.ID != "" {
			users[user.ID] = user
		}
	}
	for _, group := range dashboard.TeamProjects {
		for _, project := range group.Projects {
			if project.ID != "" {
				projects[project.ID] = project
			}
		}
	}
	for _, label := range dashboard.WorkspaceLabels {
		if label.ID != "" {
			labels[label.ID] = label
		}
	}
	for _, group := range dashboard.TeamLabels {
		for _, label := range group.Labels {
			if label.ID != "" {
				labels[label.ID] = label
			}
		}
	}
	for _, issue := range dashboard.Issues {
		states[workflowStateKey(issue.State)] = issue.State
		if issue.Assignee != nil {
			users[issue.Assignee.ID] = *issue.Assignee
		}
		if issue.Project != nil {
			projects[issue.Project.ID] = *issue.Project
		}
		for _, label := range issue.Labels {
			labels[label.ID] = label
		}
	}
	for cacheID, state := range states {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_states VALUES (?, ?, ?, ?, ?, ?)`, accountKey, cacheID, state.ID, state.Name, state.Type, state.Color); err != nil {
			return fmt.Errorf("store cached workflow states: %w", err)
		}
	}
	for _, group := range dashboard.TeamStates {
		if !teamSeen[group.TeamID] {
			continue
		}
		for position, state := range group.States {
			if _, err := tx.ExecContext(ctx, `INSERT INTO team_workflow_states VALUES (?, ?, ?, ?)`,
				accountKey, group.TeamID, workflowStateKey(state), position); err != nil {
				return fmt.Errorf("store cached team workflow states: %w", err)
			}
		}
	}
	for _, user := range users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users VALUES (?, ?, ?, ?)`, accountKey, user.ID, user.Name, user.DisplayName); err != nil {
			return fmt.Errorf("store cached users: %w", err)
		}
	}
	for _, project := range projects {
		if _, err := tx.ExecContext(ctx, `INSERT INTO projects VALUES (?, ?, ?)`, accountKey, project.ID, project.Name); err != nil {
			return fmt.Errorf("store cached projects: %w", err)
		}
	}
	for _, group := range dashboard.TeamProjects {
		if !teamSeen[group.TeamID] {
			continue
		}
		seen := make(map[string]bool, len(group.Projects))
		for position, project := range group.Projects {
			if project.ID == "" || seen[project.ID] {
				continue
			}
			seen[project.ID] = true
			if _, ok := projects[project.ID]; !ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO team_projects VALUES (?, ?, ?, ?)`,
				accountKey, group.TeamID, project.ID, position); err != nil {
				return fmt.Errorf("store cached team projects: %w", err)
			}
		}
	}
	for _, label := range labels {
		if _, err := tx.ExecContext(ctx, `INSERT INTO labels VALUES (?, ?, ?, ?)`, accountKey, label.ID, label.Name, label.Color); err != nil {
			return fmt.Errorf("store cached labels: %w", err)
		}
	}
	seenWorkspaceLabels := make(map[string]bool, len(dashboard.WorkspaceLabels))
	for position, label := range dashboard.WorkspaceLabels {
		if label.ID == "" || seenWorkspaceLabels[label.ID] {
			continue
		}
		seenWorkspaceLabels[label.ID] = true
		if _, ok := labels[label.ID]; !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO workspace_labels VALUES (?, ?, ?)`, accountKey, label.ID, position); err != nil {
			return fmt.Errorf("store cached workspace labels: %w", err)
		}
	}
	for _, group := range dashboard.TeamLabels {
		if !teamSeen[group.TeamID] {
			continue
		}
		seen := make(map[string]bool, len(group.Labels))
		for position, label := range group.Labels {
			if label.ID == "" || seen[label.ID] {
				continue
			}
			seen[label.ID] = true
			if _, ok := labels[label.ID]; !ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO team_labels VALUES (?, ?, ?, ?)`, accountKey, group.TeamID, label.ID, position); err != nil {
				return fmt.Errorf("store cached team labels: %w", err)
			}
		}
	}

	for position, issue := range dashboard.Issues {
		var assigneeID, projectID any
		if issue.Assignee != nil {
			assigneeID = issue.Assignee.ID
		}
		if issue.Project != nil {
			projectID = issue.Project.ID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issues VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountKey, issue.ID, issue.Identifier, issue.Title, issue.Description, issue.Priority, issue.URL,
			formatTime(issue.CreatedAt), formatTime(issue.UpdatedAt), workflowStateKey(issue.State), assigneeID, issue.Team.ID, projectID, position); err != nil {
			return fmt.Errorf("store cached issues: %w", err)
		}
		for labelPosition, label := range issue.Labels {
			if _, err := tx.ExecContext(ctx, `INSERT INTO issue_labels VALUES (?, ?, ?, ?)`, accountKey, issue.ID, label.ID, labelPosition); err != nil {
				return fmt.Errorf("store cached issue labels: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cached dashboard: %w", err)
	}
	return nil
}

func (s *Store) Load(ctx context.Context, accountKey string) (linear.Dashboard, time.Time, error) {
	if s == nil || s.db == nil {
		return linear.Dashboard{}, time.Time{}, errors.New("cache store is closed")
	}
	var dashboard linear.Dashboard
	var cachedAtNS int64
	err := s.db.QueryRowContext(ctx, `SELECT cached_at_ns, organization_id, organization_name, organization_url_key,
        viewer_id, viewer_name, viewer_display_name, viewer_email FROM snapshots WHERE account_key = ?`, accountKey).Scan(
		&cachedAtNS, &dashboard.Organization.ID, &dashboard.Organization.Name, &dashboard.Organization.URLKey,
		&dashboard.Viewer.ID, &dashboard.Viewer.Name, &dashboard.Viewer.DisplayName, &dashboard.Viewer.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return linear.Dashboard{}, time.Time{}, ErrNotFound
	}
	if err != nil {
		return linear.Dashboard{}, time.Time{}, fmt.Errorf("load cached dashboard: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, team_key, name FROM teams WHERE account_key = ? ORDER BY position`, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, fmt.Errorf("load cached teams: %w", err)
	}
	for rows.Next() {
		var team linear.Team
		if err := rows.Scan(&team.ID, &team.Key, &team.Name); err != nil {
			rows.Close()
			return linear.Dashboard{}, time.Time{}, fmt.Errorf("read cached team: %w", err)
		}
		dashboard.Teams = append(dashboard.Teams, team)
	}
	if err := rows.Close(); err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}

	states, err := loadStates(ctx, s.db, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	dashboard.TeamStates, err = loadTeamWorkflowStates(ctx, s.db, accountKey, dashboard.Teams, states)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	users, err := loadUsers(ctx, s.db, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	for _, user := range users {
		dashboard.Users = append(dashboard.Users, user)
	}
	sort.SliceStable(dashboard.Users, func(i, j int) bool {
		return strings.ToLower(dashboard.Users[i].Label()) < strings.ToLower(dashboard.Users[j].Label())
	})
	projects, err := loadProjects(ctx, s.db, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	dashboard.TeamProjects, err = loadTeamProjects(ctx, s.db, accountKey, dashboard.Teams, projects)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	labels, err := loadLabels(ctx, s.db, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	dashboard.WorkspaceLabels, err = loadWorkspaceLabels(ctx, s.db, accountKey, labels)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	dashboard.TeamLabels, err = loadTeamLabels(ctx, s.db, accountKey, dashboard.Teams, labels)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	issueLabels, err := loadIssueLabels(ctx, s.db, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	teamByID := make(map[string]linear.Team, len(dashboard.Teams))
	for _, team := range dashboard.Teams {
		teamByID[team.ID] = team
	}

	rows, err = s.db.QueryContext(ctx, `SELECT id, identifier, title, description, priority, url, created_at, updated_at,
        state_id, assignee_id, team_id, project_id FROM issues WHERE account_key = ? ORDER BY position`, accountKey)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, fmt.Errorf("load cached issues: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var issue linear.Issue
		var createdAt, updatedAt string
		var assigneeID, projectID sql.NullString
		var stateID, teamID string
		if err := rows.Scan(&issue.ID, &issue.Identifier, &issue.Title, &issue.Description, &issue.Priority, &issue.URL,
			&createdAt, &updatedAt, &stateID, &assigneeID, &teamID, &projectID); err != nil {
			return linear.Dashboard{}, time.Time{}, fmt.Errorf("read cached issue: %w", err)
		}
		issue.CreatedAt = parseTime(createdAt)
		issue.UpdatedAt = parseTime(updatedAt)
		issue.State = states[stateID]
		issue.Team = teamByID[teamID]
		if assigneeID.Valid {
			assignee := users[assigneeID.String]
			issue.Assignee = &assignee
		}
		if projectID.Valid {
			project := projects[projectID.String]
			issue.Project = &project
		}
		issue.Labels = issueLabels[issue.ID]
		dashboard.Issues = append(dashboard.Issues, issue)
	}
	if err := rows.Err(); err != nil {
		return linear.Dashboard{}, time.Time{}, fmt.Errorf("read cached issues: %w", err)
	}
	return dashboard, time.Unix(0, cachedAtNS), nil
}

func loadStates(ctx context.Context, db *sql.DB, accountKey string) (map[string]linear.WorkflowState, error) {
	rows, err := db.QueryContext(ctx, `SELECT cache_id, id, name, type, color FROM workflow_states WHERE account_key = ?`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached workflow states: %w", err)
	}
	defer rows.Close()
	values := map[string]linear.WorkflowState{}
	for rows.Next() {
		var cacheID string
		var value linear.WorkflowState
		if err := rows.Scan(&cacheID, &value.ID, &value.Name, &value.Type, &value.Color); err != nil {
			return nil, err
		}
		values[cacheID] = value
	}
	return values, rows.Err()
}

func loadTeamWorkflowStates(ctx context.Context, db *sql.DB, accountKey string, teams []linear.Team, states map[string]linear.WorkflowState) ([]linear.TeamWorkflowStates, error) {
	rows, err := db.QueryContext(ctx, `SELECT team_id, state_cache_id FROM team_workflow_states
        WHERE account_key = ? ORDER BY team_id, position`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached team workflow states: %w", err)
	}
	defer rows.Close()
	byTeam := make(map[string][]linear.WorkflowState, len(teams))
	for rows.Next() {
		var teamID, stateID string
		if err := rows.Scan(&teamID, &stateID); err != nil {
			return nil, err
		}
		if state, ok := states[stateID]; ok {
			byTeam[teamID] = append(byTeam[teamID], state)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	groups := make([]linear.TeamWorkflowStates, 0, len(byTeam))
	for _, team := range teams {
		if values := byTeam[team.ID]; len(values) > 0 {
			groups = append(groups, linear.TeamWorkflowStates{TeamID: team.ID, States: values})
		}
	}
	return groups, nil
}

func loadUsers(ctx context.Context, db *sql.DB, accountKey string) (map[string]linear.User, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, display_name FROM users WHERE account_key = ?`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached users: %w", err)
	}
	defer rows.Close()
	values := map[string]linear.User{}
	for rows.Next() {
		var value linear.User
		if err := rows.Scan(&value.ID, &value.Name, &value.DisplayName); err != nil {
			return nil, err
		}
		values[value.ID] = value
	}
	return values, rows.Err()
}

func loadProjects(ctx context.Context, db *sql.DB, accountKey string) (map[string]linear.Project, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name FROM projects WHERE account_key = ?`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached projects: %w", err)
	}
	defer rows.Close()
	values := map[string]linear.Project{}
	for rows.Next() {
		var value linear.Project
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, err
		}
		values[value.ID] = value
	}
	return values, rows.Err()
}

func loadTeamProjects(ctx context.Context, db *sql.DB, accountKey string, teams []linear.Team, projects map[string]linear.Project) ([]linear.TeamProjects, error) {
	rows, err := db.QueryContext(ctx, `SELECT team_id, project_id FROM team_projects
        WHERE account_key = ? ORDER BY team_id, position`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached team projects: %w", err)
	}
	defer rows.Close()
	byTeam := make(map[string][]linear.Project, len(teams))
	for rows.Next() {
		var teamID, projectID string
		if err := rows.Scan(&teamID, &projectID); err != nil {
			return nil, err
		}
		if project, ok := projects[projectID]; ok {
			byTeam[teamID] = append(byTeam[teamID], project)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	groups := make([]linear.TeamProjects, 0, len(byTeam))
	for _, team := range teams {
		if values := byTeam[team.ID]; len(values) > 0 {
			groups = append(groups, linear.TeamProjects{TeamID: team.ID, Projects: values})
		}
	}
	return groups, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func workflowStateKey(state linear.WorkflowState) string {
	if state.ID != "" {
		return state.ID
	}
	sum := sha256.Sum256([]byte(state.Name + "\x00" + state.Type + "\x00" + state.Color))
	return "missing:" + hex.EncodeToString(sum[:8])
}
