package issuefilter

// State contains the composable local issue filters that may be persisted for
// a Linear profile. Empty include values and empty exclusion lists mean any.
type State struct {
	Assignee string `json:"assignee,omitempty"`
	Status   string `json:"status,omitempty"`
	Priority string `json:"priority,omitempty"`
	Project  string `json:"project,omitempty"`

	ExcludedAssignees  []string `json:"excluded_assignees,omitempty"`
	ExcludedStatuses   []string `json:"excluded_statuses,omitempty"`
	ExcludedPriorities []string `json:"excluded_priorities,omitempty"`
	ExcludedProjects   []string `json:"excluded_projects,omitempty"`
}

// Clone returns a copy whose slices do not share backing storage with State.
func (s State) Clone() State {
	s.ExcludedAssignees = append([]string(nil), s.ExcludedAssignees...)
	s.ExcludedStatuses = append([]string(nil), s.ExcludedStatuses...)
	s.ExcludedPriorities = append([]string(nil), s.ExcludedPriorities...)
	s.ExcludedProjects = append([]string(nil), s.ExcludedProjects...)
	return s
}

func (s State) Empty() bool {
	return s.Assignee == "" && s.Status == "" && s.Priority == "" && s.Project == "" &&
		len(s.ExcludedAssignees) == 0 && len(s.ExcludedStatuses) == 0 &&
		len(s.ExcludedPriorities) == 0 && len(s.ExcludedProjects) == 0
}
