package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yenst/tuinear/internal/linear"
)

func (m *Model) updateCreateIssueEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.createEditor
	if editor == nil {
		return nil
	}
	switch editor.mode {
	case createEditTitle:
		m.updateCreateTitle(msg)
	case createEditDescription:
		m.updateCreateDescription(msg)
	case createEditChoice:
		m.updateCreateChoice(msg)
	case createEditLabels:
		m.updateCreateLabels(msg)
	default:
		return m.updateCreateForm(msg)
	}
	return nil
}

func (m *Model) updateCreateForm(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.createEditor
	switch msg.String() {
	case "esc":
		m.createEditor = nil
	case "ctrl+s":
		return m.submitIssueCreate()
	case "j", "down", "tab":
		editor.selected = (editor.selected + 1) % (createSubmit + 1)
	case "k", "up", "shift+tab":
		editor.selected = (editor.selected + createSubmit) % (createSubmit + 1)
	case "g", "home":
		editor.selected = createTitle
	case "G", "end":
		editor.selected = createSubmit
	case "enter", "return":
		if editor.selected == createSubmit {
			return m.submitIssueCreate()
		}
		m.beginCreateFieldEdit()
	}
	return nil
}

func (m *Model) beginCreateFieldEdit() {
	editor := m.createEditor
	if editor == nil {
		return
	}
	editor.err = nil
	switch editor.selected {
	case createTitle:
		editor.mode = createEditTitle
		editor.titleCursor = clamp(editor.titleCursor, 0, len(editor.title))
	case createDescription:
		editor.mode = createEditDescription
		editor.descriptionCursor = clamp(editor.descriptionCursor, 0, len(editor.description))
	case createStatus:
		editor.choiceOptions = statusChoices(m.editableStates(createDraftIssue(editor)))
		if len(editor.choiceOptions) == 0 {
			editor.err = fmt.Errorf("no statuses are available for %s", editor.team.Name)
			return
		}
		editor.choiceSelected = choiceIndex(editor.choiceOptions, func(choice issueChoice) bool {
			return choice.state.ID == editor.state.ID
		})
		editor.mode = createEditChoice
	case createPriority:
		editor.choiceOptions = priorityChoices()
		editor.choiceSelected = choiceIndex(editor.choiceOptions, func(choice issueChoice) bool {
			return choice.priority == editor.priority
		})
		editor.mode = createEditChoice
	case createAssignee:
		editor.choiceOptions = assigneeChoices(m.dashboard.Users, editor.assignee)
		editor.choiceSelected = choiceIndex(editor.choiceOptions, func(choice issueChoice) bool {
			return assigneeID(choice.assignee) == assigneeID(editor.assignee)
		})
		editor.mode = createEditChoice
	case createProject:
		editor.choiceOptions = projectChoices(m.editableProjects(createDraftIssue(editor)), editor.project)
		editor.choiceSelected = choiceIndex(editor.choiceOptions, func(choice issueChoice) bool {
			return projectID(choice.project) == projectID(editor.project)
		})
		editor.mode = createEditChoice
	case createLabels:
		editor.labelOptions = labelChoices(m.editableLabels(createDraftIssue(editor)), editor.labels)
		if len(editor.labelOptions) == 0 {
			editor.err = fmt.Errorf("no labels are available for %s", editor.team.Name)
			return
		}
		editor.labelSelected = 0
		editor.labelChecked = make(map[string]bool, len(editor.labels))
		for _, label := range editor.labels {
			editor.labelChecked[label.ID] = true
		}
		editor.mode = createEditLabels
	}
}

func choiceIndex(options []issueChoice, matches func(issueChoice) bool) int {
	for index, option := range options {
		if matches(option) {
			return index
		}
	}
	return 0
}

func createDraftIssue(editor *createIssueEditor) linear.Issue {
	return linear.Issue{
		Title: string(editor.title), Description: string(editor.description),
		State: editor.state, Priority: editor.priority, Assignee: editor.assignee,
		Team: editor.team, Project: editor.project, Labels: editor.labels,
	}
}

func (m *Model) updateCreateTitle(msg tea.KeyPressMsg) {
	editor := m.createEditor
	switch msg.String() {
	case "esc":
		editor.mode = createBrowse
	case "enter", "return":
		editor.mode = createBrowse
		editor.selected = createDescription
	case "left", "ctrl+b":
		editor.titleCursor = max(0, editor.titleCursor-1)
	case "right", "ctrl+f":
		editor.titleCursor = min(len(editor.title), editor.titleCursor+1)
	case "home", "ctrl+a":
		editor.titleCursor = 0
	case "end", "ctrl+e":
		editor.titleCursor = len(editor.title)
	case "backspace", "ctrl+h":
		if editor.titleCursor > 0 {
			editor.title = append(editor.title[:editor.titleCursor-1], editor.title[editor.titleCursor:]...)
			editor.titleCursor--
		}
	case "delete":
		if editor.titleCursor < len(editor.title) {
			editor.title = append(editor.title[:editor.titleCursor], editor.title[editor.titleCursor+1:]...)
		}
	case "ctrl+u":
		editor.title = nil
		editor.titleCursor = 0
	default:
		text := keyText(msg)
		if text != "" {
			insert := []rune(strings.ReplaceAll(text, "\n", " "))
			editor.title = insertRunes(editor.title, editor.titleCursor, insert)
			editor.titleCursor += len(insert)
		}
	}
}

func (m *Model) updateCreateDescription(msg tea.KeyPressMsg) {
	editor := m.createEditor
	switch msg.String() {
	case "esc":
		editor.mode = createBrowse
	case "ctrl+s":
		editor.mode = createBrowse
		editor.selected = createStatus
	case "left", "ctrl+b":
		editor.descriptionCursor = max(0, editor.descriptionCursor-1)
	case "right", "ctrl+f":
		editor.descriptionCursor = min(len(editor.description), editor.descriptionCursor+1)
	case "up":
		editor.descriptionCursor = verticalCursor(editor.description, editor.descriptionCursor, -1)
	case "down":
		editor.descriptionCursor = verticalCursor(editor.description, editor.descriptionCursor, 1)
	case "home", "ctrl+a":
		editor.descriptionCursor = lineStart(editor.description, editor.descriptionCursor)
	case "end", "ctrl+e":
		editor.descriptionCursor = lineEnd(editor.description, editor.descriptionCursor)
	case "backspace", "ctrl+h":
		if editor.descriptionCursor > 0 {
			editor.description = append(editor.description[:editor.descriptionCursor-1], editor.description[editor.descriptionCursor:]...)
			editor.descriptionCursor--
		}
	case "delete":
		if editor.descriptionCursor < len(editor.description) {
			editor.description = append(editor.description[:editor.descriptionCursor], editor.description[editor.descriptionCursor+1:]...)
		}
	case "ctrl+u":
		editor.description = nil
		editor.descriptionCursor = 0
	case "enter", "return":
		editor.description = insertRunes(editor.description, editor.descriptionCursor, []rune{'\n'})
		editor.descriptionCursor++
	default:
		text := keyText(msg)
		if text != "" {
			insert := []rune(strings.ReplaceAll(text, "\r\n", "\n"))
			editor.description = insertRunes(editor.description, editor.descriptionCursor, insert)
			editor.descriptionCursor += len(insert)
		}
	}
}

func keyText(msg tea.KeyPressMsg) string {
	text := msg.Text
	if text == "" && msg.Code >= 32 && msg.Code != utf8.RuneError {
		text = string(msg.Code)
	}
	if text == "" || (msg.Mod != 0 && msg.Mod != tea.ModShift) {
		return ""
	}
	return text
}

func insertRunes(value []rune, cursor int, insert []rune) []rune {
	value = append(value, make([]rune, len(insert))...)
	copy(value[cursor+len(insert):], value[cursor:])
	copy(value[cursor:], insert)
	return value
}

func (m *Model) updateCreateChoice(msg tea.KeyPressMsg) {
	editor := m.createEditor
	if len(editor.choiceOptions) == 0 {
		editor.mode = createBrowse
		return
	}
	switch msg.String() {
	case "esc":
		editor.mode = createBrowse
	case "j", "down", "right", "tab":
		editor.choiceSelected = (editor.choiceSelected + 1) % len(editor.choiceOptions)
	case "k", "up", "left", "shift+tab":
		editor.choiceSelected = (editor.choiceSelected - 1 + len(editor.choiceOptions)) % len(editor.choiceOptions)
	case "g", "home":
		editor.choiceSelected = 0
	case "G", "end":
		editor.choiceSelected = len(editor.choiceOptions) - 1
	case "enter", "return":
		choice := editor.choiceOptions[editor.choiceSelected]
		switch editor.selected {
		case createStatus:
			editor.state = choice.state
		case createPriority:
			editor.priority = choice.priority
		case createAssignee:
			editor.assignee = cloneUser(choice.assignee)
		case createProject:
			editor.project = cloneProject(choice.project)
		}
		editor.mode = createBrowse
		editor.selected++
	}
}

func (m *Model) updateCreateLabels(msg tea.KeyPressMsg) {
	editor := m.createEditor
	if len(editor.labelOptions) == 0 {
		editor.mode = createBrowse
		return
	}
	switch msg.String() {
	case "esc":
		editor.mode = createBrowse
	case "j", "down", "right", "tab":
		editor.labelSelected = (editor.labelSelected + 1) % len(editor.labelOptions)
	case "k", "up", "left", "shift+tab":
		editor.labelSelected = (editor.labelSelected - 1 + len(editor.labelOptions)) % len(editor.labelOptions)
	case "g", "home":
		editor.labelSelected = 0
	case "G", "end":
		editor.labelSelected = len(editor.labelOptions) - 1
	case "space", " ":
		id := editor.labelOptions[editor.labelSelected].ID
		editor.labelChecked[id] = !editor.labelChecked[id]
	case "enter", "return":
		editor.labels = editor.labels[:0]
		for _, label := range editor.labelOptions {
			if editor.labelChecked[label.ID] {
				editor.labels = append(editor.labels, label)
			}
		}
		editor.mode = createBrowse
		editor.selected = createSubmit
	}
}

func (m Model) renderCreateIssueEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.createEditor == nil {
		return panel("Create ticket", width, height, fitLines(nil, innerHeight))
	}
	switch m.createEditor.mode {
	case createEditDescription:
		return m.renderCreateDescription(width, height, innerWidth, innerHeight)
	case createEditChoice:
		return m.renderCreateChoices(width, height, innerWidth, innerHeight)
	case createEditLabels:
		return m.renderCreateLabels(width, height, innerWidth, innerHeight)
	default:
		return m.renderCreateForm(width, height, innerWidth, innerHeight)
	}
}

func (m Model) renderCreateForm(width, height, innerWidth, innerHeight int) string {
	editor := m.createEditor
	lines := []string{
		accentStyle.Bold(true).Render(editor.team.Key + " · " + editor.team.Name),
		mutedStyle.Render("Enter edits a field · Ctrl+S creates · Esc cancels"),
		"",
	}
	values := []struct{ label, value string }{
		{"Title", createTitleValue(editor, innerWidth)},
		{"Description", descriptionSummary(editor.description)},
		{"Status", valueOr(editor.state.Name, "Team default")},
		{"Priority", priorityLabel(editor.priority)},
		{"Assignee", userLabel(editor.assignee)},
		{"Project", projectLabel(editor.project)},
		{"Labels", labelsLabel(editor.labels)},
		{"Create ticket", "Send to Linear"},
	}
	for index, row := range values {
		prefix := "  "
		if createIssueField(index) == editor.selected {
			prefix = "› "
		}
		label := prefix + row.label
		gap := max(1, 18-lipgloss.Width(label))
		line := label + strings.Repeat(" ", gap) + clip(row.value, max(1, innerWidth-18))
		if createIssueField(index) == editor.selected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(theme.text).Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	if editor.err != nil {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(theme.red).Render("Error: "+editor.err.Error()))
	}
	return panel("Create ticket", width, height, fitLines(lines, innerHeight))
}

func createTitleValue(editor *createIssueEditor, width int) string {
	if editor.mode == createEditTitle {
		return editorWindow(editor.title, editor.titleCursor, max(1, width-18))
	}
	if strings.TrimSpace(string(editor.title)) == "" {
		return "Required"
	}
	return string(editor.title)
}

func descriptionSummary(value []rune) string {
	if len(value) == 0 {
		return "Empty"
	}
	first := strings.SplitN(string(value), "\n", 2)[0]
	if strings.TrimSpace(first) == "" {
		return "Markdown description"
	}
	return first
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func priorityLabel(priority int) string {
	return (linear.Issue{Priority: priority}).PriorityLabel()
}

func userLabel(user *linear.User) string {
	if user == nil {
		return "Unassigned"
	}
	return user.Label()
}

func projectLabel(project *linear.Project) string {
	if project == nil {
		return "No project"
	}
	return project.Name
}

func labelsLabel(labels []linear.Label) string {
	if len(labels) == 0 {
		return "No labels"
	}
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, label.Name)
	}
	return strings.Join(names, ", ")
}

func (m Model) renderCreateDescription(width, height, innerWidth, innerHeight int) string {
	editor := m.createEditor
	header := []string{
		accentStyle.Bold(true).Render(editor.team.Key + " · Description"),
		mutedStyle.Render("Markdown supported · Ctrl+S returns to the form · Esc keeps the draft"),
		"",
	}
	lines := descriptionViewport(editor.description, editor.descriptionCursor, innerWidth, max(1, innerHeight-len(header)))
	return panel("Create ticket · description", width, height, fitLines(append(header, lines...), innerHeight))
}

func (m Model) renderCreateChoices(width, height, innerWidth, innerHeight int) string {
	editor := m.createEditor
	field := createFieldName(editor.selected)
	lines := []string{
		accentStyle.Bold(true).Render(editor.team.Key + " · " + field),
		mutedStyle.Render("Choose a value, then press Enter · Esc keeps the current value"),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := max(0, editor.choiceSelected-capacity+1)
	end := min(len(editor.choiceOptions), start+capacity)
	for index := start; index < end; index++ {
		choice := editor.choiceOptions[index]
		prefix := "  "
		if index == editor.choiceSelected {
			prefix = "› "
		}
		line := prefix + choice.glyph + " " + clip(choice.label, max(1, innerWidth-4))
		if index == editor.choiceSelected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Create ticket · "+strings.ToLower(field), width, height, fitLines(lines, innerHeight))
}

func (m Model) renderCreateLabels(width, height, innerWidth, innerHeight int) string {
	editor := m.createEditor
	lines := []string{
		accentStyle.Bold(true).Render(editor.team.Key + " · Labels"),
		mutedStyle.Render("Space toggles · Enter applies · Esc keeps the current labels"),
		"",
	}
	capacity := max(1, innerHeight-len(lines))
	start := max(0, editor.labelSelected-capacity+1)
	end := min(len(editor.labelOptions), start+capacity)
	for index := start; index < end; index++ {
		label := editor.labelOptions[index]
		marker := "[ ]"
		if editor.labelChecked[label.ID] {
			marker = "[x]"
		}
		prefix := "  "
		if index == editor.labelSelected {
			prefix = "› "
		}
		line := prefix + marker + " " + clip(label.Name, max(1, innerWidth-6))
		if index == editor.labelSelected {
			line = selectedRowStyle.Width(innerWidth).Render(line)
		}
		lines = append(lines, line)
	}
	return panel("Create ticket · labels", width, height, fitLines(lines, innerHeight))
}

func createFieldName(field createIssueField) string {
	switch field {
	case createStatus:
		return "Status"
	case createPriority:
		return "Priority"
	case createAssignee:
		return "Assignee"
	case createProject:
		return "Project"
	default:
		return "Field"
	}
}
