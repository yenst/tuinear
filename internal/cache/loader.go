package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/yenst/tuinear/internal/issuefilter"
	"github.com/yenst/tuinear/internal/linear"
)

type DashboardLoader interface {
	FetchDashboard(context.Context) (linear.Dashboard, error)
}

type AccountSwitcher interface {
	SwitchAccount(context.Context, string) (linear.Dashboard, error)
}

type AccountSelector interface {
	SelectAccount(string) error
	ActiveAccountID() (string, error)
}

type IssueUpdater interface {
	UpdateIssue(context.Context, string, linear.IssueUpdate) (linear.Issue, error)
}

type IssueArchiver interface {
	ArchiveIssue(context.Context, string) error
}

type DashboardDecorator interface {
	DecorateDashboard(linear.Dashboard) (linear.Dashboard, error)
}

type KeyFunc func() (string, error)

type Loader struct {
	store           *Store
	remote          DashboardLoader
	key             KeyFunc
	now             func() time.Time
	remoteMu        sync.Mutex
	filterMu        sync.Mutex
	filterRevisions map[string]uint64
}

func NewLoader(store *Store, remote DashboardLoader, key KeyFunc) *Loader {
	return &Loader{store: store, remote: remote, key: key, now: time.Now}
}

func APIKeyCacheKey(apiKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return "api-key:" + hex.EncodeToString(sum[:16])
}

func (l *Loader) LoadCachedDashboard(ctx context.Context) (linear.Dashboard, time.Time, error) {
	key, err := l.cacheKey()
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	dashboard, cachedAt, err := l.store.Load(ctx, key)
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	if decorator, ok := l.remote.(DashboardDecorator); ok {
		dashboard, err = decorator.DecorateDashboard(dashboard)
		if err != nil {
			return linear.Dashboard{}, time.Time{}, err
		}
	}
	return dashboard, cachedAt, nil
}

func (l *Loader) FetchDashboard(ctx context.Context) (linear.Dashboard, error) {
	if l == nil || l.remote == nil {
		return linear.Dashboard{}, errors.New("cached dashboard loader is not configured")
	}
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	dashboard, err := l.remote.FetchDashboard(ctx)
	if err != nil {
		return linear.Dashboard{}, err
	}
	l.saveBestEffort(ctx, dashboard)
	return dashboard, nil
}

func (l *Loader) SwitchAccount(ctx context.Context, accountID string) (linear.Dashboard, error) {
	if l == nil || l.remote == nil {
		return linear.Dashboard{}, errors.New("cached dashboard loader is not configured")
	}
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	switcher, ok := l.remote.(AccountSwitcher)
	if !ok {
		return linear.Dashboard{}, errors.New("account switching is not configured")
	}
	dashboard, err := switcher.SwitchAccount(ctx, accountID)
	if err != nil {
		return linear.Dashboard{}, err
	}
	l.saveBestEffort(ctx, dashboard)
	return dashboard, nil
}

// SwitchAccountCached selects an account and returns its cached snapshot
// immediately when one exists. The UI can then refresh that account in the
// background, keeping account switching useful while Linear is unavailable.
func (l *Loader) SwitchAccountCached(ctx context.Context, accountID string) (linear.Dashboard, time.Time, error) {
	if l == nil || l.remote == nil {
		return linear.Dashboard{}, time.Time{}, errors.New("cached dashboard loader is not configured")
	}
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	selector, ok := l.remote.(AccountSelector)
	if !ok {
		return linear.Dashboard{}, time.Time{}, errors.New("cached account switching is not configured")
	}
	previousAccountID, err := selector.ActiveAccountID()
	if err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}
	if err := selector.SelectAccount(accountID); err != nil {
		return linear.Dashboard{}, time.Time{}, err
	}

	key, err := l.cacheKey()
	if err == nil {
		dashboard, cachedAt, loadErr := l.store.Load(ctx, key)
		if loadErr == nil {
			if decorator, ok := l.remote.(DashboardDecorator); ok {
				dashboard, loadErr = decorator.DecorateDashboard(dashboard)
			}
			if loadErr == nil {
				return dashboard, cachedAt, nil
			}
		}
	}

	dashboard, err := l.remote.FetchDashboard(ctx)
	if err != nil {
		if previousAccountID != "" {
			_ = selector.SelectAccount(previousAccountID)
		}
		return linear.Dashboard{}, time.Time{}, err
	}
	l.saveBestEffort(ctx, dashboard)
	return dashboard, time.Time{}, nil
}

func (l *Loader) UpdateIssue(ctx context.Context, issueID string, update linear.IssueUpdate) (linear.Issue, error) {
	if l == nil || l.remote == nil {
		return linear.Issue{}, errors.New("cached dashboard loader is not configured")
	}
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	updater, ok := l.remote.(IssueUpdater)
	if !ok {
		return linear.Issue{}, errors.New("issue editing is not configured")
	}
	issue, err := updater.UpdateIssue(ctx, issueID, update)
	if err != nil {
		return linear.Issue{}, err
	}
	key, err := l.cacheKey()
	if err != nil {
		return issue, nil
	}
	dashboard, _, err := l.store.Load(ctx, key)
	if err != nil {
		return issue, nil
	}
	for index := range dashboard.Issues {
		if dashboard.Issues[index].ID == issue.ID {
			dashboard.Issues[index] = issue
			l.saveBestEffort(ctx, dashboard)
			break
		}
	}
	return issue, nil
}

func (l *Loader) ArchiveIssue(ctx context.Context, issueID string) error {
	if l == nil || l.remote == nil {
		return errors.New("cached dashboard loader is not configured")
	}
	l.remoteMu.Lock()
	defer l.remoteMu.Unlock()
	archiver, ok := l.remote.(IssueArchiver)
	if !ok {
		return errors.New("issue archiving is not configured")
	}
	if err := archiver.ArchiveIssue(ctx, issueID); err != nil {
		return err
	}
	key, err := l.cacheKey()
	if err != nil {
		return nil
	}
	dashboard, _, err := l.store.Load(ctx, key)
	if err != nil {
		return nil
	}
	issues := dashboard.Issues[:0]
	for _, issue := range dashboard.Issues {
		if issue.ID != issueID {
			issues = append(issues, issue)
		}
	}
	dashboard.Issues = issues
	l.saveBestEffort(ctx, dashboard)
	return nil
}

func (l *Loader) LoadIssueFilters(ctx context.Context, profileKey string) (issuefilter.State, error) {
	if l == nil || l.store == nil {
		return issuefilter.State{}, errors.New("cache loader is not configured")
	}
	return l.store.LoadIssueFilters(ctx, profileKey)
}

func (l *Loader) SaveIssueFilters(ctx context.Context, profileKey string, filters issuefilter.State, revision uint64) error {
	if l == nil || l.store == nil {
		return errors.New("cache loader is not configured")
	}
	l.filterMu.Lock()
	defer l.filterMu.Unlock()
	if l.filterRevisions == nil {
		l.filterRevisions = make(map[string]uint64)
	}
	if revision < l.filterRevisions[profileKey] {
		return nil
	}
	if err := l.store.SaveIssueFilters(ctx, profileKey, filters); err != nil {
		return err
	}
	l.filterRevisions[profileKey] = revision
	return nil
}

func (l *Loader) cacheKey() (string, error) {
	if l == nil || l.store == nil || l.key == nil {
		return "", errors.New("cache loader is not configured")
	}
	key, err := l.key()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("cache account key is empty")
	}
	return key, nil
}

func (l *Loader) saveBestEffort(ctx context.Context, dashboard linear.Dashboard) {
	key, err := l.cacheKey()
	if err != nil {
		return
	}
	now := time.Now
	if l.now != nil {
		now = l.now
	}
	_ = l.store.Save(ctx, key, dashboard, now())
}
