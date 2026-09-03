package linear

import "time"

// Dashboard is the workspace data needed by Tuinear's issue browser and safe
// field editors.
type Dashboard struct {
	Viewer          Viewer
	Organization    Organization
	Accounts        []Account
	ActiveAccountID string
	Teams           []Team
	TeamStates      []TeamWorkflowStates
	TeamProjects    []TeamProjects
	WorkspaceLabels []Label
	TeamLabels      []TeamLabels
	Users           []User
	Issues          []Issue
}

type Account struct {
	ID            string
	WorkspaceName string
	WorkspaceKey  string
	UserName      string
	UserEmail     string
}

func (a Account) Label() string {
	workspace := a.WorkspaceName
	if workspace == "" {
		workspace = a.WorkspaceKey
	}
	user := a.UserName
	if user == "" {
		user = a.UserEmail
	}
	if workspace == "" {
		return user
	}
	if user == "" {
		return workspace
	}
	return workspace + " / " + user
}

type Organization struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URLKey string `json:"urlKey"`
}

type Viewer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

func (v Viewer) Label() string {
	if v.DisplayName != "" {
		return v.DisplayName
	}
	if v.Name != "" {
		return v.Name
	}
	return v.Email
}

type Team struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type WorkflowState struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Color    string  `json:"color"`
	Position float64 `json:"position"`
}

type TeamWorkflowStates struct {
	TeamID string
	States []WorkflowState
}

type TeamProjects struct {
	TeamID   string
	Projects []Project
}

func (d Dashboard) StatesForTeam(teamID string) []WorkflowState {
	for _, group := range d.TeamStates {
		if group.TeamID == teamID {
			return group.States
		}
	}
	return nil
}

func (d Dashboard) ProjectsForTeam(teamID string) []Project {
	for _, group := range d.TeamProjects {
		if group.TeamID == teamID {
			return group.Projects
		}
	}
	return nil
}

type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func (u User) Label() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TeamLabels struct {
	TeamID string
	Labels []Label
}

func (d Dashboard) LabelsForTeam(teamID string) []Label {
	seen := make(map[string]bool, len(d.WorkspaceLabels))
	labels := make([]Label, 0, len(d.WorkspaceLabels))
	for _, label := range d.WorkspaceLabels {
		if label.ID == "" || seen[label.ID] {
			continue
		}
		seen[label.ID] = true
		labels = append(labels, label)
	}
	for _, group := range d.TeamLabels {
		if group.TeamID != teamID {
			continue
		}
		for _, label := range group.Labels {
			if label.ID == "" || seen[label.ID] {
				continue
			}
			seen[label.ID] = true
			labels = append(labels, label)
		}
		break
	}
	return labels
}

type Issue struct {
	ID          string        `json:"id"`
	Identifier  string        `json:"identifier"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Priority    int           `json:"priority"`
	URL         string        `json:"url"`
	BranchName  string        `json:"branchName"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	State       WorkflowState `json:"state"`
	Assignee    *User         `json:"assignee"`
	Team        Team          `json:"team"`
	Project     *Project      `json:"project"`
	Labels      []Label       `json:"-"`
	LabelData   struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
}

// IssueUpdate contains only fields that Tuinear is allowed to mutate.
// AssigneeID and ProjectID use two pointer levels so nil omits the field, while
// a non-nil outer pointer containing nil explicitly sends null to clear it.
type IssueUpdate struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	StateID     *string   `json:"stateId,omitempty"`
	Priority    *int      `json:"priority,omitempty"`
	AssigneeID  **string  `json:"assigneeId,omitempty"`
	ProjectID   **string  `json:"projectId,omitempty"`
	LabelIDs    *[]string `json:"labelIds,omitempty"`
}

// IssueCreate contains the fields supported by Tuinear's creation form.
// Pointer priority preserves an explicit "no priority" value (zero).
type IssueCreate struct {
	TeamID      string   `json:"teamId"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	StateID     string   `json:"stateId,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	AssigneeID  string   `json:"assigneeId,omitempty"`
	ProjectID   string   `json:"projectId,omitempty"`
	LabelIDs    []string `json:"labelIds,omitempty"`
}

func (i *Issue) Normalize() {
	if len(i.Labels) == 0 && len(i.LabelData.Nodes) > 0 {
		i.Labels = append([]Label(nil), i.LabelData.Nodes...)
	}
}

func (i Issue) PriorityLabel() string {
	switch i.Priority {
	case 1:
		return "Urgent"
	case 2:
		return "High"
	case 3:
		return "Medium"
	case 4:
		return "Low"
	default:
		return "No priority"
	}
}
