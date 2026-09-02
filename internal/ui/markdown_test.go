package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderMarkdownUsesVisibleFormatting(t *testing.T) {
	source := "# Heading\n\nThis is **strong**, *emphasized*, and `code`.\n\n- first item\n- [Linear](https://linear.app)\n\n> quoted text\n\n```go\nfmt.Println(\"hello\")\n```"
	lines := renderMarkdown(source, 80)
	if len(lines) == 0 {
		t.Fatal("renderMarkdown returned no lines")
	}
	visible := strings.Join(lines, "\n")
	for _, want := range []string{"Heading", "strong", "emphasized", "code", "first item", "Linear", "quoted text", "fmt.Println"} {
		if !strings.Contains(visible, want) {
			t.Errorf("rendered Markdown missing %q: %q", want, visible)
		}
	}
	for _, raw := range []string{"# Heading", "**strong**", "*emphasized*", "`code`", "- first item", "[Linear]", "> quoted", "```"} {
		if strings.Contains(visible, raw) {
			t.Errorf("rendered Markdown retained raw syntax %q: %q", raw, visible)
		}
	}
}

func TestRenderMarkdownRespectsDisplayWidth(t *testing.T) {
	for _, width := range []int{1, 4, 12, 24} {
		lines := renderMarkdown("## 日本語 **wide text** and [link](https://example.com)\n\n> a quoted line", width)
		for index, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("width %d line %d has display width %d: %q", width, index, got, line)
			}
		}
	}
}

func TestDetailsPanelRendersDescriptionMarkdown(t *testing.T) {
	dashboard := editorDashboard(t)
	dashboard.Issues[0].Description = "# Markdown heading\n\nA **formatted phrase**."
	m := NewWithDashboard(dashboard)
	view := m.renderDetailsPanel(80, 30)
	for _, want := range []string{"Markdown", "heading", "formatted", "phrase"} {
		if !strings.Contains(view, want) {
			t.Errorf("details panel missing rendered text %q", want)
		}
	}
	for _, raw := range []string{"# Markdown heading", "**formatted phrase**"} {
		if strings.Contains(view, raw) {
			t.Errorf("details panel retained Markdown syntax %q", raw)
		}
	}
}
