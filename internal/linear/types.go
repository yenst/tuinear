package linear

import "time"

// Dashboard is the read-only workspace data needed by the MVP.
type Dashboard struct {
	Viewer Viewer
	Teams  []Team
	Issues []Issue
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
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

type Issue struct {
	ID          string        `json:"id"`
	Identifier  string        `json:"identifier"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Priority    int           `json:"priority"`
	URL         string        `json:"url"`
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
