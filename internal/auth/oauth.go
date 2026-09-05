package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Login starts a loopback callback, emits the authorization URL, and completes PKCE.
func (m *Manager) Login(ctx context.Context, showURL func(string)) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validate(); err != nil {
		return Profile{}, err
	}
	redirect, err := validateRedirectURI(m.redirectURI)
	if err != nil {
		return Profile{}, err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return Profile{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		return Profile{}, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return Profile{}, fmt.Errorf("start OAuth callback on %s: %w", redirect.Host, err)
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
		return Profile{}, err
	}
	if showURL != nil {
		showURL(authorizeURL)
	}

	code, err := waitForCallback(ctx, m.callbackTimeout, callback, serveErr)
	if err != nil {
		return Profile{}, err
	}
	token, err := m.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {m.redirectURI},
		"client_id":     {m.clientID},
		"code_verifier": {verifier},
	})
	if err != nil {
		return Profile{}, fmt.Errorf("exchange Linear authorization code: %w", err)
	}
	profile, err := m.fetchProfile(ctx, token.AccessToken)
	if err != nil {
		return Profile{}, fmt.Errorf("identify Linear account: %w", err)
	}
	if err := m.store.Save(Session{Profile: profile, Token: token}, true); err != nil {
		return Profile{}, fmt.Errorf("save Linear credentials: %w", err)
	}
	m.profileID = profile.ID
	return profile, nil
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
	query.Set("scope", "read,write")
	query.Set("prompt", "consent")
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func validateRedirectURI(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth redirect URI: %w", err)
	}
	port, portErr := strconv.Atoi(u.Port())
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || portErr != nil || port < 1 || port > 65535 {
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
