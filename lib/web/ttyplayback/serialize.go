/**
 * Copyright (C) 2024 Gravitational, Inc.
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

package ttyplayback

import (
	"fmt"
	"strings"

	"github.com/hinshun/vt10x"
)

type TerminalState struct {
	Cols   int        `json:"cols"`
	Rows   int        `json:"rows"`
	Cursor CursorInfo `json:"cursor"`
	Lines  []LineInfo `json:"lines"`
	Dump   string     `json:"dump"`
}

type CursorInfo struct {
	X     int   `json:"x"`
	Y     int   `json:"y"`
	Glyph Glyph `json:"glyph"`
	State uint8 `json:"state"`
}

type LineInfo struct {
	Cells []CellInfo `json:"cells"`
}

type Glyph struct {
	Char       rune   `json:"char"`
	Mode       int16  `json:"mode"`
	Foreground string `json:"fg,omitempty"`
	Background string `json:"bg,omitempty"`
}

type CellInfo struct {
	Char       rune   `json:"char"`
	Mode       int16  `json:"mode"`
	Foreground string `json:"fg,omitempty"`
	Background string `json:"bg,omitempty"`
	Bold       bool   `json:"bold,omitempty"`
	Dim        bool   `json:"dim,omitempty"`
	Italic     bool   `json:"italic,omitempty"`
	Underline  bool   `json:"underline,omitempty"`
	Blink      bool   `json:"blink,omitempty"`
	Reverse    bool   `json:"reverse,omitempty"`
}

func SerializeTerminal(vt vt10x.Terminal, theme *Theme) TerminalState {
	cols, rows := vt.Size()
	cursor := vt.Cursor()

	state := TerminalState{
		Cols: cols,
		Rows: rows,
		Cursor: CursorInfo{
			X: cursor.X,
			Y: cursor.Y,
			Glyph: Glyph{
				Char:       cursor.Attr.Char,
				Mode:       cursor.Attr.Mode,
				Foreground: colorToHex(cursor.Attr.FG, theme),
				Background: colorToHex(cursor.Attr.BG, theme),
			},
			State: cursor.State,
		},
		Lines: make([]LineInfo, rows),
	}

	for y := 0; y < rows; y++ {
		line := LineInfo{
			Cells: make([]CellInfo, cols),
		}

		for x := 0; x < cols; x++ {
			cell := vt.Cell(x, y)
			cellInfo := CellInfo{
				Char:       cell.Char,
				Mode:       cell.Mode,
				Foreground: colorToHex(cell.FG, theme),
				Background: colorToHex(cell.BG, theme),
			}

			attrs := getTextAttrs(&cell, vt.Cursor(), x, y, theme)

			if cell.FG != vt10x.DefaultFG {
				cellInfo.Foreground = colorToHex(cell.FG, theme)
			}
			if cell.BG != vt10x.DefaultBG {
				cellInfo.Background = colorToHex(cell.BG, theme)
			}

			cellInfo.Bold = attrs.bold
			cellInfo.Dim = attrs.faint
			cellInfo.Italic = attrs.italic
			cellInfo.Underline = attrs.underline
			cellInfo.Blink = attrs.blink
			cellInfo.Reverse = attrs.reverse

			line.Cells[x] = cellInfo
		}

		state.Lines[y] = line
	}

	state.Dump = generateDumpSequence(vt, theme)

	return state
}

func generateDumpSequence(vt vt10x.Terminal, theme *Theme) string {
	var seq strings.Builder

	cols, rows := vt.Size()
	cursor := vt.Cursor()

	seq.WriteString("\033[H\033[2J")

	for y := 0; y < rows; y++ {
		seq.WriteString(fmt.Sprintf("\033[%d;1H", y+1))

		var lastAttrs textAttrs
		var lastFG, lastBG vt10x.Color = vt10x.DefaultFG, vt10x.DefaultBG

		for x := 0; x < cols; x++ {
			cell := vt.Cell(x, y)

			attrs := getTextAttrs(&cell, cursor, x, y, theme)

			if !attrsAreEqual(&lastAttrs, &attrs) || cell.FG != lastFG || cell.BG != lastBG {
				seq.WriteString("\033[0m")

				if attrs.bold {
					seq.WriteString("\033[1m")
				}
				if attrs.faint {
					seq.WriteString("\033[2m")
				}
				if attrs.italic {
					seq.WriteString("\033[3m")
				}
				if attrs.underline {
					seq.WriteString("\033[4m")
				}
				if attrs.blink {
					seq.WriteString("\033[5m")
				}
				if attrs.reverse {
					seq.WriteString("\033[7m")
				}

				if fg := colorToANSI(cell.FG, true); fg != "" {
					seq.WriteString(fg)
				}
				if bg := colorToANSI(cell.BG, false); bg != "" {
					seq.WriteString(bg)
				}

				lastAttrs = attrs
				lastFG = cell.FG
				lastBG = cell.BG
			}

			if cell.Char != 0 {
				seq.WriteRune(cell.Char)
			} else {
				seq.WriteRune(' ')
			}
		}
	}

	seq.WriteString(fmt.Sprintf("\033[%d;%dH", vt.Cursor().Y+1, vt.Cursor().X+1))

	if !vt.CursorVisible() {
		seq.WriteString("\033[?25l")
	}

	return seq.String()
}

func colorToANSI(c vt10x.Color, isForeground bool) string {
	base := 30
	if !isForeground {
		base = 40
	}

	if c < 8 {
		return fmt.Sprintf("\033[%dm", base+int(c))
	} else if c < 16 {
		return fmt.Sprintf("\033[%d;1m", base+int(c)-8)
	} else {
		if isForeground {
			return fmt.Sprintf("\033[38;5;%dm", c)
		}
		return fmt.Sprintf("\033[48;5;%dm", c)
	}
}
