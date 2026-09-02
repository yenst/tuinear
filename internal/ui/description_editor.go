package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/linear"
)

type descriptionEditor struct {
	issueID    string
	identifier string
	original   linear.Issue
	value      []rune
	cursor     int
}

func (m *Model) beginDescriptionEdit() {
	issue, ok := m.beginChoiceEdit()
	if !ok {
		return
	}
	value := []rune(issue.Description)
	m.descriptionEditor = &descriptionEditor{
		issueID: issue.ID, identifier: issue.Identifier, original: issue,
		value: value, cursor: len(value),
	}
}

func (m *Model) updateDescriptionEditor(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.descriptionEditor
	if editor == nil {
		return nil
	}
	switch msg.String() {
	case "esc":
		m.descriptionEditor = nil
	case "ctrl+s":
		return m.submitDescriptionEdit()
	case "left", "ctrl+b":
		editor.cursor = max(0, editor.cursor-1)
	case "right", "ctrl+f":
		editor.cursor = min(len(editor.value), editor.cursor+1)
	case "up":
		editor.cursor = verticalCursor(editor.value, editor.cursor, -1)
	case "down":
		editor.cursor = verticalCursor(editor.value, editor.cursor, 1)
	case "home", "ctrl+a":
		editor.cursor = lineStart(editor.value, editor.cursor)
	case "end", "ctrl+e":
		editor.cursor = lineEnd(editor.value, editor.cursor)
	case "backspace", "ctrl+h":
		if editor.cursor > 0 {
			editor.value = append(editor.value[:editor.cursor-1], editor.value[editor.cursor:]...)
			editor.cursor--
		}
	case "delete":
		if editor.cursor < len(editor.value) {
			editor.value = append(editor.value[:editor.cursor], editor.value[editor.cursor+1:]...)
		}
	case "ctrl+u":
		editor.value = nil
		editor.cursor = 0
	case "enter", "return":
		insertDescriptionRunes(editor, []rune{'\n'})
	default:
		text := msg.Text
		if text == "" && msg.Code >= 32 && msg.Code != utf8.RuneError {
			text = string(msg.Code)
		}
		if text != "" && (msg.Mod == 0 || msg.Mod == tea.ModShift) {
			insertDescriptionRunes(editor, []rune(strings.ReplaceAll(text, "\r\n", "\n")))
		}
	}
	return nil
}

func insertDescriptionRunes(editor *descriptionEditor, values []rune) {
	editor.value = append(editor.value, make([]rune, len(values))...)
	copy(editor.value[editor.cursor+len(values):], editor.value[editor.cursor:])
	copy(editor.value[editor.cursor:], values)
	editor.cursor += len(values)
}

func lineStart(value []rune, cursor int) int {
	cursor = clamp(cursor, 0, len(value))
	for cursor > 0 && value[cursor-1] != '\n' {
		cursor--
	}
	return cursor
}

func lineEnd(value []rune, cursor int) int {
	cursor = clamp(cursor, 0, len(value))
	for cursor < len(value) && value[cursor] != '\n' {
		cursor++
	}
	return cursor
}

func verticalCursor(value []rune, cursor, delta int) int {
	start := lineStart(value, cursor)
	column := cursor - start
	if delta < 0 {
		if start == 0 {
			return cursor
		}
		previousEnd := start - 1
		previousStart := lineStart(value, previousEnd)
		return min(previousStart+column, previousEnd)
	}
	end := lineEnd(value, cursor)
	if end == len(value) {
		return cursor
	}
	nextStart := end + 1
	nextEnd := lineEnd(value, nextStart)
	return min(nextStart+column, nextEnd)
}

func (m *Model) submitDescriptionEdit() tea.Cmd {
	if m.descriptionEditor == nil || m.issueUpdater == nil {
		return nil
	}
	description := string(m.descriptionEditor.value)
	if description == m.descriptionEditor.original.Description {
		m.descriptionEditor = nil
		return nil
	}
	before := m.descriptionEditor.original
	optimistic := before
	optimistic.Description = description
	optimistic.UpdatedAt = time.Now()
	m.pendingEdit = &pendingIssueEdit{
		issueID: before.ID, identifier: before.Identifier, before: before,
		optimistic: optimistic, kind: editDescription,
	}
	m.descriptionEditor = nil
	m.editErr = nil
	m.replaceIssue(optimistic)
	return updateIssueDescription(m.issueUpdater, before.ID, description)
}

func updateIssueDescription(updater IssueUpdater, issueID, description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		issue, err := updater.UpdateIssue(ctx, issueID, linear.IssueUpdate{Description: &description})
		if err != nil {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save description: %w", err)}
		}
		if issue.ID != issueID {
			return issueUpdateFailedMsg{issueID: issueID, err: fmt.Errorf("save description: Linear returned a different issue")}
		}
		return issueUpdatedMsg{issue: issue}
	}
}

func (m Model) renderDescriptionEditor(width, height int) string {
	innerWidth, innerHeight := panelInnerSize(width, height)
	if m.descriptionEditor == nil {
		return panel("Edit description", width, height, fitLines(nil, innerHeight))
	}
	editor := m.descriptionEditor
	header := []string{
		accentStyle.Bold(true).Render(editor.identifier),
		mutedStyle.Render("Markdown supported · Ctrl+S saves · Esc cancels"),
		"",
	}
	lines := descriptionViewport(editor.value, editor.cursor, innerWidth, max(1, innerHeight-len(header)))
	return panel("Edit description", width, height, fitLines(append(header, lines...), innerHeight))
}

func descriptionViewport(value []rune, cursor, width, height int) []string {
	cursor = clamp(cursor, 0, len(value))
	width = max(1, width)
	marked := append([]rune(nil), value...)
	marked = append(marked, 0)
	copy(marked[cursor+1:], marked[cursor:])
	marked[cursor] = '│'
	lines := []string{}
	line := []rune{}
	cursorLine := 0
	for index, value := range marked {
		if value == '\n' {
			lines = append(lines, string(line))
			line = nil
			continue
		}
		if len(line) == width {
			lines = append(lines, string(line))
			line = nil
		}
		line = append(line, value)
		if index == cursor {
			cursorLine = len(lines)
		}
	}
	lines = append(lines, string(line))
	start := 0
	if cursorLine >= height {
		start = cursorLine - height + 1
	}
	return lines[start:min(len(lines), start+height)]
}
