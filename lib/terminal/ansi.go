/**
 * Copyright (C) 2025 Gravitational, Inc.
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

package terminal

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hinshun/vt10x"
)

// terminalStateToANSI converts a terminal state to an ANSI escape sequence string,
// including both the primary and alternate screen buffers.
// It handles terminal modes, colors, cursor position, and other settings.
func terminalStateToANSI(state vt10x.TerminalState) string {
	var buf bytes.Buffer

	// Initialize the terminal
	buf.WriteString("\x1b[!p") // Soft reset (DECSTR)
	buf.WriteString("\x1b[2J") // Clear screen
	buf.WriteString("\x1b[H")  // Home cursor

	// Set terminal size
	fmt.Fprintf(&buf, "\x1b[8;%d;%dt", state.Rows, state.Cols)

	if state.Title != "" {
		fmt.Fprintf(&buf, "\x1b]0;%s\x07", state.Title)
	}

	// Apply terminal mode settings
	writeMode(&buf, "?7", state.Wrap)         // Line wrap
	writeMode(&buf, "4", state.Insert)        // Insert mode
	writeMode(&buf, "?6", state.Origin)       // Origin mode
	writeMode(&buf, "?5", state.ReverseVideo) // Reverse video

	// Configure tab stops
	buf.WriteString("\x1b[3g") // Clear all tab stops
	for _, col := range state.TabStops {
		fmt.Fprintf(&buf, "\x1b[%dG", col+1)
		buf.WriteString("\x1bH")
	}

	// Set scroll region (if not default)
	if state.ScrollTop != 0 || state.ScrollBottom != state.Rows-1 {
		fmt.Fprintf(&buf, "\x1b[%d;%dr", state.ScrollTop+1, state.ScrollBottom+1)
	}

	// Handle alternate screen mode
	if state.AltScreen {
		// Render background buffer first (will be hidden by alternate screen)
		renderBuffer(&buf, state.AlternateBuffer)

		// Save the current cursor position
		fmt.Fprintf(&buf, "\x1b[%d;%dH", state.SavedCursorY+1, state.SavedCursorX+1)

		// Switch to alternate screen
		buf.WriteString("\x1b[?1049h")
		buf.WriteString("\x1b[2J") // Clear the alternate screen
		buf.WriteString("\x1b[H")  // Home cursor

		// Render the primary buffer on the alternate screen
		renderBuffer(&buf, state.PrimaryBuffer)
	} else {
		renderBuffer(&buf, state.PrimaryBuffer)
	}

	// Set cursor position and visibility
	fmt.Fprintf(&buf, "\x1b[%d;%dH", state.CursorY+1, state.CursorX+1)
	writeMode(&buf, "?25", state.CursorVisible)

	return buf.String()
}

func writeMode(buf *bytes.Buffer, mode string, enabled bool) {
	if enabled {
		fmt.Fprintf(buf, "\x1b[%sh", mode)
	} else {
		fmt.Fprintf(buf, "\x1b[%sl", mode)
	}
}

func renderBuffer(buf *bytes.Buffer, buffer [][]vt10x.Glyph) {
	buf.WriteString("\x1b[0m") // Reset all attributes

	// Track last rendered attributes to minimize escape sequences
	lastFG := vt10x.DefaultFG
	lastBG := vt10x.DefaultBG

	var lastMode int16 = -1

	for y := 0; y < len(buffer); y++ {
		isEmpty := true
		for x := 0; x < len(buffer[y]); x++ {
			if buffer[y][x].Char != 0 && buffer[y][x].Char != ' ' {
				isEmpty = false
				break
			}
		}

		if isEmpty {
			continue
		}

		fmt.Fprintf(buf, "\x1b[%d;1H", y+1)

		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]
			var codes []string

			// Handle reverse video by swapping colors
			fg, bg := glyph.FG, glyph.BG
			if vt10x.IsReverse(glyph.Mode) {
				fg, bg = bg, fg
			}

			// Rebuild text attributes if mode changed
			if glyph.Mode != lastMode {
				// Only reset if we had attributes before, and they're different now
				// (not just going from no attributes to some attributes)
				if lastMode > 0 && glyph.Mode != lastMode {
					codes = append(codes, "0")
					lastFG = vt10x.DefaultFG
					lastBG = vt10x.DefaultBG
				}

				if vt10x.IsBold(glyph.Mode) {
					codes = append(codes, "1")
				}
				if vt10x.IsUnderline(glyph.Mode) {
					codes = append(codes, "4")
				}
				if vt10x.IsItalic(glyph.Mode) {
					codes = append(codes, "3")
				}
				if vt10x.IsBlink(glyph.Mode) {
					codes = append(codes, "5")
				}
				lastMode = glyph.Mode
			}

			// Update foreground color if changed
			if fg != lastFG {
				// Only emit color codes if it's not the default color
				if fg != vt10x.DefaultFG {
					codes = append(codes, colorToANSI(fg, true)...)
				} else if lastFG != vt10x.DefaultFG && bg == vt10x.DefaultBG {
					// Only emit default foreground if we're not also changing background
					// (if background is changing, we can rely on it being more specific)
					codes = append(codes, colorToANSI(fg, true)...)
				}
				lastFG = fg
			}

			// Update background color if changed
			if bg != lastBG {
				// Only emit color codes if it's not the default color
				if bg != vt10x.DefaultBG {
					codes = append(codes, colorToANSI(bg, false)...)
				} else if lastBG != vt10x.DefaultBG {
					// Changing from non-default to default
					codes = append(codes, colorToANSI(bg, false)...)
				}
				lastBG = bg
			}

			// Emit escape sequence if any attributes changed
			if len(codes) > 0 {
				buf.WriteString("\x1b[")
				buf.WriteString(strings.Join(codes, ";"))
				buf.WriteString("m")
			}

			// Write character (space for null character)
			if glyph.Char == 0 {
				buf.WriteRune(' ')
			} else {
				buf.WriteRune(glyph.Char)
			}
		}
	}

	buf.WriteString("\x1b[0m") // Reset attributes at end
}

func colorToANSI(color vt10x.Color, isForeground bool) []string {
	baseCode := 30
	if !isForeground {
		baseCode = 40
	}

	// Default color
	if (isForeground && color == vt10x.DefaultFG) || (!isForeground && color == vt10x.DefaultBG) {
		return []string{fmt.Sprintf("%d", baseCode+9)}
	}

	// Basic 16 colors
	if color < 16 {
		if color < 8 {
			return []string{fmt.Sprintf("%d", baseCode+int(color))}
		}
		return []string{fmt.Sprintf("%d", baseCode+60+int(color)-8)}
	}

	// 256 color palette
	if color < 256 {
		prefix := "38"
		if !isForeground {
			prefix = "48"
		}
		return []string{prefix, "5", fmt.Sprintf("%d", color)}
	}

	r := (color >> 16) & 0xFF
	g := (color >> 8) & 0xFF
	b := color & 0xFF

	prefix := "38"
	if !isForeground {
		prefix = "48"
	}

	return []string{prefix, "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b)}
}
