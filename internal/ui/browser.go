package ui

import (
	"fmt"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) openSelectedIssue() tea.Cmd {
	issue := m.selectedIssue()
	if issue == nil || strings.TrimSpace(issue.URL) == "" {
		return nil
	}
	issueURL, err := validateIssueURL(issue.URL)
	if err != nil {
		m.browserErr = err
		return nil
	}
	if m.browserOpen == nil {
		m.browserErr = fmt.Errorf("open issue in browser: browser launcher is not configured")
		return nil
	}
	m.browserErr = nil
	return openIssueURL(m.browserOpen, issueURL)
}

func openIssueURL(opener BrowserOpener, issueURL string) tea.Cmd {
	return func() tea.Msg {
		if err := opener(issueURL); err != nil {
			return issueBrowserFailedMsg{err: fmt.Errorf("open issue in browser: %w", err)}
		}
		return issueBrowserOpenedMsg{}
	}
}

func validateIssueURL(raw string) (string, error) {
	issueURL := strings.TrimSpace(raw)
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("cannot open issue URL: URL must use http or https")
	}
	return issueURL, nil
}
