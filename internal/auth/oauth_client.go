package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const profileQuery = `query TuinearOAuthProfile {
  viewer { id name displayName email }
  organization { id name urlKey }
}`

func (m *Manager) fetchProfile(ctx context.Context, accessToken string) (Profile, error) {
	payload, err := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: profileQuery})
	if err != nil {
		return Profile{}, fmt.Errorf("encode account identity request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.graphQLURL, strings.NewReader(string(payload)))
	if err != nil {
		return Profile{}, fmt.Errorf("create account identity request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tuinear/0.1")
	resp, err := m.http.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("contact Linear GraphQL API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Profile{}, fmt.Errorf("read account identity response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Profile{}, fmt.Errorf("Linear GraphQL API returned HTTP %d", resp.StatusCode)
	}
	var decoded struct {
		Data struct {
			Viewer struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				Email       string `json:"email"`
			} `json:"viewer"`
			Organization struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				URLKey string `json:"urlKey"`
			} `json:"organization"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Profile{}, fmt.Errorf("decode account identity response: %w", err)
	}
	if len(decoded.Errors) > 0 {
		return Profile{}, fmt.Errorf("Linear GraphQL error: %s", decoded.Errors[0].Message)
	}
	if decoded.Data.Viewer.ID == "" || decoded.Data.Organization.ID == "" {
		return Profile{}, errors.New("Linear did not return a workspace and user identity")
	}
	userName := decoded.Data.Viewer.DisplayName
	if userName == "" {
		userName = decoded.Data.Viewer.Name
	}
	return Profile{
		ID:            profileID(decoded.Data.Organization.ID, decoded.Data.Viewer.ID),
		UserID:        decoded.Data.Viewer.ID,
		UserName:      userName,
		UserEmail:     decoded.Data.Viewer.Email,
		WorkspaceID:   decoded.Data.Organization.ID,
		WorkspaceName: decoded.Data.Organization.Name,
		WorkspaceKey:  decoded.Data.Organization.URLKey,
	}, nil
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
