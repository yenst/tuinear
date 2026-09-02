package linear

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type httpDoerFunc func(*http.Request) (*http.Response, error)

type tokenSourceFunc func(context.Context) (string, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (f tokenSourceFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

func TestFetchDashboard(t *testing.T) {
	t.Helper()
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "lin_api_test" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"first":100`) {
			t.Errorf("request does not contain page size: %s", body)
		}
		response := `{
          "data": {
            "viewer": {"id":"me","name":"Jamie","displayName":"J","email":"j@example.com"},
            "teams": {"nodes":[{"id":"t1","key":"ENG","name":"Engineering"}]},
            "issues": {"nodes":[{
              "id":"i1","identifier":"ENG-1","title":"First ticket","description":"Details",
              "priority":2,"url":"https://linear.app/i1",
              "createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z",
              "state":{"id":"s1","name":"Todo","type":"unstarted","color":"#fff"},
              "assignee":null,"team":{"id":"t1","key":"ENG","name":"Engineering"},
              "project":null,"labels":{"nodes":[{"id":"l1","name":"bug","color":"#f00"}]}
            }]}
          }
        }`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(response)),
		}, nil
	})

	client := NewClient("lin_api_test", WithEndpoint("https://linear.test/graphql"), WithHTTPClient(doer))
	dashboard, err := client.FetchDashboard(context.Background())
	if err != nil {
		t.Fatalf("FetchDashboard: %v", err)
	}
	if dashboard.Viewer.Label() != "J" || len(dashboard.Teams) != 1 || len(dashboard.Issues) != 1 {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
	if got := dashboard.Issues[0].Labels; len(got) != 1 || got[0].Name != "bug" {
		t.Fatalf("labels were not normalized: %#v", got)
	}
}

func TestFetchDashboardUsesOAuthBearerToken(t *testing.T) {
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-test" {
			t.Errorf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"viewer":{},"teams":{"nodes":[]},"issues":{"nodes":[]}}}`)),
		}, nil
	})
	client := NewOAuthClient(tokenSourceFunc(func(context.Context) (string, error) {
		return "oauth-test", nil
	}), WithHTTPClient(doer))
	if _, err := client.FetchDashboard(context.Background()); err != nil {
		t.Fatalf("FetchDashboard: %v", err)
	}
}

func TestFetchDashboardErrors(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		_, err := NewClient("").FetchDashboard(context.Background())
		if err == nil || !strings.Contains(err.Error(), "LINEAR_API_KEY") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("graphql", func(t *testing.T) {
		doer := httpDoerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"not authorized"}]}`)),
				Header:     make(http.Header),
			}, nil
		})
		_, err := NewClient("secret", WithHTTPClient(doer)).FetchDashboard(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not authorized") {
			t.Fatalf("error = %v", err)
		}
	})
}
