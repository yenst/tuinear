package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) copySelectedIssue(kind issueCopyKind) tea.Cmd {
	issue := m.selectedIssue()
	if issue == nil {
		return issueCopyFailure(kind, "no issue is selected")
	}
	value := issue.URL
	if kind == issueCopyBranch {
		value = issue.BranchName
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return issueCopyFailure(kind, fmt.Sprintf("selected issue has no %s", kind))
	}
	if m.clipboardSet == nil {
		return issueCopyFailure(kind, "clipboard is not configured")
	}
	m.clipboardErr = nil
	m.clipboardNotice = ""
	identifier := issue.Identifier
	return func() tea.Msg {
		return issueCopiedMsg{kind: kind, identifier: identifier, value: value}
	}
}

func issueCopyFailure(kind issueCopyKind, reason string) tea.Cmd {
	return func() tea.Msg {
		return issueCopyFailedMsg{err: fmt.Errorf("copy %s: %s", kind, reason)}
	}
}
