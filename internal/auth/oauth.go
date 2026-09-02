package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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

type Store interface {
	Load() (Token, error)
	Save(Token) error
	Delete() error
}

type Manager struct {
	clientID        string
	redirectURI     string
	authorizeURL    string
	tokenURL        string
	revokeURL       string
	store           Store
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
	token, err := m.store.Load()
	if errors.Is(err, ErrTokenNotFound) {
		return "", ErrNotLoggedIn
	}
	if err != nil {
		return "", fmt.Errorf("load Linear credentials: %w", err)
	}
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
	if err := m.store.Save(refreshed); err != nil {
		return "", fmt.Errorf("save refreshed Linear credentials: %w", err)
	}
	return refreshed.AccessToken, nil
}

// Login starts a loopback callback, emits the authorization URL, and completes PKCE.
func (m *Manager) Login(ctx context.Context, showURL func(string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validate(); err != nil {
		return err
	}
	redirect, err := validateRedirectURI(m.redirectURI)
	if err != nil {
		return err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return fmt.Errorf("generate OAuth state: %w", err)
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		return fmt.Errorf("generate PKCE verifier: %w", err)
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return fmt.Errorf("start OAuth callback on %s: %w", redirect.Host, err)
	}
	defer listener.Close()

	callback := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		result := validateCallback(r.URL.Query(), state)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if result.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, callbackPage("Tuinear could not sign in", "Return to your terminal and try again."))
			if !callbackStateMatches(r.URL.Query(), state) {
				return
			}
		} else {
			_, _ = io.WriteString(w, callbackPage("Tuinear is connected", "You can close this tab and return to your terminal."))
		}
		select {
		case callback <- result:
		default:
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	authorizeURL, err := m.authorizationURL(state, challenge)
	if err != nil {
		return err
	}
	if showURL != nil {
		showURL(authorizeURL)
	}

	code, err := waitForCallback(ctx, m.callbackTimeout, callback, serveErr)
	if err != nil {
		return err
	}

	token, err := m.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {m.redirectURI},
		"client_id":     {m.clientID},
		"code_verifier": {verifier},
	})
	if err != nil {
		return fmt.Errorf("exchange Linear authorization code: %w", err)
	}
	if err := m.store.Save(token); err != nil {
		return fmt.Errorf("save Linear credentials: %w", err)
	}
	return nil
}

func (m *Manager) Logout(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validate(); err != nil {
		return err
	}
	token, err := m.store.Load()
	if errors.Is(err, ErrTokenNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load Linear credentials: %w", err)
	}
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
	if err := m.store.Delete(); err != nil && !errors.Is(err, ErrTokenNotFound) {
		return fmt.Errorf("delete Linear credentials: %w", err)
	}
	return nil
}

func (m *Manager) authorizationURL(state, challenge string) (string, error) {
	u, err := url.Parse(m.authorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse Linear authorization URL: %w", err)
	}
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", m.clientID)
	query.Set("redirect_uri", m.redirectURI)
	query.Set("scope", "read")
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (m *Manager) requestToken(ctx context.Context, values url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Tuinear/0.1")
	resp, err := m.http.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("contact Linear OAuth: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Token{}, fmt.Errorf("read Linear OAuth response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Token{}, fmt.Errorf("Linear OAuth returned HTTP %d: %s", resp.StatusCode, oauthError(body))
	}
	var decoded tokenResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Token{}, fmt.Errorf("decode Linear OAuth response: %w", err)
	}
	if decoded.AccessToken == "" {
		return Token{}, errors.New("Linear OAuth response did not include an access token")
	}
	tokenType := decoded.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return Token{
		AccessToken:  decoded.AccessToken,
		TokenType:    tokenType,
		RefreshToken: decoded.RefreshToken,
		Scope:        decoded.Scope.String(),
		ExpiresAt:    m.now().Add(time.Duration(decoded.ExpiresIn) * time.Second),
	}, nil
}

func (m *Manager) revoke(ctx context.Context, token, hint string) error {
	values := url.Values{"token": {token}, "token_type_hint": {hint}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.revokeURL, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("create Linear revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Tuinear/0.1")
	resp, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("revoke Linear session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("Linear revoke returned HTTP %d: %s", resp.StatusCode, oauthError(body))
	}
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

func validateRedirectURI(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth redirect URI: %w", err)
	}
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" {
		return nil, errors.New("OAuth redirect URI must use http://127.0.0.1:<port>/<path>")
	}
	if u.Path == "" || u.Path == "/" {
		return nil, errors.New("OAuth redirect URI must include a callback path")
	}
	return u, nil
}

type callbackResult struct {
	code string
	err  error
}

func waitForCallback(ctx context.Context, timeout time.Duration, callback <-chan callbackResult, serveErr <-chan error) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case result := <-callback:
		if result.err != nil {
			return "", result.err
		}
		return result.code, nil
	case err := <-serveErr:
		return "", fmt.Errorf("serve OAuth callback: %w", err)
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			return "", errors.New("OAuth login timed out")
		}
		return "", waitCtx.Err()
	}
}

func validateCallback(values url.Values, expectedState string) callbackResult {
	if !callbackStateMatches(values, expectedState) {
		return callbackResult{err: errors.New("OAuth state mismatch")}
	}
	if oauthErr := values.Get("error"); oauthErr != "" {
		description := values.Get("error_description")
		if description == "" {
			description = oauthErr
		}
		return callbackResult{err: fmt.Errorf("Linear authorization failed: %s", description)}
	}
	code := values.Get("code")
	if code == "" {
		return callbackResult{err: errors.New("OAuth callback did not include an authorization code")}
	}
	return callbackResult{code: code}
}

func callbackStateMatches(values url.Values, expectedState string) bool {
	state := values.Get("state")
	return state != "" && subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) == 1
}

func randomURLToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func callbackPage(title, message string) string {
	return "<!doctype html><meta charset=utf-8><title>" + title + "</title>" +
		"<style>body{font:16px system-ui;max-width:34rem;margin:15vh auto;padding:2rem;color:#e8e8ea;background:#0f0f11}h1{color:#7c83ff}</style>" +
		"<h1>" + title + "</h1><p>" + message + "</p>"
}

func oauthError(body []byte) string {
	var response struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &response) == nil {
		if response.ErrorDescription != "" {
			return response.ErrorDescription
		}
		if response.Error != "" {
			return response.Error
		}
	}
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return "request failed"
	}
	return detail
}

type scopeValue []string

func (s *scopeValue) UnmarshalJSON(data []byte) error {
	var single string
	if json.Unmarshal(data, &single) == nil {
		if single == "" {
			*s = nil
		} else {
			*s = strings.Fields(single)
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

func (s scopeValue) String() string {
	return strings.Join(s, " ")
}

type tokenResponse struct {
	AccessToken  string     `json:"access_token"`
	TokenType    string     `json:"token_type"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int64      `json:"expires_in"`
	Scope        scopeValue `json:"scope"`
}
