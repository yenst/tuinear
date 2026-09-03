package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yenst/tuinear/internal/issuefilter"
)

func (s *Store) LoadIssueFilters(ctx context.Context, profileKey string) (issuefilter.State, error) {
	if s == nil || s.db == nil {
		return issuefilter.State{}, errors.New("cache store is not configured")
	}
	profileKey = strings.TrimSpace(profileKey)
	if profileKey == "" {
		return issuefilter.State{}, errors.New("filter profile key is empty")
	}
	var encoded string
	err := s.db.QueryRowContext(ctx,
		"SELECT filters_json FROM issue_filter_preferences WHERE profile_key = ?", profileKey).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return issuefilter.State{}, nil
	}
	if err != nil {
		return issuefilter.State{}, fmt.Errorf("load saved issue filters: %w", err)
	}
	var filters issuefilter.State
	if err := json.Unmarshal([]byte(encoded), &filters); err != nil {
		return issuefilter.State{}, fmt.Errorf("decode saved issue filters: %w", err)
	}
	return filters, nil
}

func (s *Store) SaveIssueFilters(ctx context.Context, profileKey string, filters issuefilter.State) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is not configured")
	}
	profileKey = strings.TrimSpace(profileKey)
	if profileKey == "" {
		return errors.New("filter profile key is empty")
	}
	if filters.Empty() {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM issue_filter_preferences WHERE profile_key = ?", profileKey); err != nil {
			return fmt.Errorf("clear saved issue filters: %w", err)
		}
		return nil
	}
	encoded, err := json.Marshal(filters.Clone())
	if err != nil {
		return fmt.Errorf("encode saved issue filters: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO issue_filter_preferences (profile_key, filters_json)
        VALUES (?, ?) ON CONFLICT(profile_key) DO UPDATE SET filters_json = excluded.filters_json`, profileKey, string(encoded)); err != nil {
		return fmt.Errorf("save issue filters: %w", err)
	}
	return nil
}
