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
	var requests []graphQLRequest
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "lin_api_test" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
		var response string
		switch {
		case strings.Contains(request.Query, "TuinearDashboardWorkspace"):
			response = `{"data": {
            "viewer": {"id":"me","name":"Jamie","displayName":"J","email":"j@example.com"},
            "organization": {"id":"org-1","name":"Acme","urlKey":"acme"},
            "users": {"nodes":[
              {"id":"me","name":"Jamie","displayName":"J"},
              {"id":"u2","name":"Aisha Chen","displayName":"Aisha"}
            ]},
			"issueLabels": {"nodes":[{"id":"l2","name":"quality","color":"#00f"},{"id":"l1","name":"bug","color":"#f00"}]}
		  }}`
		case strings.Contains(request.Query, "TuinearDashboardTeams"):
			response = `{"data":{"teams":{"nodes":[{"id":"t1","key":"ENG","name":"Engineering"}]}}}`
		case strings.Contains(request.Query, "TuinearDashboardTeamDetails"):
			response = `{"data": {
            "team": {"id":"t1","key":"ENG","name":"Engineering","states":{"nodes":[
              {"id":"s0","name":"Backlog","type":"backlog","color":"#888","position":20},
              {"id":"s1","name":"Todo","type":"unstarted","color":"#fff","position":10}
            ]},"projects":{"nodes":[
              {"id":"p2","name":"Website"},{"id":"p1","name":"API"}
            ]},"labels":{"nodes":[{"id":"l3","name":"backend","color":"#0f0"}]}}
		  }}`
		case strings.Contains(request.Query, "TuinearDashboardIssues"):
			response = `{"data": {
            "issues": {"nodes":[{
              "id":"i1","identifier":"ENG-1","title":"First ticket","description":"Details",
              "priority":2,"url":"https://linear.app/i1","branchName":"eng-first-ticket",
              "createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z",
              "state":{"id":"s1","name":"Todo","type":"unstarted","color":"#fff"},
              "assignee":null,"team":{"id":"t1","key":"ENG","name":"Engineering"},
              "project":null,"labels":{"nodes":[{"id":"l1","name":"bug","color":"#f00"}]}
            }]}
		  }}`
		default:
			t.Fatalf("unexpected dashboard query: %s", request.Query)
		}
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
	if len(requests) != 4 {
		t.Fatalf("request count = %d, want 4", len(requests))
	}
	for index, request := range requests {
		if request.Variables["first"] != float64(100) {
			t.Errorf("request %d page size = %#v", index+1, request.Variables["first"])
		}
	}
	if query := requests[0].Query; !strings.Contains(query, "users(first: $first)") || !strings.Contains(query, "issueLabels(first: $first)") || strings.Contains(query, "teams {") || strings.Contains(query, "issues(first:") {
		t.Errorf("workspace request has unexpected structure: %s", query)
	}
	if query := requests[1].Query; !strings.Contains(query, "teams(first: $first)") || strings.Contains(query, "states(") || strings.Contains(query, "projects(") || strings.Contains(query, "labels(") {
		t.Errorf("team discovery request has nested connections: %s", query)
	}
	if query := requests[2].Query; !strings.Contains(query, "team(id: $teamID)") || !strings.Contains(query, "states(first: $nestedFirst)") || !strings.Contains(query, "projects(first: $first)") || !strings.Contains(query, "labels(first: $nestedFirst)") {
		t.Errorf("team metadata request is not explicitly bounded: %s", query)
	}
	if requests[2].Variables["teamID"] != "t1" || requests[2].Variables["nestedFirst"] != float64(50) {
		t.Errorf("team metadata variables = %#v", requests[2].Variables)
	}
	if query := requests[3].Query; !strings.Contains(query, "issues(first: $first") || !strings.Contains(query, "labels(first: $nestedFirst)") || strings.Contains(query, "teams(") || strings.Contains(query, "users(first:") {
		t.Errorf("issues request has unexpected structure: %s", query)
	}
	if requests[3].Variables["nestedFirst"] != float64(50) {
		t.Errorf("issues nested page size = %#v", requests[3].Variables["nestedFirst"])
	}
	if dashboard.Viewer.Label() != "J" || dashboard.Organization.Name != "Acme" || len(dashboard.Teams) != 1 || len(dashboard.Issues) != 1 {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
	if got := dashboard.Issues[0].Labels; len(got) != 1 || got[0].Name != "bug" {
		t.Fatalf("labels were not normalized: %#v", got)
	}
	if got := dashboard.Issues[0].BranchName; got != "eng-first-ticket" {
		t.Fatalf("branch name = %q, want %q", got, "eng-first-ticket")
	}
	if got := dashboard.StatesForTeam("t1"); len(got) != 2 || got[0].ID != "s1" {
		t.Fatalf("team workflow states = %#v", got)
	}
	if len(dashboard.Users) != 2 || dashboard.Users[1].Label() != "Aisha" {
		t.Fatalf("workspace users = %#v", dashboard.Users)
	}
	if got := dashboard.ProjectsForTeam("t1"); len(got) != 2 || got[0].ID != "p1" {
		t.Fatalf("team projects = %#v", dashboard.TeamProjects)
	}
	if got := dashboard.LabelsForTeam("t1"); len(got) != 3 || got[0].ID != "l1" || got[2].ID != "l3" {
		t.Fatalf("editable labels = %#v", got)
	}
}

func TestFetchDashboardUsesOAuthBearerToken(t *testing.T) {
	requestCount := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		requestCount++
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
	if requestCount != 3 {
		t.Fatalf("request count with no teams = %d, want 3", requestCount)
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

	t.Run("stops at failed request", func(t *testing.T) {
		phases := []string{"workspace", "teams", "team metadata t1", "issues"}
		for failedIndex, phase := range phases {
			t.Run(phase, func(t *testing.T) {
				requestCount := 0
				doer := httpDoerFunc(func(request *http.Request) (*http.Response, error) {
					requestCount++
					if requestCount == failedIndex+1 {
						return &http.Response{
							StatusCode: http.StatusBadRequest,
							Body:       io.NopCloser(strings.NewReader(`{"error":"Query too complex"}`)),
							Header:     make(http.Header),
						}, nil
					}
					var response string
					if requestCount == 2 {
						response = `{"data":{"teams":{"nodes":[{"id":"t1","key":"ENG","name":"Engineering"}]}}}`
					} else {
						response = `{"data":{}}`
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(response)),
						Header:     make(http.Header),
					}, nil
				})
				_, err := NewClient("secret", WithHTTPClient(doer)).FetchDashboard(context.Background())
				if err == nil || !strings.Contains(err.Error(), "dashboard "+phase) || !strings.Contains(err.Error(), "Query too complex") {
					t.Fatalf("error = %v", err)
				}
				if requestCount != failedIndex+1 {
					t.Fatalf("request count = %d, want %d", requestCount, failedIndex+1)
				}
			})
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
		if _, present := input["assigneeId"]; present {
			t.Fatalf("title-only mutation unexpectedly sent assignee: %#v", input)
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
	priorityBelowRange := -1
	priorityAboveRange := 5
	blankAssignee := ptr("  ")
	blankProject := ptr("  ")
	blankLabels := []string{"label-1", "  "}
	for _, test := range []struct {
		name    string
		issueID string
		update  IssueUpdate
	}{
		{name: "missing ID", update: IssueUpdate{Title: &title}},
		{name: "no fields", issueID: "issue-1"},
		{name: "blank title", issueID: "issue-1", update: IssueUpdate{Title: ptr("  ")}},
		{name: "blank state", issueID: "issue-1", update: IssueUpdate{StateID: ptr("  ")}},
		{name: "priority below range", issueID: "issue-1", update: IssueUpdate{Priority: &priorityBelowRange}},
		{name: "priority above range", issueID: "issue-1", update: IssueUpdate{Priority: &priorityAboveRange}},
		{name: "blank assignee", issueID: "issue-1", update: IssueUpdate{AssigneeID: &blankAssignee}},
		{name: "blank project", issueID: "issue-1", update: IssueUpdate{ProjectID: &blankProject}},
		{name: "blank label", issueID: "issue-1", update: IssueUpdate{LabelIDs: &blankLabels}},
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

func TestUpdateIssueAcceptsPriorityOnly(t *testing.T) {
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		if input["priority"] != float64(1) {
			t.Fatalf("priority mutation input = %#v", input)
		}
		if _, present := input["title"]; present {
			t.Fatalf("priority-only mutation unexpectedly sent title: %#v", input)
		}
		if _, present := input["stateId"]; present {
			t.Fatalf("priority-only mutation unexpectedly sent state: %#v", input)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-1","title":"Ticket","priority":1,"state":{"id":"state-todo","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"ENG","name":"Engineering"},"labels":{"nodes":[]}}}}}`,
		))}, nil
	})
	priority := 1
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{Priority: &priority})
	if err != nil || issue.Priority != 1 {
		t.Fatalf("priority UpdateIssue = %#v, %v", issue, err)
	}
}

func TestUpdateIssueAcceptsAssigneeAndUnassignOnly(t *testing.T) {
	requests := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		if _, present := input["title"]; present {
			t.Fatalf("assignee-only mutation unexpectedly sent title: %#v", input)
		}
		if _, present := input["stateId"]; present {
			t.Fatalf("assignee-only mutation unexpectedly sent state: %#v", input)
		}
		if _, present := input["priority"]; present {
			t.Fatalf("assignee-only mutation unexpectedly sent priority: %#v", input)
		}
		requests++
		if requests == 1 {
			if input["assigneeId"] != "user-2" {
				t.Fatalf("assign input = %#v", input)
			}
			return issueUpdateResponse(`{"id":"user-2","name":"Aisha Chen","displayName":"Aisha"}`), nil
		}
		value, present := input["assigneeId"]
		if !present || value != nil {
			t.Fatalf("unassign input = %#v", input)
		}
		return issueUpdateResponse("null"), nil
	})

	assigneeID := " user-2 "
	selected := &assigneeID
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{AssigneeID: &selected})
	if err != nil || issue.Assignee == nil || issue.Assignee.ID != "user-2" {
		t.Fatalf("assign UpdateIssue = %#v, %v", issue, err)
	}
	var unassigned *string
	issue, err = NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{AssigneeID: &unassigned})
	if err != nil || issue.Assignee != nil || requests != 2 {
		t.Fatalf("unassign UpdateIssue = %#v, %v, requests=%d", issue, err, requests)
	}
}

func issueUpdateResponse(assigneeJSON string) *http.Response {
	body := `{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-1","title":"Ticket","priority":2,"assignee":` + assigneeJSON + `,"state":{"id":"state-todo","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"ENG","name":"Engineering"},"labels":{"nodes":[]}}}}}`
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestUpdateIssueAcceptsProjectAndClearOnly(t *testing.T) {
	requests := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		for _, unrelated := range []string{"title", "stateId", "priority", "assigneeId"} {
			if _, present := input[unrelated]; present {
				t.Fatalf("project-only mutation unexpectedly sent %s: %#v", unrelated, input)
			}
		}
		requests++
		projectJSON := "null"
		if requests == 1 {
			if input["projectId"] != "project-2" {
				t.Fatalf("project input = %#v", input)
			}
			projectJSON = `{"id":"project-2","name":"Website"}`
		} else if value, present := input["projectId"]; !present || value != nil {
			t.Fatalf("clear project input = %#v", input)
		}
		body := `{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-1","title":"Ticket","priority":2,"project":` + projectJSON + `,"state":{"id":"state-todo","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"ENG","name":"Engineering"},"labels":{"nodes":[]}}}}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	projectID := " project-2 "
	selected := &projectID
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{ProjectID: &selected})
	if err != nil || issue.Project == nil || issue.Project.ID != "project-2" {
		t.Fatalf("project UpdateIssue = %#v, %v", issue, err)
	}
	var noProject *string
	issue, err = NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{ProjectID: &noProject})
	if err != nil || issue.Project != nil || requests != 2 {
		t.Fatalf("clear project UpdateIssue = %#v, %v, requests=%d", issue, err, requests)
	}
}

func TestUpdateIssueAcceptsLabelsAndClearOnly(t *testing.T) {
	requests := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		for _, unrelated := range []string{"title", "stateId", "priority", "assigneeId", "projectId"} {
			if _, present := input[unrelated]; present {
				t.Fatalf("label-only mutation unexpectedly sent %s: %#v", unrelated, input)
			}
		}
		requests++
		labels, ok := input["labelIds"].([]any)
		if !ok {
			t.Fatalf("label input = %#v", input)
		}
		labelsJSON := "[]"
		if requests == 1 {
			if len(labels) != 2 || labels[0] != "label-2" || labels[1] != "label-1" {
				t.Fatalf("label IDs = %#v", labels)
			}
			labelsJSON = `[{"id":"label-2","name":"quality","color":"#00f"},{"id":"label-1","name":"bug","color":"#f00"}]`
		} else if len(labels) != 0 {
			t.Fatalf("clear labels input = %#v", input)
		}
		body := `{"data":{"issueUpdate":{"success":true,"issue":{"id":"issue-1","identifier":"ENG-1","title":"Ticket","priority":2,"state":{"id":"state-todo","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"ENG","name":"Engineering"},"labels":{"nodes":` + labelsJSON + `}}}}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	labelIDs := []string{" label-2 ", "label-1", "label-2"}
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{LabelIDs: &labelIDs})
	if err != nil || len(issue.Labels) != 2 || issue.Labels[0].ID != "label-2" {
		t.Fatalf("labels UpdateIssue = %#v, %v", issue, err)
	}
	cleared := []string{}
	issue, err = NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{LabelIDs: &cleared})
	if err != nil || len(issue.Labels) != 0 || requests != 2 {
		t.Fatalf("clear labels UpdateIssue = %#v, %v, requests=%d", issue, err, requests)
	}
}

func TestUpdateIssueAcceptsDescriptionAndClearOnly(t *testing.T) {
	requests := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		input := request.Variables["input"].(map[string]any)
		for _, unrelated := range []string{"title", "stateId", "priority", "assigneeId", "projectId", "labelIds"} {
			if _, present := input[unrelated]; present {
				t.Fatalf("description-only mutation unexpectedly sent %s: %#v", unrelated, input)
			}
		}
		requests++
		want := "# Heading\n\nDetails"
		if requests == 2 {
			want = ""
		}
		if input["description"] != want {
			t.Fatalf("description input = %#v", input)
		}
		body, err := json.Marshal(map[string]any{"data": map[string]any{"issueUpdate": map[string]any{
			"success": true,
			"issue": map[string]any{
				"id": "issue-1", "identifier": "ENG-1", "title": "Ticket", "description": want,
				"state":  map[string]any{"id": "state-todo", "name": "Todo", "type": "unstarted"},
				"team":   map[string]any{"id": "team-1", "key": "ENG", "name": "Engineering"},
				"labels": map[string]any{"nodes": []any{}},
			},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})

	description := "# Heading\n\nDetails"
	issue, err := NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{Description: &description})
	if err != nil || issue.Description != description {
		t.Fatalf("description UpdateIssue = %#v, %v", issue, err)
	}
	cleared := ""
	issue, err = NewClient("token", WithHTTPClient(doer)).UpdateIssue(t.Context(), "issue-1", IssueUpdate{Description: &cleared})
	if err != nil || issue.Description != "" || requests != 2 {
		t.Fatalf("clear description UpdateIssue = %#v, %v, requests=%d", issue, err, requests)
	}
}

func TestArchiveIssueRequiresConfirmationFromLinear(t *testing.T) {
	requests := 0
	doer := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(request.Query, "issueArchive") || request.Variables["id"] != "issue-1" {
			t.Fatalf("archive request = %#v query=%q", request.Variables, request.Query)
		}
		requests++
		success := requests == 1
		body, _ := json.Marshal(map[string]any{"data": map[string]any{"issueArchive": map[string]any{"success": success}}})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	client := NewClient("token", WithHTTPClient(doer))
	if err := client.ArchiveIssue(t.Context(), " issue-1 "); err != nil {
		t.Fatalf("ArchiveIssue: %v", err)
	}
	if err := client.ArchiveIssue(t.Context(), "issue-1"); err == nil || !strings.Contains(err.Error(), "did not confirm") {
		t.Fatalf("unconfirmed archive error = %v", err)
	}
	if err := client.ArchiveIssue(t.Context(), "  "); err == nil {
		t.Fatal("blank archive issue ID was accepted")
	}
}
