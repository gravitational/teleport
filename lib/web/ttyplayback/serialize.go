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

type SerializedTerminal struct {
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	CursorX int    `json:"cursorX"`
	CursorY int    `json:"cursorY"`
	Data    string `json:"data"`
}

const (
	attrReverse = 1 << iota
	attrUnderline
	attrBold
	attrGfx
	attrItalic
	attrBlink
	attrWrap
)

func SerializeTerminal(vt vt10x.Terminal) SerializedTerminal {
	cols, rows := vt.Size()
	data := dumpTerminalWithANSI(vt)

	return SerializedTerminal{
		Cols: cols,
		Rows: rows,
		Data: data,
	}
}
func dumpToAnsi(cols, rows, cursorX, cursorY int, cursorVisible bool, lines [][]vt10x.Glyph) string {
	var output strings.Builder
	lastFG := vt10x.Color(^uint32(0))
	lastBG := vt10x.Color(^uint32(0))
	var lastMode int16 = -1

	output.WriteString("\x1b[H\x1b[2J")
	output.WriteString("\x1b[0m")

	//if cursorVisible {
	output.WriteString("\x1b[?25h")
	//} else {
	//	output.WriteString("\x1b[?25l")
	//}

	for y := 0; y < rows; y++ {
		if y > 0 {
			output.WriteString("\r\n")
		}

		for x := 0; x < cols; x++ {
			glyph := lines[y][x]
			var codes []string

			fg := glyph.FG
			bg := glyph.BG
			if glyph.Mode&attrReverse != 0 {
				fg, bg = bg, fg
			}

			if glyph.Mode != lastMode {
				if lastMode != -1 {
					codes = append(codes, "0")
					lastFG = vt10x.Color(^uint32(0))
					lastBG = vt10x.Color(^uint32(0))
				}

				if glyph.Mode&attrBold != 0 {
					codes = append(codes, "1")
				}
				if glyph.Mode&attrUnderline != 0 {
					codes = append(codes, "4")
				}
				if glyph.Mode&attrItalic != 0 {
					codes = append(codes, "3")
				}
				if glyph.Mode&attrBlink != 0 {
					codes = append(codes, "5")
				}
				lastMode = glyph.Mode
			}

			if fg != lastFG {
				if fg == vt10x.DefaultFG {
					codes = append(codes, "39")
				} else if fg < 16 {
					if fg < 8 {
						codes = append(codes, fmt.Sprintf("%d", 30+fg))
					} else {
						codes = append(codes, fmt.Sprintf("%d", 90+fg-8))
					}
				} else if fg < 256 {
					codes = append(codes, "38", "5", fmt.Sprintf("%d", fg))
				} else {
					r := (fg >> 16) & 0xFF
					g := (fg >> 8) & 0xFF
					b := fg & 0xFF
					codes = append(codes, "38", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
				}
				lastFG = fg
			}

			if bg != lastBG {
				if bg == vt10x.DefaultBG {
					codes = append(codes, "49")
				} else if bg < 16 {
					if bg < 8 {
						codes = append(codes, fmt.Sprintf("%d", 40+bg))
					} else {
						codes = append(codes, fmt.Sprintf("%d", 100+bg-8))
					}
				} else if bg < 256 {
					codes = append(codes, "48", "5", fmt.Sprintf("%d", bg))
				} else {
					r := (bg >> 16) & 0xFF
					g := (bg >> 8) & 0xFF
					b := bg & 0xFF
					codes = append(codes, "48", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
				}
				lastBG = bg
			}

			if len(codes) > 0 {
				output.WriteString("\x1b[")
				output.WriteString(strings.Join(codes, ";"))
				output.WriteString("m")
			}

			if glyph.Char == 0 {
				output.WriteRune(' ')
			} else {
				output.WriteRune(glyph.Char)
			}
		}
	}

	output.WriteString("\x1b[0m")
	output.WriteString(fmt.Sprintf("\x1b[%d;%dH", cursorY+1, cursorX+1))

	return output.String()
}

func dumpTerminalWithANSI(vt vt10x.Terminal) string {
	cursor := vt.Cursor()
	cols, rows := vt.Size()

	lines := make([][]vt10x.Glyph, rows)

	for row := 0; row < rows; row++ {
		lines[row] = make([]vt10x.Glyph, cols)
		for col := 0; col < cols; col++ {
			cell := vt.Cell(col, row)
			lines[row][col] = cell
		}
	}

	return dumpToAnsi(
		cols,
		rows,
		cursor.X,
		cursor.Y,
		vt.CursorVisible(),
		lines,
	)
}
