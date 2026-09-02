package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const DefaultEndpoint = "https://api.linear.app/graphql"

const dashboardQuery = `query TuinearDashboard($first: Int!) {
  viewer { id name displayName email }
  organization { id name urlKey }
  users(first: $first) { nodes { id name displayName } }
	issueLabels(first: $first) { nodes { id name color } }
  teams {
    nodes {
      id key name
      states { nodes { id name type color position } }
	  projects(first: $first) { nodes { id name } }
	  labels { nodes { id name color } }
    }
  }
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

const issueUpdateMutation = `mutation TuinearIssueUpdate($id: String!, $input: IssueUpdateInput!) {
  issueUpdate(id: $id, input: $input) {
    success
    issue {
      id identifier title description priority url createdAt updatedAt
      state { id name type color }
      assignee { id name displayName }
      team { id key name }
      project { id name }
      labels { nodes { id name color } }
    }
  }
}`

const issueArchiveMutation = `mutation TuinearIssueArchive($id: String!) {
  issueArchive(id: $id) { success }
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

type dashboardData struct {
	Viewer       Viewer       `json:"viewer"`
	Organization Organization `json:"organization"`
	Teams        struct {
		Nodes []struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Name   string `json:"name"`
			States struct {
				Nodes []WorkflowState `json:"nodes"`
			} `json:"states"`
			Projects struct {
				Nodes []Project `json:"nodes"`
			} `json:"projects"`
			Labels struct {
				Nodes []Label `json:"nodes"`
			} `json:"labels"`
		} `json:"nodes"`
	} `json:"teams"`
	Users struct {
		Nodes []User `json:"nodes"`
	} `json:"users"`
	IssueLabels struct {
		Nodes []Label `json:"nodes"`
	} `json:"issueLabels"`
	Issues struct {
		Nodes []Issue `json:"nodes"`
	} `json:"issues"`
}

func (c *Client) FetchDashboard(ctx context.Context) (Dashboard, error) {
	var decoded dashboardData
	if err := c.graphQL(ctx, dashboardQuery, map[string]any{"first": 100}, &decoded); err != nil {
		return Dashboard{}, err
	}
	issues := decoded.Issues.Nodes
	for index := range issues {
		issues[index].Normalize()
	}
	teams := make([]Team, 0, len(decoded.Teams.Nodes))
	teamStates := make([]TeamWorkflowStates, 0, len(decoded.Teams.Nodes))
	teamProjects := make([]TeamProjects, 0, len(decoded.Teams.Nodes))
	teamLabels := make([]TeamLabels, 0, len(decoded.Teams.Nodes))
	for _, team := range decoded.Teams.Nodes {
		sort.SliceStable(team.States.Nodes, func(i, j int) bool {
			return team.States.Nodes[i].Position < team.States.Nodes[j].Position
		})
		teams = append(teams, Team{ID: team.ID, Key: team.Key, Name: team.Name})
		teamStates = append(teamStates, TeamWorkflowStates{TeamID: team.ID, States: team.States.Nodes})
		sort.SliceStable(team.Projects.Nodes, func(i, j int) bool {
			return strings.ToLower(team.Projects.Nodes[i].Name) < strings.ToLower(team.Projects.Nodes[j].Name)
		})
		teamProjects = append(teamProjects, TeamProjects{TeamID: team.ID, Projects: team.Projects.Nodes})
		sort.SliceStable(team.Labels.Nodes, func(i, j int) bool {
			return strings.ToLower(team.Labels.Nodes[i].Name) < strings.ToLower(team.Labels.Nodes[j].Name)
		})
		teamLabels = append(teamLabels, TeamLabels{TeamID: team.ID, Labels: team.Labels.Nodes})
	}
	sort.SliceStable(decoded.IssueLabels.Nodes, func(i, j int) bool {
		return strings.ToLower(decoded.IssueLabels.Nodes[i].Name) < strings.ToLower(decoded.IssueLabels.Nodes[j].Name)
	})
	return Dashboard{
		Viewer:          decoded.Viewer,
		Organization:    decoded.Organization,
		Teams:           teams,
		TeamStates:      teamStates,
		TeamProjects:    teamProjects,
		WorkspaceLabels: decoded.IssueLabels.Nodes,
		TeamLabels:      teamLabels,
		Users:           decoded.Users.Nodes,
		Issues:          issues,
	}, nil
}

func (c *Client) UpdateIssue(ctx context.Context, issueID string, update IssueUpdate) (Issue, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return Issue{}, errors.New("Linear issue ID is empty")
	}
	if update.Title == nil && update.Description == nil && update.StateID == nil && update.Priority == nil && update.AssigneeID == nil && update.ProjectID == nil && update.LabelIDs == nil {
		return Issue{}, errors.New("Linear issue update has no fields")
	}
	if update.Title != nil {
		title := strings.TrimSpace(*update.Title)
		if title == "" {
			return Issue{}, errors.New("Linear issue title cannot be empty")
		}
		update.Title = &title
	}
	if update.StateID != nil {
		stateID := strings.TrimSpace(*update.StateID)
		if stateID == "" {
			return Issue{}, errors.New("Linear workflow state ID cannot be empty")
		}
		update.StateID = &stateID
	}
	if update.Priority != nil && (*update.Priority < 0 || *update.Priority > 4) {
		return Issue{}, errors.New("Linear issue priority must be between 0 and 4")
	}
	if update.AssigneeID != nil && *update.AssigneeID != nil {
		assigneeID := strings.TrimSpace(**update.AssigneeID)
		if assigneeID == "" {
			return Issue{}, errors.New("Linear assignee ID cannot be empty")
		}
		trimmed := &assigneeID
		update.AssigneeID = &trimmed
	}
	if update.ProjectID != nil && *update.ProjectID != nil {
		projectID := strings.TrimSpace(**update.ProjectID)
		if projectID == "" {
			return Issue{}, errors.New("Linear project ID cannot be empty")
		}
		trimmed := &projectID
		update.ProjectID = &trimmed
	}
	if update.LabelIDs != nil {
		seen := make(map[string]bool, len(*update.LabelIDs))
		labelIDs := make([]string, 0, len(*update.LabelIDs))
		for _, value := range *update.LabelIDs {
			labelID := strings.TrimSpace(value)
			if labelID == "" {
				return Issue{}, errors.New("Linear label ID cannot be empty")
			}
			if !seen[labelID] {
				seen[labelID] = true
				labelIDs = append(labelIDs, labelID)
			}
		}
		update.LabelIDs = &labelIDs
	}
	var decoded struct {
		IssueUpdate struct {
			Success bool  `json:"success"`
			Issue   Issue `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := c.graphQL(ctx, issueUpdateMutation, map[string]any{"id": issueID, "input": update}, &decoded); err != nil {
		return Issue{}, err
	}
	if !decoded.IssueUpdate.Success || decoded.IssueUpdate.Issue.ID == "" {
		return Issue{}, errors.New("Linear did not confirm the issue update")
	}
	decoded.IssueUpdate.Issue.Normalize()
	return decoded.IssueUpdate.Issue, nil
}

func (c *Client) ArchiveIssue(ctx context.Context, issueID string) error {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return errors.New("Linear issue ID is empty")
	}
	var decoded struct {
		IssueArchive struct {
			Success bool `json:"success"`
		} `json:"issueArchive"`
	}
	if err := c.graphQL(ctx, issueArchiveMutation, map[string]any{"id": issueID}, &decoded); err != nil {
		return err
	}
	if !decoded.IssueArchive.Success {
		return errors.New("Linear did not confirm the issue archive")
	}
	return nil
}

func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, target any) error {
	if c == nil {
		return errors.New("Linear client is nil")
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return errors.New("Linear API endpoint is empty")
	}
	authorization := c.token
	if c.tokenSource != nil {
		token, err := c.tokenSource.Token(ctx)
		if err != nil {
			return fmt.Errorf("get Linear OAuth token: %w", err)
		}
		if strings.TrimSpace(token) == "" {
			return errors.New("Linear OAuth token is empty")
		}
		authorization = "Bearer " + token
	} else if authorization == "" {
		return errors.New("LINEAR_API_KEY is empty")
	}
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("encode Linear request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create Linear request: %w", err)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tuinear/0.1")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("contact Linear: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("Linear returned HTTP %d: %s", resp.StatusCode, detail)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode Linear response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, graphErr := range envelope.Errors {
			if strings.TrimSpace(graphErr.Message) != "" {
				messages = append(messages, graphErr.Message)
			}
		}
		return fmt.Errorf("Linear GraphQL error: %s", strings.Join(messages, "; "))
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode Linear data: %w", err)
	}
	return nil
}
