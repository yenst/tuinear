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
	sessions map[string]Session
	active   string
	saves    int
}

func newMemoryStore(sessions ...Session) *memoryStore {
	store := &memoryStore{sessions: make(map[string]Session)}
	for _, session := range sessions {
		store.sessions[session.Profile.ID] = session
		if store.active == "" {
			store.active = session.Profile.ID
		}
	}
	return store
}

func (s *memoryStore) List() ([]Profile, error) {
	profiles := make([]Profile, 0, len(s.sessions))
	for _, session := range s.sessions {
		profiles = append(profiles, session.Profile)
	}
	return profiles, nil
}

func (s *memoryStore) Active() (string, error) {
	if s.active == "" {
		return "", ErrProfileNotFound
	}
	return s.active, nil
}

func (s *memoryStore) Load(profileID string) (Session, error) {
	if profileID == "" {
		profileID = s.active
	}
	session, ok := s.sessions[profileID]
	if !ok {
		return Session{}, ErrProfileNotFound
	}
	return session, nil
}

func (s *memoryStore) Save(session Session, activate bool) error {
	if s.sessions == nil {
		s.sessions = make(map[string]Session)
	}
	s.sessions[session.Profile.ID] = session
	if activate || s.active == "" {
		s.active = session.Profile.ID
	}
	s.saves++
	return nil
}

func (s *memoryStore) SetActive(profileID string) error {
	if _, ok := s.sessions[profileID]; !ok {
		return ErrProfileNotFound
	}
	s.active = profileID
	return nil
}

func (s *memoryStore) Delete(profileID string) error {
	if profileID == "" {
		profileID = s.active
	}
	if _, ok := s.sessions[profileID]; !ok {
		return ErrProfileNotFound
	}
	delete(s.sessions, profileID)
	if s.active == profileID {
		s.active = ""
		for id := range s.sessions {
			s.active = id
			break
		}
	}
	return nil
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthorizationURLUsesReadWritePKCE(t *testing.T) {
	manager := NewManager("client-123", WithStore(newMemoryStore()))
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
		"scope":                 "read,write",
		"prompt":                "consent",
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

func TestHasScopeUsesActiveProfile(t *testing.T) {
	store := newMemoryStore(
		Session{Profile: Profile{ID: "work"}, Token: Token{Scope: "read"}},
		Session{Profile: Profile{ID: "personal"}, Token: Token{Scope: "read,write"}},
	)
	store.active = "personal"
	manager := NewManager("client-123", WithStore(store))
	granted, err := manager.HasScope("write")
	if err != nil || !granted {
		t.Fatalf("write scope = %v, %v", granted, err)
	}
	store.active = "work"
	granted, err = manager.HasScope("write")
	if err != nil || granted {
		t.Fatalf("read-only write scope = %v, %v", granted, err)
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
	profile := Profile{ID: "work", WorkspaceName: "Work", UserName: "Jamie"}
	store := newMemoryStore(Session{Profile: profile, Token: Token{
		AccessToken: "expired-access", RefreshToken: "old-refresh", ExpiresAt: now.Add(-time.Minute),
	}})
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
	stored := store.sessions[profile.ID].Token
	if store.saves != 1 || stored.RefreshToken != "new-refresh" {
		t.Fatalf("stored token = %#v, saves = %d", stored, store.saves)
	}
	if !stored.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry = %v", stored.ExpiresAt)
	}
}

func TestTokenReturnsNotLoggedIn(t *testing.T) {
	manager := NewManager("client-123", WithStore(newMemoryStore()))
	_, err := manager.Token(context.Background())
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("error = %v", err)
	}
}

func TestLogoutRevokesBeforeDeleting(t *testing.T) {
	profile := Profile{ID: "personal", WorkspaceName: "Personal", UserName: "Jamie"}
	store := newMemoryStore(Session{Profile: profile, Token: Token{AccessToken: "access", RefreshToken: "refresh"}})
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
	if _, present := store.sessions[profile.ID]; !revoked || present {
		t.Fatalf("revoked = %v, credential still present = %v", revoked, present)
	}
}

func TestLogoutRemovesProfileWithMissingToken(t *testing.T) {
	store := &missingTokenStore{memoryStore: *newMemoryStore(Session{Profile: Profile{ID: "orphan"}})}
	manager := NewManager("client-123", WithStore(store))

	if err := manager.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, present := store.sessions["orphan"]; present {
		t.Fatal("orphaned profile was not removed")
	}
}

type missingTokenStore struct {
	memoryStore
}

func (s *missingTokenStore) Load(profileID string) (Session, error) {
	if profileID == "" {
		profileID = s.active
	}
	if _, ok := s.sessions[profileID]; !ok {
		return Session{}, ErrProfileNotFound
	}
	return Session{}, ErrTokenNotFound
}

func TestSelectProfileChangesTokenSource(t *testing.T) {
	work := Session{Profile: Profile{ID: "work", WorkspaceName: "Work"}, Token: Token{AccessToken: "work-token"}}
	personal := Session{Profile: Profile{ID: "personal", WorkspaceName: "Personal"}, Token: Token{AccessToken: "personal-token"}}
	store := newMemoryStore(work, personal)
	manager := NewManager("client-123", WithStore(store))

	if err := manager.SelectProfile("personal"); err != nil {
		t.Fatalf("SelectProfile: %v", err)
	}
	got, err := manager.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "personal-token" || store.active != "personal" {
		t.Fatalf("token = %q, active = %q", got, store.active)
	}
}

func TestRefreshDoesNotOverwriteAnotherProfile(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	work := Session{Profile: Profile{ID: "work"}, Token: Token{AccessToken: "old-work", RefreshToken: "refresh-work", ExpiresAt: now.Add(-time.Minute)}}
	personal := Session{Profile: Profile{ID: "personal"}, Token: Token{AccessToken: "personal-token", ExpiresAt: now.Add(time.Hour)}}
	store := newMemoryStore(work, personal)
	store.active = "personal"
	doer := httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"access_token":"new-work","expires_in":3600,"refresh_token":"new-refresh-work"}`), nil
	})
	manager := NewManager("client-123", WithStore(store), WithProfileID("work"), WithHTTPClient(doer), WithNow(func() time.Time { return now }))

	if _, err := manager.Token(context.Background()); err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got := store.sessions["work"].Token.AccessToken; got != "new-work" {
		t.Fatalf("work token = %q", got)
	}
	if got := store.sessions["personal"].Token.AccessToken; got != "personal-token" {
		t.Fatalf("personal token = %q", got)
	}
	if store.active != "personal" {
		t.Fatalf("active profile changed to %q", store.active)
	}
}

func TestFetchProfileUsesWorkspaceAndUserIdentity(t *testing.T) {
	doer := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer access" {
			t.Errorf("Authorization = %q", got)
		}
		return response(http.StatusOK, `{"data":{"viewer":{"id":"user-1","name":"Jamie Doe","displayName":"Jamie","email":"jamie@example.com"},"organization":{"id":"org-1","name":"Personal","urlKey":"personal"}}}`), nil
	})
	manager := NewManager("client-123", WithStore(newMemoryStore()), WithHTTPClient(doer), WithGraphQLEndpoint("https://linear.test/graphql"))

	profile, err := manager.fetchProfile(context.Background(), "access")
	if err != nil {
		t.Fatalf("fetchProfile: %v", err)
	}
	if profile.ID == "" || profile.WorkspaceName != "Personal" || profile.UserEmail != "jamie@example.com" {
		t.Fatalf("profile = %#v", profile)
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

func TestRedirectRejectsPortZero(t *testing.T) {
	_, err := validateRedirectURI("http://127.0.0.1:0/oauth/callback")
	if err == nil {
		t.Fatal("expected port zero to be rejected")
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
