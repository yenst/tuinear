package linear

import (
	"context"
	"time"
)

type DemoClient struct{}

func (DemoClient) FetchDashboard(context.Context) (Dashboard, error) {
	now := time.Now()
	platform := Team{ID: "team-platform", Key: "PLAT", Name: "Platform"}
	product := Team{ID: "team-product", Key: "PROD", Name: "Product"}
	aisha := User{ID: "user-aisha", Name: "Aisha Chen", DisplayName: "Aisha"}
	marcus := User{ID: "user-marcus", Name: "Marcus Bell", DisplayName: "Marcus"}
	launch := Project{ID: "project-launch", Name: "Public launch"}
	quality := Label{ID: "label-quality", Name: "quality", Color: "#5E6AD2"}

	return Dashboard{
		Viewer: Viewer{ID: "viewer", Name: "Jamie", DisplayName: "Jamie", Email: "jamie@example.com"},
		Teams:  []Team{platform, product},
		Issues: []Issue{
			{
				ID: "1", Identifier: "PLAT-42", Title: "Persist the issue cache between sessions",
				Description: "Warm startup should render cached tickets immediately, then reconcile with Linear in the background.",
				Priority:    2, UpdatedAt: now.Add(-8 * time.Minute), CreatedAt: now.Add(-72 * time.Hour),
				State: WorkflowState{Name: "In Progress", Type: "started", Color: "#F2C94C"}, Team: platform,
				Assignee: &aisha, Project: &launch, Labels: []Label{quality}, URL: "https://linear.app/demo/issue/PLAT-42",
			},
			{
				ID: "2", Identifier: "PROD-18", Title: "Polish the keyboard-first issue browser",
				Description: "The selected ticket should remain obvious while moving quickly through a long list. Empty, loading and failure states need the same level of care.",
				Priority:    1, UpdatedAt: now.Add(-31 * time.Minute), CreatedAt: now.Add(-96 * time.Hour),
				State: WorkflowState{Name: "Todo", Type: "unstarted", Color: "#5E6AD2"}, Team: product,
				Assignee: &marcus, Project: &launch, URL: "https://linear.app/demo/issue/PROD-18",
			},
			{
				ID: "3", Identifier: "PLAT-37", Title: "Add an end-to-end terminal test harness",
				Description: "Drive the real application with key presses and assert the visible screen. This is the reliability bar we want to borrow from LazyGit.",
				Priority:    3, UpdatedAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-7 * 24 * time.Hour),
				State: WorkflowState{Name: "Backlog", Type: "backlog", Color: "#858585"}, Team: platform,
				Labels: []Label{{ID: "label-test", Name: "testing", Color: "#4CB782"}}, URL: "https://linear.app/demo/issue/PLAT-37",
			},
			{
				ID: "4", Identifier: "PROD-11", Title: "Define the read-only MVP boundary",
				Description: "Editing and destructive operations are explicitly deferred until browsing is fast, legible and trustworthy.",
				Priority:    4, UpdatedAt: now.Add(-24 * time.Hour), CreatedAt: now.Add(-10 * 24 * time.Hour),
				State: WorkflowState{Name: "Done", Type: "completed", Color: "#4CB782"}, Team: product,
				Assignee: &aisha, Project: &launch, URL: "https://linear.app/demo/issue/PROD-11",
			},
		},
	}, nil
}
