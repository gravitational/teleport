// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package terminal

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// StripTerminalControlSequences removes terminal control sequences from s,
// preserving printable text, newlines, and tabs.
func StripTerminalControlSequences(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			c := s[i]
			switch {
			case c == '\x1b':
				i = consumeEscapeSequence(s, i+1)
			case c == '\x90' || c == '\x98' || c == '\x9d' || c == '\x9e' || c == '\x9f':
				i = consumeStringControl(s, i+1)
			case c == '\x9b':
				i = consumeCSISequence(s, i+1)
			case c == '\n' || c == '\t':
				b.WriteByte(c)
				i++
			case c < ' ' || c == '\x7f' || (c >= '\x80' && c <= '\x9f'):
				i++
			default:
				b.WriteByte(c)
				i++
			}
			continue
		}

		switch r {
		case '\x1b':
			i = consumeEscapeSequence(s, i+size)
			continue
		case '\u0090', '\u0098', '\u009d', '\u009e', '\u009f':
			i = consumeStringControl(s, i+size)
			continue
		case '\u009b':
			i = consumeCSISequence(s, i+size)
			continue
		}

		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			i += size
			continue
		}

		b.WriteString(s[i : i+size])
		i += size
	}

	return b.String()
}

func consumeEscapeSequence(s string, i int) int {
	if i >= len(s) {
		return i
	}

	switch s[i] {
	case '[':
		return consumeCSISequence(s, i+1)
	case ']', 'P', 'X', '^', '_':
		return consumeStringControl(s, i+1)
	case '(', ')', '*', '+', '-', '.', '/', '#':
		if i+1 < len(s) {
			return i + 2
		}
		return i + 1
	}

	for i < len(s) {
		c := s[i]
		i++
		if c >= '\x30' && c <= '\x7e' {
			return i
		}
		if c < ' ' || c > '\x7e' {
			return i
		}
	}
	return i
}

func consumeCSISequence(s string, i int) int {
	for i < len(s) {
		c := s[i]
		i++
		if c >= '\x40' && c <= '\x7e' {
			return i
		}
	}
	return i
}

func consumeStringControl(s string, i int) int {
	for i < len(s) {
		if s[i] == '\a' {
			return i + 1
		}
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			if s[i] == '\x9c' {
				return i + 1
			}
			i++
			continue
		}
		if r == '\u009c' {
			return i + size
		}
		i += size
	}
	return i
}
