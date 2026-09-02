package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultEndpoint = "https://api.linear.app/graphql"

const dashboardQuery = `query TuinearDashboard($first: Int!) {
  viewer { id name displayName email }
  teams { nodes { id key name } }
  issues(first: $first, orderBy: updatedAt) {
    nodes {
      id identifier title description priority url createdAt updatedAt
      state { id name type color }
      assignee { id name displayName }
      team { id key name }
      project { id name }
      labels { nodes { id name color } }
    }
  }
}`

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type TokenSource interface {
	Token(context.Context) (string, error)
}

type Client struct {
	token       string
	tokenSource TokenSource
	endpoint    string
	http        HTTPDoer
}

type Option func(*Client)

func WithEndpoint(endpoint string) Option {
	return func(c *Client) { c.endpoint = endpoint }
}

func WithHTTPClient(client HTTPDoer) Option {
	return func(c *Client) { c.http = client }
}

func NewClient(token string, options ...Option) *Client {
	c := &Client{
		token:    strings.TrimSpace(token),
		endpoint: DefaultEndpoint,
		http:     &http.Client{Timeout: 20 * time.Second},
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func NewOAuthClient(source TokenSource, options ...Option) *Client {
	c := &Client{
		tokenSource: source,
		endpoint:    DefaultEndpoint,
		http:        &http.Client{Timeout: 20 * time.Second},
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type dashboardResponse struct {
	Data struct {
		Viewer Viewer `json:"viewer"`
		Teams  struct {
			Nodes []Team `json:"nodes"`
		} `json:"teams"`
		Issues struct {
			Nodes []Issue `json:"nodes"`
		} `json:"issues"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

func (c *Client) FetchDashboard(ctx context.Context) (Dashboard, error) {
	if c == nil {
		return Dashboard{}, errors.New("Linear client is nil")
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return Dashboard{}, errors.New("Linear API endpoint is empty")
	}
	authorization := c.token
	if c.tokenSource != nil {
		token, err := c.tokenSource.Token(ctx)
		if err != nil {
			return Dashboard{}, fmt.Errorf("get Linear OAuth token: %w", err)
		}
		if strings.TrimSpace(token) == "" {
			return Dashboard{}, errors.New("Linear OAuth token is empty")
		}
		authorization = "Bearer " + token
	} else if authorization == "" {
		return Dashboard{}, errors.New("LINEAR_API_KEY is empty")
	}

	payload, err := json.Marshal(graphQLRequest{
		Query:     dashboardQuery,
		Variables: map[string]any{"first": 100},
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("encode Linear request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return Dashboard{}, fmt.Errorf("create Linear request: %w", err)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tuinear/0.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return Dashboard{}, fmt.Errorf("contact Linear: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return Dashboard{}, fmt.Errorf("Linear returned HTTP %d: %s", resp.StatusCode, detail)
	}

	var decoded dashboardResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if err := decoder.Decode(&decoded); err != nil {
		return Dashboard{}, fmt.Errorf("decode Linear response: %w", err)
	}
	if len(decoded.Errors) > 0 {
		messages := make([]string, 0, len(decoded.Errors))
		for _, graphErr := range decoded.Errors {
			if strings.TrimSpace(graphErr.Message) != "" {
				messages = append(messages, graphErr.Message)
			}
		}
		return Dashboard{}, fmt.Errorf("Linear GraphQL error: %s", strings.Join(messages, "; "))
	}

	issues := decoded.Data.Issues.Nodes
	for index := range issues {
		issues[index].Normalize()
	}
	return Dashboard{
		Viewer: decoded.Data.Viewer,
		Teams:  decoded.Data.Teams.Nodes,
		Issues: issues,
	}, nil
}
