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

package discovery

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/term"
)

type textStyle struct {
	enabled    bool
	tableWidth int
}

type keyValue struct {
	Key   string
	Value string
}

type nextAction struct {
	Comment  string
	Commands []string
}

func newTextStyle(w io.Writer) textStyle {
	return textStyle{
		enabled:    colorEnabled(w),
		tableWidth: preferredTableWidth(w),
	}
}

func colorEnabled(w io.Writer) bool {
	if forceColorEnabled() {
		return true
	}
	if !utils.IsTerminal(w) {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb")
}

func forceColorEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("FORCE_COLOR"))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func preferredTableWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	width = width - 2
	if width < 72 {
		return 0
	}
	if width > 140 {
		width = 140
	}
	return width
}

func (s textStyle) wrap(code, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", code, text)
}

func (s textStyle) section(text string) string {
	return s.wrap("1;36", text)
}

func (s textStyle) info(text string) string {
	return s.wrap("36", text)
}

func (s textStyle) warning(text string) string {
	return s.wrap("33", text)
}

func (s textStyle) good(text string) string {
	return s.wrap("32", text)
}

func (s textStyle) bad(text string) string {
	return s.wrap("31", text)
}

func (s textStyle) discoveredCount(value uint64) string {
	if value == 0 {
		return s.warning("0")
	}
	return s.good(strconv.FormatUint(value, 10))
}

func (s textStyle) failedCount(value uint64) string {
	if value > 0 {
		return s.bad(strconv.FormatUint(value, 10))
	}
	return s.good("0")
}

func (s textStyle) pendingCount(value uint64) string {
	if value > 0 {
		return s.warning(strconv.FormatUint(value, 10))
	}
	return s.good("0")
}

func (s textStyle) statusValue(value string) string {
	v := strings.TrimSpace(value)
	switch strings.ToLower(v) {
	case "success", "healthy", "resolved", "tds00i":
		return s.good(value)
	case "failed", "failure", "error", "unhealthy", "tds00w":
		return s.bad(value)
	case "timedout", "timeout", "unknown", "open", "syncing":
		return s.warning(value)
	default:
		return value
	}
}

func renderNextActions(w io.Writer, style textStyle, actions []nextAction) error {
	filtered := make([]nextAction, 0, len(actions))
	for _, action := range actions {
		cleanCommands := make([]string, 0, len(action.Commands))
		for _, command := range action.Commands {
			command = strings.TrimSpace(command)
			if command != "" {
				cleanCommands = append(cleanCommands, command)
			}
		}
		if len(cleanCommands) == 0 {
			continue
		}
		action.Commands = cleanCommands
		filtered = append(filtered, action)
	}
	if len(filtered) == 0 {
		return nil
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Next:"))
	for i, action := range filtered {
		if i > 0 {
			fmt.Fprintln(w, "")
		}
		if comment := strings.TrimSpace(action.Comment); comment != "" {
			fmt.Fprintf(w, "  %s\n", style.info("# "+comment))
		}
		for _, command := range action.Commands {
			fmt.Fprintf(w, "  %s\n", command)
		}
	}
	return nil
}

func renderAlignedKeyValues(w io.Writer, indent string, lines []keyValue) error {
	if len(lines) == 0 {
		return nil
	}
	maxKeyWidth := 0
	for _, line := range lines {
		maxKeyWidth = max(maxKeyWidth, len(line.Key))
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s%-*s: %s\n", indent, maxKeyWidth, line.Key, line.Value); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visualWidth(value string) int {
	clean := ansiEscapePattern.ReplaceAllString(value, "")
	return utf8.RuneCountInString(clean)
}

func padRight(value string, width int) string {
	pad := width - visualWidth(value)
	if pad <= 0 {
		return value
	}
	return value + strings.Repeat(" ", pad)
}

func tableWidthCalc(columns []int) int {
	total := 2 // left and right borders
	for _, col := range columns {
		total += col + 2 // one space left/right padding
	}
	total += len(columns) - 1 // internal separators
	return total
}

func renderTable(w io.Writer, headers []string, rows [][]string, targetWidth int) error {
	if len(headers) == 0 {
		return nil
	}
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = visualWidth(h)
	}

	normalizedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		r := make([]string, cols)
		copy(r, row)
		normalizedRows = append(normalizedRows, r)
		for i := 0; i < cols; i++ {
			widths[i] = max(widths[i], visualWidth(r[i]))
		}
	}

	naturalWidth := tableWidthCalc(widths)
	if targetWidth > naturalWidth {
		extra := targetWidth - naturalWidth
		for i := 0; extra > 0; i = (i + 1) % cols {
			widths[i]++
			extra--
		}
	}

	writeBorder := func(left, sep, right rune) error {
		if _, err := fmt.Fprintf(w, "%c", left); err != nil {
			return trace.Wrap(err)
		}
		for i, width := range widths {
			if _, err := io.WriteString(w, strings.Repeat("─", width+2)); err != nil {
				return trace.Wrap(err)
			}
			if i < len(widths)-1 {
				if _, err := fmt.Fprintf(w, "%c", sep); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		if _, err := fmt.Fprintf(w, "%c\n", right); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	writeRow := func(row []string) error {
		if _, err := fmt.Fprint(w, "│"); err != nil {
			return trace.Wrap(err)
		}
		for i, width := range widths {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if _, err := fmt.Fprintf(w, " %s ", padRight(cell, width)); err != nil {
				return trace.Wrap(err)
			}
			if _, err := fmt.Fprint(w, "│"); err != nil {
				return trace.Wrap(err)
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	if err := writeBorder('┌', '┬', '┐'); err != nil {
		return trace.Wrap(err)
	}
	if err := writeRow(headers); err != nil {
		return trace.Wrap(err)
	}
	if err := writeBorder('├', '┼', '┤'); err != nil {
		return trace.Wrap(err)
	}
	for _, row := range normalizedRows {
		if err := writeRow(row); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := writeBorder('└', '┴', '┘'); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
