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
	"testing"
)

// ansiRE strips ANSI escape codes so tests compare visible text only.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestRenderInline(t *testing.T) {
	p := buildPalette()

	for _, tt := range []struct {
		name     string
		input    string
		expected string
	}{
		{"plain", "plain text", "plain text"},
		{"bold_star", "**bold** word", "bold word"},
		{"bold_under", "__bold__ word", "bold word"},
		{"italic_star", "*italic* word", "italic word"},
		{"italic_under", "_italic_ word", "italic word"},
		{"code", "`code` word", "code word"},
		{"unbalanced_bold", "no **closing", "no **closing"},
		{"unbalanced_italic", "no *closing", "no *closing"},
		{"unbalanced_code", "no `closing", "no `closing"},
		{"mixed", "**bold** and *italic* and `code`", "bold and italic and code"},
		{"empty", "", ""},
		{"code_beats_star", "`*not italic*`", "*not italic*"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(renderInline(tt.input, p))
			if got != tt.expected {
				t.Errorf("renderInline(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
