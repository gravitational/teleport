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

	closeHeader := writeSVGHeader(&buf, pixelWidth, pixelHeight)

	var cursor *cursorPos
	if state.CursorVisible {
		cursor = &cursorPos{x: state.CursorX, y: state.CursorY}
	}

	renderBackgrounds(&buf, state.PrimaryBuffer, cols, rows, charWidth, rowHeight, cursor)
	renderText(&buf, state.PrimaryBuffer, cols, rows, charWidth, cursor)

	buf.WriteString(closeHeader())

	return buf.Bytes()
}

// writeSVGHeader writes the SVG header and styles to the buffer.
// It returns a function that closes the SVG tags when called.
func writeSVGHeader(buf *bytes.Buffer, width, height int) func() string {
	x := 1.0 * 100.0 / (float64(width) / fontSize / 0.6)
	y := 0.5 * 100.0 / (float64(height) / fontSize / lineHeight)

	fmt.Fprintf(buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-size="%.0f" class="terminal">`,
		width, height, fontSize)
	fmt.Fprintf(buf, `<rect width="100%%" height="100%%" class="bg-default"/><svg x="%.3f%%" y="%.3f%%">`,
		x, y)

	return func() string {
		return "</svg></svg>"
	}
}

func renderBackgrounds(buf *bytes.Buffer, buffer [][]vt10x.Glyph, cols, rows int, charWidth, rowHeight float64, cursor *cursorPos) {
	type bgRect struct {
		x, y, w float64
		color   vt10x.Color
	}
	var rects []bgRect

	for y := 0; y < len(buffer); y++ {
		if y >= len(buffer) {
			continue
		}

		yPos := 100.0 * float64(y) / (float64(rows) + 1.0)
		var currentRect *bgRect

		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]
			attrs := getTextAttrs(glyph, cursor, x, y)

			if attrs.background == vt10x.DefaultBG {
				currentRect = nil
				continue
			}

			xPos := 100.0 * float64(x) / (float64(cols) + 2.0)

			if currentRect != nil && currentRect.color == attrs.background && currentRect.y == yPos {
				currentRect.w += charWidth
			} else {
				newRect := bgRect{x: xPos, y: yPos, w: charWidth, color: attrs.background}
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

func renderText(buf *bytes.Buffer, buffer [][]vt10x.Glyph, cols, rows int, charWidth float64, cursor *cursorPos) {
	buf.WriteString(`<text>`)

	for y := 0; y < len(buffer); y++ {
		isEmpty := true
		for x := 0; x < len(buffer[y]); x++ {
			isCursor := cursor != nil && cursor.x == x && cursor.y == y
			if (buffer[y][x].Char != 0 && buffer[y][x].Char != ' ') || isCursor {
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
			x     float64
			text  strings.Builder
			attrs textAttrs
		}

		var spans []span
		var currentSpan *span

		var lastX = -1
		for x := 0; x < len(buffer[y]); x++ {
			glyph := buffer[y][x]
			isCursor := cursor != nil && cursor.x == x && cursor.y == y

			if glyph.Char == 0 && !isCursor {
				continue
			}

			attrs := getTextAttrs(glyph, cursor, x, y)

			// Check if we need a new span
			needNewSpan := currentSpan == nil ||
				currentSpan.attrs.foreground != attrs.foreground ||
				currentSpan.attrs.bold != attrs.bold ||
				currentSpan.attrs.italic != attrs.italic ||
				currentSpan.attrs.underline != attrs.underline ||
				(lastX >= 0 && x > lastX+1)

			if needNewSpan {
				xPos := 100.0 * float64(x) / (float64(cols) + 2.0)
				newSpan := span{x: xPos, attrs: attrs}
				spans = append(spans, newSpan)
				currentSpan = &spans[len(spans)-1]
			}

			// Add character to current span
			charToRender := glyph.Char
			if isCursor && charToRender == 0 {
				charToRender = ' '
			}

			if charToRender != ' ' || (lastX >= 0 && x == lastX+1) || isCursor {
				spanPtr := &spans[len(spans)-1]
				switch charToRender {
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
					spanPtr.text.WriteRune(charToRender)
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

			if s.attrs.foreground == vt10x.DefaultFG {
				// No class needed for the default foreground, inherits from parent
			} else if s.attrs.foreground < 16 {
				// Basic 16 ANSI colors - use class for theming
				classes = append(classes, fmt.Sprintf("fg-%d", s.attrs.foreground))
			} else {
				// Extended colors (256 palette or RGB) - use inline style
				style = fmt.Sprintf("fill:%s", colorToHex(s.attrs.foreground))
			}

			if s.attrs.bold {
				classes = append(classes, "b")
			}
			if s.attrs.italic {
				classes = append(classes, "i")
			}
			if s.attrs.underline {
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

type cursorPos struct {
	x, y int
}

type textAttrs struct {
	foreground vt10x.Color
	background vt10x.Color
	bold       bool
	italic     bool
	underline  bool
}

func getTextAttrs(glyph vt10x.Glyph, cursor *cursorPos, x, y int) textAttrs {
	fg := glyph.FG
	bg := glyph.BG
	mode := glyph.Mode

	isCursor := cursor != nil && cursor.x == x && cursor.y == y

	if vt10x.IsBold(mode) && fg < 8 { // Brighten foreground if bold
		fg = fg + 8
	}

	if vt10x.IsBlink(mode) && bg < 8 { // Brighten background if blink
		bg = bg + 8
	}

	// inverse if either reverse mode OR cursor, but not both
	shouldInverse := vt10x.IsReverse(mode) != isCursor

	if shouldInverse {
		newFg := bg
		newBg := fg

		if newFg == vt10x.DefaultBG {
			newFg = vt10x.Color(0)
		}
		if newBg == vt10x.DefaultFG {
			newBg = vt10x.Color(7)
		}

		fg = newFg
		bg = newBg
	}

	return textAttrs{
		foreground: fg,
		background: bg,
		bold:       vt10x.IsBold(mode),
		italic:     vt10x.IsItalic(mode),
		underline:  vt10x.IsUnderline(mode),
	}
}
