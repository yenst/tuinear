package cache

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jihmy/tuinear/internal/linear"
)

func loadLabels(ctx context.Context, db *sql.DB, accountKey string) (map[string]linear.Label, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, color FROM labels WHERE account_key = ?`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached labels: %w", err)
	}
	defer rows.Close()
	values := map[string]linear.Label{}
	for rows.Next() {
		var value linear.Label
		if err := rows.Scan(&value.ID, &value.Name, &value.Color); err != nil {
			return nil, err
		}
		values[value.ID] = value
	}
	return values, rows.Err()
}

func loadWorkspaceLabels(ctx context.Context, db *sql.DB, accountKey string, labels map[string]linear.Label) ([]linear.Label, error) {
	rows, err := db.QueryContext(ctx, `SELECT label_id FROM workspace_labels WHERE account_key = ? ORDER BY position`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached workspace labels: %w", err)
	}
	defer rows.Close()
	var values []linear.Label
	for rows.Next() {
		var labelID string
		if err := rows.Scan(&labelID); err != nil {
			return nil, err
		}
		if label, ok := labels[labelID]; ok {
			values = append(values, label)
		}
	}
	return values, rows.Err()
}

func loadTeamLabels(ctx context.Context, db *sql.DB, accountKey string, teams []linear.Team, labels map[string]linear.Label) ([]linear.TeamLabels, error) {
	rows, err := db.QueryContext(ctx, `SELECT team_id, label_id FROM team_labels
        WHERE account_key = ? ORDER BY team_id, position`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached team labels: %w", err)
	}
	defer rows.Close()
	byTeam := make(map[string][]linear.Label, len(teams))
	for rows.Next() {
		var teamID, labelID string
		if err := rows.Scan(&teamID, &labelID); err != nil {
			return nil, err
		}
		if label, ok := labels[labelID]; ok {
			byTeam[teamID] = append(byTeam[teamID], label)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	groups := make([]linear.TeamLabels, 0, len(byTeam))
	for _, team := range teams {
		if values := byTeam[team.ID]; len(values) > 0 {
			groups = append(groups, linear.TeamLabels{TeamID: team.ID, Labels: values})
		}
	}
	return groups, nil
}

func loadIssueLabels(ctx context.Context, db *sql.DB, accountKey string) (map[string][]linear.Label, error) {
	rows, err := db.QueryContext(ctx, `SELECT il.issue_id, l.id, l.name, l.color FROM issue_labels il
        JOIN labels l ON l.account_key = il.account_key AND l.id = il.label_id
        WHERE il.account_key = ? ORDER BY il.issue_id, il.position`, accountKey)
	if err != nil {
		return nil, fmt.Errorf("load cached labels: %w", err)
	}
	defer rows.Close()
	values := map[string][]linear.Label{}
	for rows.Next() {
		var issueID string
		var value linear.Label
		if err := rows.Scan(&issueID, &value.ID, &value.Name, &value.Color); err != nil {
			return nil, err
		}
		values[issueID] = append(values[issueID], value)
	}
	return values, rows.Err()
}
