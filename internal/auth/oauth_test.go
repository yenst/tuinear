package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	token   Token
	present bool
	saves   int
}

func (s *memoryStore) Load() (Token, error) {
	if !s.present {
		return Token{}, ErrTokenNotFound
	}
	return s.token, nil
}

func (s *memoryStore) Save(token Token) error {
	s.token = token
	s.present = true
	s.saves++
	return nil
}

func (s *memoryStore) Delete() error {
	if !s.present {
		return ErrTokenNotFound
	}
	s.present = false
	return nil
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthorizationURLUsesReadOnlyPKCE(t *testing.T) {
	manager := NewManager("client-123", WithStore(&memoryStore{}))
	raw, err := manager.authorizationURL("state-value", "challenge-value")
	if err != nil {
		t.Fatalf("authorizationURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	want := map[string]string{
		"response_type":         "code",
		"client_id":             "client-123",
		"redirect_uri":          DefaultRedirectURI,
		"scope":                 "read",
		"state":                 "state-value",
		"code_challenge":        "challenge-value",
		"code_challenge_method": "S256",
	}
	for key, expected := range want {
		if got := u.Query().Get(key); got != expected {
			t.Errorf("%s = %q, want %q", key, got, expected)
		}
	}
	if strings.Contains(raw, "client_secret") {
		t.Fatal("authorization URL contains a client secret parameter")
	}
}

func TestValidateCallback(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		result := validateCallback(url.Values{"state": {"same"}, "code": {"auth-code"}}, "same")
		if result.err != nil || result.code != "auth-code" {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		result := validateCallback(url.Values{"state": {"different"}, "code": {"auth-code"}}, "expected")
		if result.err == nil || !strings.Contains(result.err.Error(), "state mismatch") {
			t.Fatalf("error = %v", result.err)
		}
	})

	t.Run("provider error", func(t *testing.T) {
		result := validateCallback(url.Values{"state": {"expected"}, "error": {"access_denied"}}, "expected")
		if result.err == nil || !strings.Contains(result.err.Error(), "access_denied") {
			t.Fatalf("error = %v", result.err)
		}
	})

	t.Run("provider error without state", func(t *testing.T) {
		result := validateCallback(url.Values{"error": {"access_denied"}}, "expected")
		if result.err == nil || !strings.Contains(result.err.Error(), "state mismatch") {
			t.Fatalf("error = %v", result.err)
		}
	})
}

func TestTokenRefreshesAndRotatesRefreshToken(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	store := &memoryStore{present: true, token: Token{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    now.Add(-time.Minute),
	}}
	doer := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("grant_type") != "refresh_token" || values.Get("refresh_token") != "old-refresh" {
			t.Errorf("unexpected refresh form: %s", body)
		}
		if values.Get("client_secret") != "" {
			t.Error("refresh request includes a client secret")
		}
		return response(http.StatusOK, `{"access_token":"new-access","token_type":"Bearer","expires_in":3600,"scope":"read","refresh_token":"new-refresh"}`), nil
	})
	manager := NewManager("client-123", WithStore(store), WithHTTPClient(doer), WithNow(func() time.Time { return now }))

	got, err := manager.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "new-access" {
		t.Fatalf("access token = %q", got)
	}
	if store.saves != 1 || store.token.RefreshToken != "new-refresh" {
		t.Fatalf("stored token = %#v, saves = %d", store.token, store.saves)
	}
	if !store.token.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry = %v", store.token.ExpiresAt)
	}
}

func TestTokenReturnsNotLoggedIn(t *testing.T) {
	manager := NewManager("client-123", WithStore(&memoryStore{}))
	_, err := manager.Token(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("error = %v", err)
	}
}

func TestLogoutRevokesBeforeDeleting(t *testing.T) {
	store := &memoryStore{present: true, token: Token{AccessToken: "access", RefreshToken: "refresh"}}
	revoked := false
	doer := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("token") != "refresh" || values.Get("token_type_hint") != "refresh_token" {
			t.Errorf("unexpected revoke form: %s", body)
		}
		revoked = true
		return response(http.StatusOK, `{}`), nil
	})
	manager := NewManager("client-123", WithStore(store), WithHTTPClient(doer))

	if err := manager.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !revoked || store.present {
		t.Fatalf("revoked = %v, credential still present = %v", revoked, store.present)
	}
}

func TestLoginCallbackTimeout(t *testing.T) {
	callback := make(chan callbackResult)
	serveErr := make(chan error)
	_, err := waitForCallback(context.Background(), 10*time.Millisecond, callback, serveErr)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v", err)
	}
}

func TestRedirectMustBeLoopback(t *testing.T) {
	_, err := validateRedirectURI("https://example.com/oauth/callback")
	if err == nil {
		t.Fatal("expected a non-loopback redirect to be rejected")
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
