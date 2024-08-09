/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package lib

import "strings"

// MarkdownEscape wraps some text `t` in triple backticks (escaping any backtick
// inside the message), limiting the length of the message to `n` runes (inside
// the single preformatted block). The text is trimmed before escaping.
// Backticks are escaped and thus count as two runes for the purpose of the
// truncation.
func MarkdownEscape(t string, n int) string {
	return markdownEscape(t, n, "```\n", "```")
}

// MarkdownEscapeInLine wraps some text `t` in backticks (escaping any backtick
// inside the message), limiting the length of the message to `n` runes (inside
// the single preformatted block). The text is trimmed before escaping.
// Backticks are escaped and thus count as two runes for the purpose of the
// truncation.
func MarkdownEscapeInLine(t string, n int) string {
	return markdownEscape(t, n, "`", "`")
}

func markdownEscape(t string, n int, startBackticks, endBackticks string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "(empty)"
	}

	var b strings.Builder
	b.WriteString(startBackticks)
	for i, r := range t {
		if i >= n {
			b.WriteString(endBackticks + " (truncated)")
			return b.String()
		}
		b.WriteRune(r)
		if r == '`' {
			// byte order mark, as a zero width no-break space; seems to result
			// in escaped backticks with no spurious characters in the message
			b.WriteRune('\ufeff')
			n--
		}
	}
	b.WriteString(endBackticks)
	return b.String()
}
