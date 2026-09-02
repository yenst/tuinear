package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/issuefilter"
	"github.com/jihmy/tuinear/internal/linear"
)

type IssueFilterStore interface {
	LoadIssueFilters(context.Context, string) (issuefilter.State, error)
	SaveIssueFilters(context.Context, string, issuefilter.State, uint64) error
}

type issueFiltersLoadedMsg struct {
	profileKey string
	revision   uint64
	filters    issuefilter.State
	err        error
}

type issueFiltersSavedMsg struct {
	profileKey string
	revision   uint64
	err        error
}

func issueFilterProfileKey(dashboard linear.Dashboard) string {
	if accountID := strings.TrimSpace(dashboard.ActiveAccountID); accountID != "" {
		return "profile:" + accountID
	}
	organizationID := strings.TrimSpace(dashboard.Organization.ID)
	viewerID := strings.TrimSpace(dashboard.Viewer.ID)
	if organizationID == "" && viewerID == "" {
		return ""
	}
	return "viewer:" + organizationID + ":" + viewerID
}

func (m *Model) syncIssueFilterProfile() {
	profileKey := issueFilterProfileKey(m.dashboard)
	if profileKey == m.filterProfileKey {
		return
	}
	m.filterProfileKey = profileKey
	m.filters = IssueFilters{}
	m.filterPreferencesLoading = false
	m.filterPreferencesLoaded = m.filterStore == nil || profileKey == ""
	m.filterErr = nil
	m.filterIssues()
}

func (m *Model) loadIssueFilters() tea.Cmd {
	if m.filterStore == nil || m.filterProfileKey == "" || m.filterPreferencesLoaded || m.filterPreferencesLoading {
		return nil
	}
	m.filterPreferencesLoading = true
	store := m.filterStore
	profileKey := m.filterProfileKey
	revision := m.filterRevision
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		filters, err := store.LoadIssueFilters(ctx, profileKey)
		if err != nil {
			err = fmt.Errorf("load saved filters: %w", err)
		}
		return issueFiltersLoadedMsg{profileKey: profileKey, revision: revision, filters: filters, err: err}
	}
}

func (m *Model) finishIssueFilterLoad(msg issueFiltersLoadedMsg) {
	if msg.profileKey != m.filterProfileKey {
		return
	}
	m.filterPreferencesLoading = false
	m.filterPreferencesLoaded = true
	if msg.err != nil {
		m.filterErr = msg.err
		return
	}
	if msg.revision != m.filterRevision {
		return
	}
	m.filters = msg.filters.Clone()
	m.filterErr = nil
	m.filterIssues()
}

func (m *Model) saveIssueFilters() tea.Cmd {
	if m.filterStore == nil || m.filterProfileKey == "" {
		return nil
	}
	m.filterRevision++
	revision := m.filterRevision
	profileKey := m.filterProfileKey
	filters := m.filters.Clone()
	store := m.filterStore
	m.filterErr = nil
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := store.SaveIssueFilters(ctx, profileKey, filters, revision)
		if err != nil {
			err = fmt.Errorf("save filters: %w", err)
		}
		return issueFiltersSavedMsg{profileKey: profileKey, revision: revision, err: err}
	}
}

func (m *Model) finishIssueFilterSave(msg issueFiltersSavedMsg) {
	if msg.profileKey != m.filterProfileKey || msg.revision != m.filterRevision {
		return
	}
	m.filterErr = msg.err
}
