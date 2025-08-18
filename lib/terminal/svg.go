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

const (
	fontSize   = 14.0
	lineHeight = 1.2
)

// VtStateToSvg converts a terminal state to an SVG representation
func VtStateToSvg(state *vt10x.TerminalState) []byte {
	var buf bytes.Buffer

	cols, rows := state.Cols, state.Rows
	charWidth := 100.0 / (float64(cols) + 2.0)
	rowHeight := fontSize * lineHeight
	pixelWidth := int((float64(cols) + 2.0) * (fontSize * 0.6))
	pixelHeight := int((float64(rows) + 1.0) * rowHeight)

	writeSVGHeader(&buf, pixelWidth, pixelHeight, fontSize)
	renderBackgrounds(&buf, state.PrimaryBuffer, cols, rows, charWidth, rowHeight)
	renderText(&buf, state.PrimaryBuffer, cols, rows, charWidth)

	buf.WriteString("</svg></svg>")

	return buf.Bytes()
}

func writeSVGHeader(buf *bytes.Buffer, width, height int, fontSize float64) {
	x := 1.0 * 100.0 / (float64(width) / fontSize / 0.6)
	y := 0.5 * 100.0 / (float64(height) / fontSize / lineHeight)

	fmt.Fprintf(buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-size="%.0f" class="terminal">`,
		width, height, fontSize)

	buf.WriteString("<style>")
	buf.WriteString(".b{font-weight:bold}")
	buf.WriteString(".i{font-style:italic}")
	buf.WriteString(".u{text-decoration:underline}")
	buf.WriteString("</style>")

	fmt.Fprintf(buf, `<rect width="100%%" height="100%%" class="bg-default"/><svg x="%.3f%%" y="%.3f%%">`,
		x, y)
}

func renderBackgrounds(buf *bytes.Buffer, buffer [][]vt10x.Glyph, cols, rows int, charWidth, rowHeight float64) {
	type bgRect struct {
		x, y, w float64
		color   vt10x.Color
	}
	var rects []bgRect

	for y := 0; y < len(buffer); y++ {
		if y >= len(buffer) {
			continue
		}

		isEmpty := true
		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]
			if glyph.BG != vt10x.DefaultBG || (glyph.Char != 0 && glyph.Char != ' ') {
				isEmpty = false
				break
			}
		}

		if isEmpty {
			continue
		}

		yPos := 100.0 * float64(y) / (float64(rows) + 1.0)
		var currentRect *bgRect

		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]

			// Handle reverse video by swapping colors
			bg := glyph.BG
			if vt10x.IsReverse(glyph.Mode) {
				bg = glyph.FG
			}

			if bg == vt10x.DefaultBG {
				currentRect = nil
				continue
			}

			xPos := 100.0 * float64(x) / (float64(cols) + 2.0)

			if currentRect != nil && currentRect.color == bg && currentRect.y == yPos {
				currentRect.w += charWidth
			} else {
				newRect := bgRect{x: xPos, y: yPos, w: charWidth, color: bg}
				rects = append(rects, newRect)
				currentRect = &rects[len(rects)-1]
			}
		}
	}

	// Render all background rectangles
	if len(rects) > 0 {
		buf.WriteString(`<g>`)
		for _, rect := range rects {
			fmt.Fprintf(buf, `<rect x="%.3f%%" y="%.3f%%" width="%.3f%%" height="%.3f"`,
				rect.x, rect.y, rect.w, rowHeight)

			// Handle color - use class for 16 colors, inline style for extended colors
			if rect.color < 16 {
				fmt.Fprintf(buf, ` class="bg-%d"`, rect.color)
			} else {
				fmt.Fprintf(buf, ` style="fill:%s"`, colorToHex(rect.color))
			}

			buf.WriteString("/>")
		}
		buf.WriteString("</g>")
	}
}

func renderText(buf *bytes.Buffer, buffer [][]vt10x.Glyph, cols, rows int, charWidth float64) {
	buf.WriteString(`<text>`)

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

		yPos := 100.0 * float64(y) / (float64(rows) + 1.0)
		fmt.Fprintf(buf, `<tspan y="%.3f%%" dy="1em">`, yPos)

		type span struct {
			x    float64
			text strings.Builder
			fg   vt10x.Color
			mode int16
		}

		var spans []span
		var currentSpan *span

		var lastX = -1
		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]

			if glyph.Char == 0 {
				continue
			}

			// Handle reverse video
			fg := glyph.FG
			if vt10x.IsReverse(glyph.Mode) {
				fg = glyph.BG
				if fg == vt10x.DefaultBG {
					fg = vt10x.Color(7)
				}
			}

			// Check if we need a new span
			needNewSpan := currentSpan == nil ||
				currentSpan.fg != fg ||
				currentSpan.mode != glyph.Mode ||
				(lastX >= 0 && x > lastX+1) // Gap in text

			if needNewSpan {
				xPos := 100.0 * float64(x) / (float64(cols) + 2.0)
				newSpan := span{x: xPos, fg: fg, mode: glyph.Mode}
				spans = append(spans, newSpan)
				currentSpan = &spans[len(spans)-1]
			}

			// Add character to current span
			if glyph.Char != ' ' || (lastX >= 0 && x == lastX+1) {
				spanPtr := &spans[len(spans)-1]
				switch glyph.Char {
				case '\'':
					spanPtr.text.WriteString("&#39;")
				case '"':
					spanPtr.text.WriteString("&quot;")
				case '&':
					spanPtr.text.WriteString("&amp;")
				case '>':
					spanPtr.text.WriteString("&gt;")
				case '<':
					spanPtr.text.WriteString("&lt;")
				default:
					spanPtr.text.WriteRune(glyph.Char)
				}
				lastX = x
			}
		}

		for _, s := range spans {
			buf.WriteString(`<tspan x="`)
			fmt.Fprintf(buf, "%.3f%%", s.x)
			buf.WriteString(`"`)

			var classes []string
			var style string

			if s.fg == vt10x.DefaultFG {
				// No class needed for the default foreground, inherits from parent
			} else if s.fg < 16 {
				// Basic 16 ANSI colors - use class for theming
				classes = append(classes, fmt.Sprintf("fg-%d", s.fg))
			} else {
				// Extended colors (256 palette or RGB) - use inline style
				style = fmt.Sprintf("fill:%s", colorToHex(s.fg))
			}

			if vt10x.IsBold(s.mode) {
				classes = append(classes, "b")
			}
			if vt10x.IsItalic(s.mode) {
				classes = append(classes, "i")
			}
			if vt10x.IsUnderline(s.mode) {
				classes = append(classes, "u")
			}

			if len(classes) > 0 {
				buf.WriteString(` class="`)
				buf.WriteString(strings.Join(classes, " "))
				buf.WriteString(`"`)
			}

			if style != "" {
				buf.WriteString(` style="`)
				buf.WriteString(style)
				buf.WriteString(`"`)
			}

			buf.WriteString(`>`)
			buf.WriteString(s.text.String())
			buf.WriteString(`</tspan>`)
		}

		buf.WriteString("</tspan>")
	}

	buf.WriteString("</text>")
}

func colorToHex(color vt10x.Color) string {
	if color < 256 {
		var r, g, b uint8

		if color >= 232 {
			gray := uint8((color-232)*10 + 8)
			r, g, b = gray, gray, gray
		} else if color >= 16 {
			c := color - 16
			r = uint8((c / 36) * 51)
			g = uint8(((c % 36) / 6) * 51)
			b = uint8((c % 6) * 51)
		} else {
			return "#000000"
		}

		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	r := (color >> 16) & 0xFF
	g := (color >> 8) & 0xFF
	b := color & 0xFF

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
