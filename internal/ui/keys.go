package ui

import tea "charm.land/bubbletea/v2"

func (m *Model) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}
	// Account selection changes the loader before its dashboard arrives.
	// Keep actions tied to the visible account until that transition finishes.
	if m.loading {
		if keyMatches(msg, "q") {
			return tea.Quit
		}
		return nil
	}
	if m.editor != nil {
		return m.updateTitleEditor(msg)
	}
	if m.createEditor != nil {
		return m.updateCreateIssueEditor(msg)
	}
	if m.choiceEditor != nil {
		return m.updateChoiceEditor(msg)
	}
	if m.labelEditor != nil {
		return m.updateLabelEditor(msg)
	}
	if m.descriptionEditor != nil {
		return m.updateDescriptionEditor(msg)
	}
	if m.archiveConfirm != nil {
		return m.updateArchiveConfirmation(msg)
	}
	if m.actionMenu != nil {
		return m.updateActionMenu(msg)
	}
	if m.help {
		if keyMatches(msg, "h", "H", "shift+h", "?", "shift+/", "esc") {
			m.help = false
		}
		return nil
	}
	if m.pendingArchive != nil || m.pendingCreate != nil {
		return nil
	}
	if m.palette {
		return m.updatePalette(msg)
	}
	if m.searching {
		m.updateSearch(msg)
		return nil
	}
	switch {
	case keyMatches(msg, "q"):
		return tea.Quit
	case keyMatches(msg, "r"):
		if m.refreshing || m.pendingEdit != nil || m.pendingCreate != nil {
			return nil
		}
		m.err = nil
		m.refreshErr = nil
		if m.hasDashboard() {
			m.refreshing = true
		} else {
			m.loading = true
		}
		return loadDashboard(m.loader)
	case keyMatches(msg, "a"):
		if m.pendingEdit != nil || m.pendingCreate != nil {
			return nil
		}
		return m.cycleAccount(1)
	case keyMatches(msg, "A", "shift+a"):
		if m.pendingEdit != nil || m.pendingCreate != nil {
			return nil
		}
		return m.cycleAccount(-1)
	case keyMatches(msg, "h", "H", "shift+h", "?", "shift+/"):
		m.help = true
	case keyMatches(msg, "/"):
		m.searching = true
	case keyMatches(msg, "f", "ctrl+f"):
		m.palette = true
		m.paletteIdx = 0
	case keyMatches(msg, "space", " "):
		return m.openSelectedIssue()
	case keyMatches(msg, "enter", "return"):
		m.beginIssueActions()
	case keyMatches(msg, "n"):
		m.beginIssueCreate()
	case keyMatches(msg, "e"):
		m.beginTitleEdit()
	case keyMatches(msg, "s"):
		m.beginStatusEdit()
	case keyMatches(msg, "p"):
		m.beginPriorityEdit()
	case keyMatches(msg, "u"):
		m.beginAssigneeEdit()
	case keyMatches(msg, "P", "shift+p"):
		m.beginProjectEdit()
	case keyMatches(msg, "l"):
		m.beginLabelEdit()
	case keyMatches(msg, "d"):
		m.beginDescriptionEdit()
	case keyMatches(msg, "x"):
		m.beginArchiveConfirmation()
	case keyMatches(msg, "esc"):
		if m.query != "" {
			m.query = ""
			m.filterIssues()
		} else if m.hasFilters() {
			m.filters = IssueFilters{}
			m.filterIssues()
			return m.saveIssueFilters()
		}
	case keyMatches(msg, "j", "down"):
		m.moveSelection(1)
	case keyMatches(msg, "k", "up"):
		m.moveSelection(-1)
	case keyMatches(msg, "g", "home"):
		m.selected = 0
	case keyMatches(msg, "G", "shift+g"):
		return m.copySelectedIssue(issueCopyBranch)
	case keyMatches(msg, "c"):
		return m.copySelectedIssue(issueCopyURL)
	case keyMatches(msg, "end"):
		if len(m.issues) > 0 {
			m.selected = len(m.issues) - 1
		}
	case keyMatches(msg, "tab", "]"):
		m.cycleTeam(1)
	case keyMatches(msg, "shift+tab", "["):
		m.cycleTeam(-1)
	}
	return nil
}

func keyMatches(msg tea.KeyPressMsg, names ...string) bool {
	text := msg.String()
	keystroke := msg.Keystroke()
	for _, name := range names {
		if text == name || keystroke == name {
			return true
		}
	}
	return false
}
