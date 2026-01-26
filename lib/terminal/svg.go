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
	charRatio  = 0.602 // Menlo mono font character width/height ratio
)

// VtToSvg converts a terminal state to an SVG representation
func VtToSvg(terminal vt10x.Terminal) []byte {
	var buf bytes.Buffer

	cols, rows := terminal.Size()

	charWidthPx := fontSize * charRatio
	rowHeightPx := fontSize * lineHeight
	pixelWidth := int((float64(cols) + 2.0) * charWidthPx)
	pixelHeight := int((float64(rows) + 1.0) * rowHeightPx)

	closeHeader := writeSVGHeader(&buf, pixelWidth, pixelHeight)

	var cursor *cursorPos
	if terminal.CursorVisible() {
		c := terminal.Cursor()
		cursor = &cursorPos{x: c.X, y: c.Y}
	}

	renderBackgrounds(&buf, terminal, charWidthPx, rowHeightPx, cursor)
	renderText(&buf, terminal, charWidthPx, rowHeightPx, cursor)

	buf.WriteString(closeHeader())

	return buf.Bytes()
}

// writeSVGHeader writes the SVG header and styles to the buffer.
// It returns a function that closes the SVG tags when called.
func writeSVGHeader(buf *bytes.Buffer, width, height int) func() string {
	xOffset := fontSize * 0.6
	yOffset := fontSize * lineHeight * 0.5

	fmt.Fprintf(buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-size="%.0f" class="terminal">`,
		width, height, fontSize)
	fmt.Fprintf(buf, `<rect width="100%%" height="100%%" class="bg-default"/><svg x="%.1f" y="%.1f">`,
		xOffset, yOffset)

	return func() string {
		return "</svg></svg>"
	}
}

func renderBackgrounds(buf *bytes.Buffer, terminal vt10x.Terminal, charWidthPx, rowHeightPx float64, cursor *cursorPos) {
	type bgRect struct {
		x, y, w, h float64
		color      vt10x.Color
	}
	var rects []bgRect

	cols, rows := terminal.Size()
	for y := range rows {
		yPos := float64(y) * rowHeightPx
		var currentRect *bgRect

		for x := range cols {
			glyph := terminal.Cell(x, y)
			attrs := getTextAttrs(glyph, cursor, x, y)

			if attrs.background == vt10x.DefaultBG {
				currentRect = nil
				continue
			}

			xPos := float64(x) * charWidthPx

			if currentRect != nil && currentRect.color == attrs.background && currentRect.y == yPos {
				currentRect.w += charWidthPx
			} else {
				newRect := bgRect{
					x:     xPos,
					y:     yPos,
					w:     charWidthPx,
					h:     rowHeightPx,
					color: attrs.background,
				}
				rects = append(rects, newRect)
				currentRect = &rects[len(rects)-1]
			}
		}
	}

	if len(rects) > 0 {
		buf.WriteString(`<g shape-rendering="crispEdges">`)
		for _, rect := range rects {
			fmt.Fprintf(buf, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f"`,
				rect.x, rect.y, rect.w, rect.h)

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

func renderText(buf *bytes.Buffer, terminal vt10x.Terminal, charWidthPx, rowHeightPx float64, cursor *cursorPos) {
	buf.WriteString(`<text>`)

	cols, rows := terminal.Size()
	for y := range rows {
		// Check if the entire row has any non-space content
		hasContent := false
		for x := range cols {
			glyph := terminal.Cell(x, y)
			isCursor := cursor != nil && cursor.x == x && cursor.y == y
			if (glyph.Char != 0 && glyph.Char != ' ') || isCursor {
				hasContent = true
				break
			}
		}

		if !hasContent {
			continue
		}

		yPos := float64(y) * rowHeightPx
		fmt.Fprintf(buf, `<tspan y="%.1f" dy="1em">`, yPos)

		col := 0
		for col < cols {
			glyph := terminal.Cell(col, y)
			isCursor := cursor != nil && cursor.x == col && cursor.y == y

			// Skip spaces completely (unless it's the cursor position)
			if glyph.Char == ' ' || (glyph.Char == 0 && !isCursor) {
				col++
				continue
			}

			startCol := col
			attrs := getTextAttrs(glyph, cursor, col, y)
			var text strings.Builder

			// Collect consecutive non-space characters with same attributes
			for col < cols {
				currentGlyph := terminal.Cell(col, y)
				currentIsCursor := cursor != nil && cursor.x == col && cursor.y == y

				// Stop if we hit a space or null (unless cursor)
				if currentGlyph.Char == ' ' || (currentGlyph.Char == 0 && !currentIsCursor) {
					break
				}

				// Stop if attributes change
				currentAttrs := getTextAttrs(currentGlyph, cursor, col, y)
				if currentAttrs != attrs {
					break
				}

				// Add the character
				charToRender := currentGlyph.Char
				if currentIsCursor && charToRender == 0 {
					charToRender = ' '
				}

				switch charToRender {
				case '\'':
					text.WriteString("&#39;")
				case '"':
					text.WriteString("&quot;")
				case '&':
					text.WriteString("&amp;")
				case '>':
					text.WriteString("&gt;")
				case '<':
					text.WriteString("&lt;")
				default:
					text.WriteRune(charToRender)
				}

				col++
			}

			xPos := float64(startCol) * charWidthPx
			fmt.Fprintf(buf, `<tspan x="%.1f"`, xPos)

			var classes []string
			var style string

			if attrs.foreground == vt10x.DefaultFG {
				// No class needed for default foreground
			} else if attrs.foreground < 16 {
				classes = append(classes, fmt.Sprintf("fg-%d", attrs.foreground))
			} else {
				style = fmt.Sprintf("fill:%s", colorToHex(attrs.foreground))
			}

			if attrs.bold {
				classes = append(classes, "b")
			}
			if attrs.italic {
				classes = append(classes, "i")
			}
			if attrs.underline {
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
			buf.WriteString(text.String())
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
