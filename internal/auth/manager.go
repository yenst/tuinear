package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAuthorizeURL = "https://linear.app/oauth/authorize"
	DefaultTokenURL     = "https://api.linear.app/oauth/token"
	DefaultRevokeURL    = "https://api.linear.app/oauth/revoke"
	DefaultGraphQLURL   = "https://api.linear.app/graphql"
	DefaultRedirectURI  = "http://127.0.0.1:14565/oauth/callback"
)

var ErrNotLoggedIn = errors.New("not logged in to Linear")

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Manager struct {
	clientID        string
	redirectURI     string
	authorizeURL    string
	tokenURL        string
	revokeURL       string
	graphQLURL      string
	store           Store
	profileID       string
	http            HTTPDoer
	now             func() time.Time
	callbackTimeout time.Duration
	mu              sync.Mutex
}

type Option func(*Manager)

func WithRedirectURI(uri string) Option {
	return func(m *Manager) { m.redirectURI = uri }
}

func WithEndpoints(authorizeURL, tokenURL, revokeURL string) Option {
	return func(m *Manager) {
		m.authorizeURL = authorizeURL
		m.tokenURL = tokenURL
		m.revokeURL = revokeURL
	}
}

func WithStore(store Store) Option {
	return func(m *Manager) { m.store = store }
}

func WithProfileID(profileID string) Option {
	return func(m *Manager) { m.profileID = strings.TrimSpace(profileID) }
}

func WithGraphQLEndpoint(endpoint string) Option {
	return func(m *Manager) { m.graphQLURL = endpoint }
}

func WithHTTPClient(client HTTPDoer) Option {
	return func(m *Manager) { m.http = client }
}

func WithNow(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

func WithCallbackTimeout(timeout time.Duration) Option {
	return func(m *Manager) { m.callbackTimeout = timeout }
}

func NewManager(clientID string, options ...Option) *Manager {
	clientID = strings.TrimSpace(clientID)
	m := &Manager{
		clientID:        clientID,
		redirectURI:     DefaultRedirectURI,
		authorizeURL:    DefaultAuthorizeURL,
		tokenURL:        DefaultTokenURL,
		revokeURL:       DefaultRevokeURL,
		graphQLURL:      DefaultGraphQLURL,
		store:           NewKeyringStore(clientID),
		http:            &http.Client{Timeout: 20 * time.Second},
		now:             time.Now,
		callbackTimeout: 3 * time.Minute,
	}
	for _, option := range options {
		option(m)
	}
	return m
}

// Token implements linear.TokenSource and refreshes access tokens before expiry.
func (m *Manager) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validate(); err != nil {
		return "", err
	}
	session, err := m.store.Load(m.profileID)
	if errors.Is(err, ErrTokenNotFound) || errors.Is(err, ErrProfileNotFound) {
		return "", ErrNotLoggedIn
	}
	if err != nil {
		return "", fmt.Errorf("load Linear credentials: %w", err)
	}
	token := session.Token
	if token.AccessToken == "" {
		return "", ErrNotLoggedIn
	}
	if token.ExpiresAt.IsZero() || token.ExpiresAt.After(m.now().Add(time.Minute)) {
		return token.AccessToken, nil
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("Linear session expired: %w", ErrNotLoggedIn)
	}

	refreshed, err := m.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"client_id":     {m.clientID},
	})
	if err != nil {
		return "", fmt.Errorf("refresh Linear session: %w", err)
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	session.Token = refreshed
	if err := m.store.Save(session, false); err != nil {
		return "", fmt.Errorf("save refreshed Linear credentials: %w", err)
	}
	return refreshed.AccessToken, nil
}

// HasScope reports whether the active profile's token grants an OAuth scope.
// Linear can return scopes separated by spaces, commas, or both.
func (m *Manager) HasScope(scope string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return false, err
	}
	session, err := m.store.Load(m.profileID)
	if err != nil {
		return false, err
	}
	want := strings.ToLower(strings.TrimSpace(scope))
	for _, granted := range strings.FieldsFunc(session.Token.Scope, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		if strings.EqualFold(granted, want) {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) Logout(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validate(); err != nil {
		return err
	}
	session, err := m.store.Load(m.profileID)
	if errors.Is(err, ErrProfileNotFound) {
		return nil
	}
	if errors.Is(err, ErrTokenNotFound) {
		profileID := m.profileID
		if profileID == "" {
			profileID, _ = m.store.Active()
		}
		if profileID != "" {
			return m.store.Delete(profileID)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load Linear credentials: %w", err)
	}
	token := session.Token
	revokeToken := token.RefreshToken
	hint := "refresh_token"
	if revokeToken == "" {
		revokeToken = token.AccessToken
		hint = "access_token"
	}
	if revokeToken != "" {
		if err := m.revoke(ctx, revokeToken, hint); err != nil {
			return err
		}
	}
	if err := m.store.Delete(session.Profile.ID); err != nil && !errors.Is(err, ErrTokenNotFound) && !errors.Is(err, ErrProfileNotFound) {
		return fmt.Errorf("delete Linear credentials: %w", err)
	}
	return nil
}

func (m *Manager) Profiles() ([]Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return nil, err
	}
	return m.store.List()
}

func (m *Manager) ActiveProfile() (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return Profile{}, err
	}
	session, err := m.store.Load(m.profileID)
	if err != nil {
		return Profile{}, err
	}
	return session.Profile, nil
}

func (m *Manager) ActiveProfileID() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return "", err
	}
	return m.store.Active()
}

func (m *Manager) SelectProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}
	if err := m.store.SetActive(profileID); err != nil {
		return err
	}
	m.profileID = profileID
	return nil
}

func (m *Manager) validate() error {
	if m == nil {
		return errors.New("OAuth manager is nil")
	}
	if m.clientID == "" {
		return errors.New("TUINEAR_OAUTH_CLIENT_ID is not set")
	}
	if m.store == nil || m.http == nil || m.now == nil {
		return errors.New("OAuth manager is not fully configured")
	}
	return nil
}
