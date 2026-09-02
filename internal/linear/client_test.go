package linear

import (
	"context"
	"encoding/json"
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
            "organization": {"id":"org-1","name":"Acme","urlKey":"acme"},
            "teams": {"nodes":[{"id":"t1","key":"ENG","name":"Engineering","states":{"nodes":[
              {"id":"s0","name":"Backlog","type":"backlog","color":"#888","position":20},
              {"id":"s1","name":"Todo","type":"unstarted","color":"#fff","position":10}
            ]}}]},
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
	if dashboard.Viewer.Label() != "J" || dashboard.Organization.Name != "Acme" || len(dashboard.Teams) != 1 || len(dashboard.Issues) != 1 {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
	if got := dashboard.Issues[0].Labels; len(got) != 1 || got[0].Name != "bug" {
		t.Fatalf("labels were not normalized: %#v", got)
	}
	if got := dashboard.StatesForTeam("t1"); len(got) != 2 || got[0].ID != "s1" {
		t.Fatalf("team workflow states = %#v", got)
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

func TestUpdateIssueSendsMutationAndReturnsCanonicalIssue(t *testing.T) {
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-test" {
			t.Errorf("Authorization = %q", got)
		}
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(request.Query, "issueUpdate") {
			t.Fatalf("request is not an issueUpdate mutation: %s", request.Query)
		}
		if request.Variables["id"] != "issue-1" {
			t.Fatalf("issue ID = %#v", request.Variables["id"])
		}
		input, ok := request.Variables["input"].(map[string]any)
		if !ok || input["title"] != "Renamed ticket" {
			t.Fatalf("mutation input = %#v", request.Variables["input"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"data":{"issueUpdate":{"success":true,"issue":{
              "id":"issue-1","identifier":"ENG-1","title":"Renamed ticket","description":"Details",
              "priority":2,"url":"https://linear.app/acme/issue/ENG-1","createdAt":"2026-01-01T00:00:00Z",
              "updatedAt":"2026-01-03T00:00:00Z","state":{"id":"state-1","name":"Todo","type":"unstarted"},
              "assignee":null,"team":{"id":"team-1","key":"ENG","name":"Engineering"},"project":null,
              "labels":{"nodes":[{"id":"label-1","name":"bug","color":"#f00"}]}
            }}}}`)),
		}, nil
	})
	client := NewOAuthClient(tokenSourceFunc(func(context.Context) (string, error) {
		return "oauth-test", nil
	}), WithHTTPClient(doer))
	title := "  Renamed ticket  "
	issue, err := client.UpdateIssue(t.Context(), " issue-1 ", IssueUpdate{Title: &title})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if issue.ID != "issue-1" || issue.Title != "Renamed ticket" || len(issue.Labels) != 1 {
		t.Fatalf("updated issue = %#v", issue)
	}
}

func TestUpdateIssueRejectsInvalidAndUnconfirmedUpdates(t *testing.T) {
	title := "New title"
	for _, test := range []struct {
		name    string
		issueID string
		update  IssueUpdate
	}{
		{name: "missing ID", update: IssueUpdate{Title: &title}},
		{name: "no fields", issueID: "issue-1"},
		{name: "blank title", issueID: "issue-1", update: IssueUpdate{Title: ptr("  ")}},
		{name: "blank state", issueID: "issue-1", update: IssueUpdate{StateID: ptr("  ")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewClient("token").UpdateIssue(t.Context(), test.issueID, test.update); err == nil {
				t.Fatal("invalid update was accepted")
			}
		})
	}

	doer := httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"data":{"issueUpdate":{"success":false,"issue":null}}}`,
		))}, nil
	})
	if _, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{Title: &title}); err == nil || !strings.Contains(err.Error(), "did not confirm") {
		t.Fatalf("unconfirmed update error = %v", err)
	}
}

func ptr(value string) *string { return &value }

func TestUpdateIssueAcceptsWorkflowStateOnly(t *testing.T) {
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		if input["stateId"] != "state-done" {
			t.Fatalf("state mutation input = %#v", input)
		}
		if _, present := input["title"]; present {
			t.Fatalf("status-only mutation unexpectedly sent title: %#v", input)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-1","title":"Ticket","state":{"id":"state-done","name":"Done","type":"completed"},"team":{"id":"team-1","key":"ENG","name":"Engineering"},"labels":{"nodes":[]}}}}}`,
		))}, nil
	})
	stateID := " state-done "
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{StateID: &stateID})
	if err != nil || issue.State.ID != "state-done" {
		t.Fatalf("status UpdateIssue = %#v, %v", issue, err)
	}
}
