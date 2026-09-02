package main

import (
	"context"
	"fmt"

	"github.com/jihmy/tuinear/internal/auth"
	"github.com/jihmy/tuinear/internal/linear"
)

type accountLoader struct {
	manager *auth.Manager
}

func newAccountLoader(manager *auth.Manager) *accountLoader {
	return &accountLoader{manager: manager}
}

func (l *accountLoader) FetchDashboard(ctx context.Context) (linear.Dashboard, error) {
	if l == nil || l.manager == nil {
		return linear.Dashboard{}, fmt.Errorf("account loader is not configured")
	}
	dashboard, err := linear.NewOAuthClient(l.manager).FetchDashboard(ctx)
	if err != nil {
		return linear.Dashboard{}, err
	}
	return l.DecorateDashboard(dashboard)
}

func (l *accountLoader) DecorateDashboard(dashboard linear.Dashboard) (linear.Dashboard, error) {
	if l == nil || l.manager == nil {
		return linear.Dashboard{}, fmt.Errorf("account loader is not configured")
	}
	profiles, err := l.manager.Profiles()
	if err != nil {
		return linear.Dashboard{}, fmt.Errorf("load account profiles: %w", err)
	}
	activeID, err := l.manager.ActiveProfileID()
	if err != nil {
		return linear.Dashboard{}, fmt.Errorf("load active account: %w", err)
	}
	dashboard.Accounts = make([]linear.Account, 0, len(profiles))
	for _, profile := range profiles {
		dashboard.Accounts = append(dashboard.Accounts, linear.Account{
			ID:            profile.ID,
			WorkspaceName: profile.WorkspaceName,
			WorkspaceKey:  profile.WorkspaceKey,
			UserName:      profile.UserName,
			UserEmail:     profile.UserEmail,
		})
	}
	dashboard.ActiveAccountID = activeID
	return dashboard, nil
}

func (l *accountLoader) SwitchAccount(ctx context.Context, accountID string) (linear.Dashboard, error) {
	if l == nil || l.manager == nil {
		return linear.Dashboard{}, fmt.Errorf("account loader is not configured")
	}
	if err := l.manager.SelectProfile(accountID); err != nil {
		return linear.Dashboard{}, fmt.Errorf("select account: %w", err)
	}
	return l.FetchDashboard(ctx)
}

func (l *accountLoader) UpdateIssue(ctx context.Context, issueID string, update linear.IssueUpdate) (linear.Issue, error) {
	if l == nil || l.manager == nil {
		return linear.Issue{}, fmt.Errorf("account loader is not configured")
	}
	granted, err := l.manager.HasScope("write")
	if err != nil {
		return linear.Issue{}, fmt.Errorf("check Linear write access: %w", err)
	}
	if !granted {
		return linear.Issue{}, fmt.Errorf("this account is connected read-only; reconnect it with tuinear --login to grant write access")
	}
	return linear.NewOAuthClient(l.manager).UpdateIssue(ctx, issueID, update)
}
