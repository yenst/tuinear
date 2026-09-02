package ui

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// markdownSpan is a piece of visible Markdown text with the style that should
// be applied to it. Keeping the text and style separate means wrapping can be
// done before ANSI escape sequences are added to the output.
type markdownSpan struct {
	text  string
	style lipgloss.Style
}

var (
	markdownHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.accent)
	markdownStrongStyle  = lipgloss.NewStyle().Bold(true)
	markdownEmStyle      = lipgloss.NewStyle().Italic(true)
	markdownStrikeStyle  = lipgloss.NewStyle().Strikethrough(true)
	markdownCodeStyle    = lipgloss.NewStyle().Foreground(theme.yellow)
	markdownQuoteStyle   = lipgloss.NewStyle().Foreground(theme.muted)
	// A color cue keeps link labels readable while avoiding per-rune ANSI
	// sequences produced by lipgloss's underline renderer.
	markdownLinkStyle = lipgloss.NewStyle().Foreground(theme.accent)
)

var (
	markdownHeadingPattern   = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)
	markdownUnorderedPattern = regexp.MustCompile(`^( *)([-+*])[ \t]+(.*)$`)
	markdownOrderedPattern   = regexp.MustCompile(`^( *)([0-9]+\.)[ \t]+(.*)$`)
	markdownFencePattern     = regexp.MustCompile(`^ {0,3}(` + "```" + `|~~~)[ \t]*(.*)$`)
	markdownRulePattern      = regexp.MustCompile(`^ {0,3}((\*|_|-)[ \t]*){3,}$`)
)

// renderMarkdown turns the stored Markdown source into terminal-ready lines.
// It intentionally covers the Markdown constructs useful in an issue
// description while remaining small and deterministic for a TUI renderer.
func renderMarkdown(source string, width int) []string {
	width = max(1, width)
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	if strings.TrimSpace(source) == "" {
		return nil
	}

	var lines []string
	inFence := false
	var fence string
	for _, sourceLine := range strings.Split(source, "\n") {
		if match := markdownFencePattern.FindStringSubmatch(sourceLine); match != nil {
			marker := match[1]
			if !inFence {
				inFence, fence = true, marker
				continue
			}
			if marker == fence {
				inFence, fence = false, ""
				continue
			}
		}
		if inFence {
			lines = append(lines, wrapMarkdownSpans([]markdownSpan{{text: sourceLine, style: markdownCodeStyle}}, width)...)
			continue
		}
		lines = append(lines, renderMarkdownBlock(sourceLine, width)...)
	}
	return lines
}

func renderMarkdownBlock(sourceLine string, width int) []string {
	if strings.TrimSpace(sourceLine) == "" {
		return []string{""}
	}
	if markdownRulePattern.MatchString(sourceLine) {
		return []string{strings.Repeat("─", width)}
	}
	if match := markdownHeadingPattern.FindStringSubmatch(sourceLine); match != nil {
		return wrapMarkdownSpans(inlineMarkdown(match[2], markdownHeadingStyle), width)
	}
	if match := markdownUnorderedPattern.FindStringSubmatch(sourceLine); match != nil {
		prefix := match[1] + "• "
		return wrapMarkdownSpans(append([]markdownSpan{{text: prefix}}, inlineMarkdown(match[3], lipgloss.NewStyle())...), width)
	}
	if match := markdownOrderedPattern.FindStringSubmatch(sourceLine); match != nil {
		prefix := match[1] + match[2] + " "
		return wrapMarkdownSpans(append([]markdownSpan{{text: prefix}}, inlineMarkdown(match[3], lipgloss.NewStyle())...), width)
	}
	if strings.HasPrefix(strings.TrimLeft(sourceLine, " "), ">") {
		trimmed := strings.TrimLeft(sourceLine, " ")
		content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		spans := []markdownSpan{{text: "│ ", style: markdownQuoteStyle}}
		spans = append(spans, inlineMarkdown(content, lipgloss.NewStyle())...)
		return wrapMarkdownSpans(spans, width)
	}
	return wrapMarkdownSpans(inlineMarkdown(sourceLine, lipgloss.NewStyle()), width)
}

// inlineMarkdown removes inline Markdown delimiters and assigns styles to the
// visible portions. It is deliberately conservative: text it does not
// recognize is left visible rather than being accidentally discarded.
func inlineMarkdown(source string, defaultStyle lipgloss.Style) []markdownSpan {
	var spans []markdownSpan
	flush := func(text string, style lipgloss.Style) {
		if text != "" {
			spans = append(spans, markdownSpan{text: text, style: style})
		}
	}
	plain := strings.Builder{}
	flushPlain := func() {
		flush(plain.String(), defaultStyle)
		plain.Reset()
	}
	for index := 0; index < len(source); {
		if source[index] == '\\' && index+1 < len(source) && strings.ContainsRune(`\\`+"`*_{}[]()#+-.!>~|", rune(source[index+1])) {
			plain.WriteByte(source[index+1])
			index += 2
			continue
		}
		if source[index] == '`' {
			end := strings.IndexByte(source[index+1:], '`')
			if end >= 0 {
				flushPlain()
				flush(source[index+1:index+1+end], markdownCodeStyle)
				index += end + 2
				continue
			}
		}
		matched := false
		for _, marker := range []struct {
			open, close string
			style       lipgloss.Style
		}{
			{open: "**", close: "**", style: markdownStrongStyle},
			{open: "__", close: "__", style: markdownStrongStyle},
			{open: "~~", close: "~~", style: markdownStrikeStyle},
			{open: "*", close: "*", style: markdownEmStyle},
			{open: "_", close: "_", style: markdownEmStyle},
		} {
			if !strings.HasPrefix(source[index:], marker.open) {
				continue
			}
			end := strings.Index(source[index+len(marker.open):], marker.close)
			if end < 0 || end == 0 {
				continue
			}
			flushPlain()
			flush(source[index+len(marker.open):index+len(marker.open)+end], marker.style)
			index += len(marker.open) + end + len(marker.close)
			matched = true
			break
		}
		if matched {
			continue
		}
		if source[index] == '[' {
			closeLabel := strings.IndexByte(source[index+1:], ']')
			if closeLabel >= 0 {
				labelEnd := index + 1 + closeLabel
				if labelEnd+1 < len(source) && source[labelEnd+1] == '(' {
					closeURL := strings.IndexByte(source[labelEnd+2:], ')')
					if closeURL >= 0 {
						flushPlain()
						label := source[index+1 : labelEnd]
						url := source[labelEnd+2 : labelEnd+2+closeURL]
						flush(label, markdownLinkStyle)
						if url != "" {
							flush(" ("+url+")", markdownQuoteStyle)
						}
						index = labelEnd + 3 + closeURL
						continue
					}
				}
			}
		}
		if strings.HasPrefix(source[index:], "![") {
			// An image is represented by its alt text in a terminal.
			closeLabel := strings.IndexByte(source[index+2:], ']')
			if closeLabel >= 0 {
				labelEnd := index + 2 + closeLabel
				if labelEnd+1 < len(source) && source[labelEnd+1] == '(' {
					closeURL := strings.IndexByte(source[labelEnd+2:], ')')
					if closeURL >= 0 {
						flushPlain()
						flush(source[index+2:labelEnd], markdownLinkStyle)
						index = labelEnd + 3 + closeURL
						continue
					}
				}
			}
		}
		plain.WriteByte(source[index])
		index++
	}
	flushPlain()
	return spans
}

func wrapMarkdownSpans(spans []markdownSpan, width int) []string {
	width = max(1, width)
	if len(spans) == 0 {
		return []string{""}
	}
	lines := make([][]markdownSpan, 1)
	lineWidth := 0
	for _, span := range spans {
		for _, token := range markdownTokens(span.text) {
			if token == "" {
				continue
			}
			isSpace := strings.TrimFunc(token, unicode.IsSpace) == ""
			tokenWidth := lipgloss.Width(token)
			if isSpace {
				if lineWidth > 0 {
					lines[len(lines)-1] = append(lines[len(lines)-1], markdownSpan{text: token, style: span.style})
					lineWidth += tokenWidth
				}
				continue
			}
			if lineWidth > 0 && lineWidth+tokenWidth > width {
				trimMarkdownLineEnd(&lines[len(lines)-1])
				lines = append(lines, nil)
				lineWidth = 0
			}
			for tokenWidth > width {
				part, rest := splitDisplayWidth(token, width)
				if part == "" {
					// A terminal cell cannot display a wide rune in a one-cell
					// viewport. Use the same single-cell ellipsis convention as
					// clip so this loop always makes progress.
					_, size := utf8.DecodeRuneInString(token)
					part, rest = "…", token[size:]
				}
				lines[len(lines)-1] = append(lines[len(lines)-1], markdownSpan{text: part, style: span.style})
				lines = append(lines, nil)
				lineWidth = 0
				token = rest
				tokenWidth = lipgloss.Width(token)
			}
			if token != "" {
				lines[len(lines)-1] = append(lines[len(lines)-1], markdownSpan{text: token, style: span.style})
				lineWidth += tokenWidth
			}
		}
	}
	trimMarkdownLineEnd(&lines[len(lines)-1])
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		var rendered strings.Builder
		for _, span := range line {
			rendered.WriteString(span.style.Render(span.text))
		}
		result = append(result, rendered.String())
	}
	return result
}

func markdownTokens(value string) []string {
	var tokens []string
	for len(value) > 0 {
		_, size := utf8.DecodeRuneInString(value)
		space := unicode.IsSpace([]rune(value[:size])[0])
		end := size
		for end < len(value) {
			r, next := utf8.DecodeRuneInString(value[end:])
			if unicode.IsSpace(r) != space {
				break
			}
			end += next
		}
		tokens = append(tokens, value[:end])
		value = value[end:]
	}
	return tokens
}

func trimMarkdownLineEnd(line *[]markdownSpan) {
	spans := *line
	for len(spans) > 0 {
		last := &spans[len(spans)-1]
		trimmed := strings.TrimRightFunc(last.text, unicode.IsSpace)
		if trimmed == last.text {
			break
		}
		if trimmed == "" {
			spans = spans[:len(spans)-1]
			continue
		}
		last.text = trimmed
		break
	}
	*line = spans
}

func splitDisplayWidth(value string, width int) (string, string) {
	if width <= 0 {
		return "", value
	}
	used := 0
	for index, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if used+runeWidth > width {
			return value[:index], value[index:]
		}
		used += runeWidth
	}
	return value, ""
}
