package linear

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DemoClient struct{}

func (DemoClient) FetchDashboard(context.Context) (Dashboard, error) {
	return demoDashboard("demo-work"), nil
}

func (DemoClient) SwitchAccount(_ context.Context, accountID string) (Dashboard, error) {
	if accountID != "demo-work" && accountID != "demo-personal" {
		return Dashboard{}, fmt.Errorf("unknown demo account %q", accountID)
	}
	return demoDashboard(accountID), nil
}

func (DemoClient) UpdateIssue(_ context.Context, issueID string, update IssueUpdate) (Issue, error) {
	dashboard := demoDashboard("demo-work")
	for _, issue := range dashboard.Issues {
		if issue.ID != issueID {
			continue
		}
		if update.Title != nil {
			title := strings.TrimSpace(*update.Title)
			if title == "" {
				return Issue{}, fmt.Errorf("issue title cannot be empty")
			}
			issue.Title = title
			issue.UpdatedAt = time.Now()
		}
		if update.StateID != nil {
			found := false
			for _, state := range dashboard.StatesForTeam(issue.Team.ID) {
				if state.ID == strings.TrimSpace(*update.StateID) {
					issue.State = state
					issue.UpdatedAt = time.Now()
					found = true
					break
				}
			}
			if !found {
				return Issue{}, fmt.Errorf("unknown demo workflow state %q", *update.StateID)
			}
		}
		return issue, nil
	}
	return Issue{}, fmt.Errorf("unknown demo issue %q", issueID)
}

func demoDashboard(activeAccountID string) Dashboard {
	now := time.Now()
	platform := Team{ID: "team-platform", Key: "PLAT", Name: "Platform"}
	product := Team{ID: "team-product", Key: "PROD", Name: "Product"}
	platformBacklog := WorkflowState{ID: "platform-backlog", Name: "Backlog", Type: "backlog", Color: "#858585"}
	platformTodo := WorkflowState{ID: "platform-todo", Name: "Todo", Type: "unstarted", Color: "#5E6AD2"}
	platformProgress := WorkflowState{ID: "platform-progress", Name: "In Progress", Type: "started", Color: "#F2C94C"}
	platformDone := WorkflowState{ID: "platform-done", Name: "Done", Type: "completed", Color: "#4CB782"}
	productBacklog := WorkflowState{ID: "product-backlog", Name: "Backlog", Type: "backlog", Color: "#858585"}
	productTodo := WorkflowState{ID: "product-todo", Name: "Todo", Type: "unstarted", Color: "#5E6AD2"}
	productProgress := WorkflowState{ID: "product-progress", Name: "In Progress", Type: "started", Color: "#F2C94C"}
	productDone := WorkflowState{ID: "product-done", Name: "Done", Type: "completed", Color: "#4CB782"}
	aisha := User{ID: "user-aisha", Name: "Aisha Chen", DisplayName: "Aisha"}
	marcus := User{ID: "user-marcus", Name: "Marcus Bell", DisplayName: "Marcus"}
	launch := Project{ID: "project-launch", Name: "Public launch"}
	quality := Label{ID: "label-quality", Name: "quality", Color: "#5E6AD2"}

	accounts := []Account{
		{ID: "demo-work", WorkspaceName: "Acme", WorkspaceKey: "acme", UserName: "Jamie", UserEmail: "jamie@acme.test"},
		{ID: "demo-personal", WorkspaceName: "Personal", WorkspaceKey: "personal", UserName: "Jamie", UserEmail: "jamie@example.com"},
	}
	viewer := Viewer{ID: "viewer-work", Name: "Jamie", DisplayName: "Jamie", Email: "jamie@acme.test"}
	organization := Organization{ID: "demo-work", Name: "Acme", URLKey: "acme"}
	if activeAccountID == "demo-personal" {
		viewer = Viewer{ID: "viewer-personal", Name: "Jamie", DisplayName: "Jamie", Email: "jamie@example.com"}
		organization = Organization{ID: "demo-personal", Name: "Personal", URLKey: "personal"}
	}

	return Dashboard{
		Viewer:          viewer,
		Organization:    organization,
		Accounts:        accounts,
		ActiveAccountID: activeAccountID,
		Teams:           []Team{platform, product},
		TeamStates: []TeamWorkflowStates{
			{TeamID: platform.ID, States: []WorkflowState{platformBacklog, platformTodo, platformProgress, platformDone}},
			{TeamID: product.ID, States: []WorkflowState{productBacklog, productTodo, productProgress, productDone}},
		},
		Issues: []Issue{
			{
				ID: "1", Identifier: "PLAT-42", Title: "Persist the issue cache between sessions",
				Description: "Warm startup should render cached tickets immediately, then reconcile with Linear in the background.",
				Priority:    2, UpdatedAt: now.Add(-8 * time.Minute), CreatedAt: now.Add(-72 * time.Hour),
				State: platformProgress, Team: platform,
				Assignee: &aisha, Project: &launch, Labels: []Label{quality}, URL: "https://linear.app/demo/issue/PLAT-42",
			},
			{
				ID: "2", Identifier: "PROD-18", Title: "Polish the keyboard-first issue browser",
				Description: "The selected ticket should remain obvious while moving quickly through a long list. Empty, loading and failure states need the same level of care.",
				Priority:    1, UpdatedAt: now.Add(-31 * time.Minute), CreatedAt: now.Add(-96 * time.Hour),
				State: productTodo, Team: product,
				Assignee: &marcus, Project: &launch, URL: "https://linear.app/demo/issue/PROD-18",
			},
			{
				ID: "3", Identifier: "PLAT-37", Title: "Add an end-to-end terminal test harness",
				Description: "Drive the real application with key presses and assert the visible screen. This is the reliability bar we want to borrow from LazyGit.",
				Priority:    3, UpdatedAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-7 * 24 * time.Hour),
				State: platformBacklog, Team: platform,
				Labels: []Label{{ID: "label-test", Name: "testing", Color: "#4CB782"}}, URL: "https://linear.app/demo/issue/PLAT-37",
			},
			{
				ID: "4", Identifier: "PROD-11", Title: "Define the read-only MVP boundary",
				Description: "Editing and destructive operations are explicitly deferred until browsing is fast, legible and trustworthy.",
				Priority:    4, UpdatedAt: now.Add(-24 * time.Hour), CreatedAt: now.Add(-10 * 24 * time.Hour),
				State: productDone, Team: product,
				Assignee: &aisha, Project: &launch, URL: "https://linear.app/demo/issue/PROD-11",
			},
		},
	}
}
