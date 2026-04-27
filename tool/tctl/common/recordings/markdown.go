/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordings

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var orderedListRE = regexp.MustCompile(`^\d+\.\s+`)

func renderMarkdownForTerminal(markdown string, width int, p palette) string {
	markdown = sanitize(markdown)
	if width < 20 {
		width = 20
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(p.section)
	subtitleStyle := lipgloss.NewStyle().Bold(true).Foreground(p.accent)
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.AdaptiveColor{Light: "236", Dark: "236"}).
		Padding(0, 1)
	quoteStyle := lipgloss.NewStyle().Foreground(p.faint).Italic(true)

	var out []string
	var paragraph []string
	var codeLines []string
	inCode := false

	flushParagraph := func(prefix string) {
		if len(paragraph) == 0 {
			return
		}
		text := strings.Join(paragraph, " ")
		wrapped := wrapText(text, width)
		// Apply inline formatting line-by-line after wrapping so ANSI codes don't affect wrap widths.
		lines := strings.Split(wrapped, "\n")
		for j, line := range lines {
			lines[j] = renderInline(line, p)
		}
		wrapped = strings.Join(lines, "\n")
		if prefix != "" {
			prefixedLines := strings.Split(wrapped, "\n")
			for j, line := range prefixedLines {
				if j == 0 {
					prefixedLines[j] = prefix + line
				} else {
					prefixedLines[j] = strings.Repeat(" ", len(prefix)) + line
				}
			}
			wrapped = strings.Join(prefixedLines, "\n")
		}
		out = append(out, wrapped)
		paragraph = nil
	}

	flushCode := func() {
		if len(codeLines) == 0 {
			return
		}
		blockWidth := width
		longest := 0
		for _, line := range codeLines {
			if len(line) > longest {
				longest = len(line)
			}
		}
		if longest+2 < blockWidth {
			blockWidth = longest + 2
		}
		for _, line := range codeLines {
			out = append(out, codeStyle.Width(blockWidth).Render(line))
		}
		codeLines = nil
	}

	for _, rawLine := range strings.Split(markdown, "\n") {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph("")
			if inCode {
				flushCode()
			}
			inCode = !inCode
			continue
		}

		if inCode {
			codeLines = append(codeLines, line)
			continue
		}

		if trimmed == "" {
			flushParagraph("")
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			flushParagraph("")
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			text := strings.TrimSpace(trimmed[level:])
			switch level {
			case 1:
				out = append(out, titleStyle.Render(text))
			default:
				out = append(out, subtitleStyle.Render(text))
			}
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph("")
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			out = append(out, quoteStyle.Render(wrapText(renderInline(text, p), width-2)))
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			flushParagraph("")
			out = append(out, formatBulletLine(trimmed[2:], width, "• ", p))
			continue
		}

		if orderedListRE.MatchString(trimmed) {
			flushParagraph("")
			match := orderedListRE.FindString(trimmed)
			out = append(out, formatBulletLine(strings.TrimPrefix(trimmed, match), width, match, p))
			continue
		}

		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			flushParagraph("")
			out = append(out, subtitleStyle.Render(trimmed))
			continue
		}

		paragraph = append(paragraph, trimmed)
	}

	flushParagraph("")
	flushCode()

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func formatBulletLine(text string, width int, prefix string, p palette) string {
	if width <= len(prefix) {
		return prefix + text
	}
	lines := strings.Split(wrapText(text, width-len(prefix)), "\n")
	for i, line := range lines {
		rendered := renderInline(line, p)
		if i == 0 {
			lines[i] = prefix + rendered
		} else {
			lines[i] = strings.Repeat(" ", len(prefix)) + rendered
		}
	}
	return strings.Join(lines, "\n")
}

// renderInline applies inline span formatting to a single line of text.
// Supported: **bold** / __bold__, *italic* / _italic_, `code`.
// Markers must be balanced on the same line; no nesting; first match wins left-to-right.
func renderInline(s string, _ palette) string {
	boldStyle := lipgloss.NewStyle().Bold(true)
	italicStyle := lipgloss.NewStyle().Italic(true)
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.AdaptiveColor{Light: "236", Dark: "236"})

	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '`' {
			if end := strings.Index(s[i+1:], "`"); end >= 0 {
				result.WriteString(codeStyle.Render(s[i+1 : i+1+end]))
				i += end + 2
				continue
			}
		} else if i+1 < len(s) && (s[i:i+2] == "**" || s[i:i+2] == "__") {
			marker := s[i : i+2]
			if end := strings.Index(s[i+2:], marker); end >= 0 {
				result.WriteString(boldStyle.Render(s[i+2 : i+2+end]))
				i += end + 4
				continue
			}
		} else if s[i] == '*' || s[i] == '_' {
			marker := s[i : i+1]
			if end := strings.Index(s[i+1:], marker); end >= 0 {
				result.WriteString(italicStyle.Render(s[i+1 : i+1+end]))
				i += end + 2
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// wrapText breaks text into lines not exceeding maxWidth characters.
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	var result strings.Builder
	lineLen := 0
	for i, word := range strings.Fields(text) {
		if i == 0 {
			// no leading space or newline before the first word
		} else if lineLen+1+len(word) > maxWidth {
			result.WriteString("\n")
			lineLen = 0
		} else {
			result.WriteByte(' ')
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}
	return result.String()
}
