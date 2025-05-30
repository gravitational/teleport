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

	state.Dump = dumpTerminalWithANSI(vt)

	return state
}

func dumpTerminalWithANSI(vt vt10x.Terminal) string {
	cols, rows := vt.Size()
	var output strings.Builder
	var lastFG, lastBG vt10x.Color
	var lastMode int16
	needReset := true

	cursor := vt.Cursor()
	output.WriteString("\x1b[s")

	if vt.CursorVisible() {
		output.WriteString("\x1b[?25h")
	} else {
		output.WriteString("\x1b[?25l")
	}

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			glyph := vt.Cell(x, y)

			if glyph.Mode != lastMode || glyph.FG != lastFG || glyph.BG != lastBG || needReset {
				var codes []string

				// Reset all attributes if mode changes or at start
				if (glyph.Mode != lastMode && lastMode != 0) || needReset {
					codes = append(codes, "0")
					lastMode = 0
					lastFG = vt10x.DefaultFG
					lastBG = vt10x.DefaultBG
				}

				// Apply mode attributes
				if glyph.Mode != lastMode {
					if glyph.Mode&(1<<0) != 0 {
						codes = append(codes, "1")
					} // Bold
					if glyph.Mode&(1<<1) != 0 {
						codes = append(codes, "2")
					} // Dim
					if glyph.Mode&(1<<2) != 0 {
						codes = append(codes, "4")
					} // Underline
					if glyph.Mode&(1<<3) != 0 {
						codes = append(codes, "5")
					} // Blink
					if glyph.Mode&(1<<4) != 0 {
						codes = append(codes, "7")
					} // Reverse
					if glyph.Mode&(1<<5) != 0 {
						codes = append(codes, "8")
					} // Hidden
					lastMode = glyph.Mode
				}

				// Handle foreground color
				if glyph.FG != lastFG {
					if glyph.FG == vt10x.DefaultFG {
						codes = append(codes, "39") // Default foreground
					} else if glyph.FG < 16 {
						if glyph.FG < 8 {
							codes = append(codes, fmt.Sprintf("%d", 30+glyph.FG))
						} else {
							codes = append(codes, fmt.Sprintf("%d", 90+glyph.FG-8))
						}
					} else if glyph.FG < 256 {
						codes = append(codes, "38", "5", fmt.Sprintf("%d", glyph.FG))
					} else {
						r := (glyph.FG >> 16) & 0xFF
						g := (glyph.FG >> 8) & 0xFF
						b := glyph.FG & 0xFF
						codes = append(codes, "38", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
					}
					lastFG = glyph.FG
				}

				// Handle background color
				if glyph.BG != lastBG {
					if glyph.BG == vt10x.DefaultBG {
						codes = append(codes, "49") // Default background
					} else if glyph.BG < 16 {
						if glyph.BG < 8 {
							codes = append(codes, fmt.Sprintf("%d", 40+glyph.BG))
						} else {
							codes = append(codes, fmt.Sprintf("%d", 100+glyph.BG-8))
						}
					} else if glyph.BG < 256 {
						codes = append(codes, "48", "5", fmt.Sprintf("%d", glyph.BG))
					} else {
						r := (glyph.BG >> 16) & 0xFF
						g := (glyph.BG >> 8) & 0xFF
						b := glyph.BG & 0xFF
						codes = append(codes, "48", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b))
					}
					lastBG = glyph.BG
				}

				if len(codes) > 0 {
					output.WriteString("\x1b[")
					output.WriteString(strings.Join(codes, ";"))
					output.WriteString("m")
				}
				needReset = false
			}

			if glyph.Char == 0 {
				output.WriteRune(' ')
			} else {
				output.WriteRune(glyph.Char)
			}
		}

		// Reset at end of each line to ensure clean line breaks
		if y < rows-1 {
			output.WriteString("\x1b[0m\r\n")
			needReset = true
		}
	}

	output.WriteString("\x1b[0m")
	output.WriteString(fmt.Sprintf("\x1b[%d;%dH", cursor.Y+1, cursor.X+1))

	return output.String()
}
