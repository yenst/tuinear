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
		if update.Description != nil {
			issue.Description = *update.Description
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
		if update.Priority != nil {
			if *update.Priority < 0 || *update.Priority > 4 {
				return Issue{}, fmt.Errorf("issue priority must be between 0 and 4")
			}
			issue.Priority = *update.Priority
			issue.UpdatedAt = time.Now()
		}
		if update.AssigneeID != nil {
			issue.Assignee = nil
			if *update.AssigneeID != nil {
				assigneeID := strings.TrimSpace(**update.AssigneeID)
				for _, user := range dashboard.Users {
					if user.ID == assigneeID {
						assignee := user
						issue.Assignee = &assignee
						break
					}
				}
				if issue.Assignee == nil {
					return Issue{}, fmt.Errorf("unknown demo assignee %q", assigneeID)
				}
			}
			issue.UpdatedAt = time.Now()
		}
		if update.ProjectID != nil {
			issue.Project = nil
			if *update.ProjectID != nil {
				projectID := strings.TrimSpace(**update.ProjectID)
				for _, project := range dashboard.ProjectsForTeam(issue.Team.ID) {
					if project.ID == projectID {
						selected := project
						issue.Project = &selected
						break
					}
				}
				if issue.Project == nil {
					return Issue{}, fmt.Errorf("unknown demo project %q", projectID)
				}
			}
			issue.UpdatedAt = time.Now()
		}
		if update.LabelIDs != nil {
			available := dashboard.LabelsForTeam(issue.Team.ID)
			byID := make(map[string]Label, len(available))
			for _, label := range available {
				byID[label.ID] = label
			}
			labels := make([]Label, 0, len(*update.LabelIDs))
			seen := make(map[string]bool, len(*update.LabelIDs))
			for _, value := range *update.LabelIDs {
				labelID := strings.TrimSpace(value)
				label, ok := byID[labelID]
				if !ok {
					return Issue{}, fmt.Errorf("unknown demo label %q", labelID)
				}
				if !seen[labelID] {
					seen[labelID] = true
					labels = append(labels, label)
				}
			}
			issue.Labels = labels
			issue.UpdatedAt = time.Now()
		}
		return issue, nil
	}
	return Issue{}, fmt.Errorf("unknown demo issue %q", issueID)
}

func (DemoClient) ArchiveIssue(_ context.Context, issueID string) error {
	for _, issue := range demoDashboard("demo-work").Issues {
		if issue.ID == issueID {
			return nil
		}
	}
	return fmt.Errorf("unknown demo issue %q", issueID)
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
	platformQuality := Project{ID: "project-platform-quality", Name: "Platform quality"}
	productPolish := Project{ID: "project-product-polish", Name: "Product polish"}
	quality := Label{ID: "label-quality", Name: "quality", Color: "#5E6AD2"}
	testing := Label{ID: "label-testing", Name: "testing", Color: "#4CB782"}
	platformLabel := Label{ID: "label-platform", Name: "platform", Color: "#F2C94C"}
	productLabel := Label{ID: "label-product", Name: "product", Color: "#EB5757"}

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
	jamie := User{ID: viewer.ID, Name: viewer.Name, DisplayName: viewer.DisplayName}

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
		TeamProjects: []TeamProjects{
			{TeamID: platform.ID, Projects: []Project{launch, platformQuality}},
			{TeamID: product.ID, Projects: []Project{launch, productPolish}},
		},
		WorkspaceLabels: []Label{quality, testing},
		TeamLabels: []TeamLabels{
			{TeamID: platform.ID, Labels: []Label{platformLabel}},
			{TeamID: product.ID, Labels: []Label{productLabel}},
		},
		Users: []User{jamie, aisha, marcus},
		Issues: []Issue{
			{
				ID: "1", Identifier: "PLAT-42", Title: "Persist the issue cache between sessions",
				Description: "Warm startup should render cached tickets immediately, then reconcile with Linear in the background.",
				Priority:    2, UpdatedAt: now.Add(-8 * time.Minute), CreatedAt: now.Add(-72 * time.Hour),
				State: platformProgress, Team: platform,
				Assignee: &aisha, Project: &launch, Labels: []Label{quality}, URL: "https://linear.app/demo/issue/PLAT-42", BranchName: "persist-issue-cache",
			},
			{
				ID: "2", Identifier: "PROD-18", Title: "Polish the keyboard-first issue browser",
				Description: "The selected ticket should remain obvious while moving quickly through a long list. Empty, loading and failure states need the same level of care.",
				Priority:    1, UpdatedAt: now.Add(-31 * time.Minute), CreatedAt: now.Add(-96 * time.Hour),
				State: productTodo, Team: product,
				Assignee: &marcus, Project: &launch, URL: "https://linear.app/demo/issue/PROD-18", BranchName: "polish-keyboard-browser",
			},
			{
				ID: "3", Identifier: "PLAT-37", Title: "Add an end-to-end terminal test harness",
				Description: "Drive the real application with key presses and assert the visible screen. This is the reliability bar we want to borrow from LazyGit.",
				Priority:    3, UpdatedAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-7 * 24 * time.Hour),
				State: platformBacklog, Team: platform,
				Labels: []Label{testing}, URL: "https://linear.app/demo/issue/PLAT-37", BranchName: "add-terminal-test-harness",
			},
			{
				ID: "4", Identifier: "PROD-11", Title: "Define the read-only MVP boundary",
				Description: "Editing and destructive operations are explicitly deferred until browsing is fast, legible and trustworthy.",
				Priority:    4, UpdatedAt: now.Add(-24 * time.Hour), CreatedAt: now.Add(-10 * 24 * time.Hour),
				State: productDone, Team: product,
				Assignee: &aisha, Project: &launch, URL: "https://linear.app/demo/issue/PROD-11", BranchName: "define-mvp-boundary",
			},
		},
	}
}
