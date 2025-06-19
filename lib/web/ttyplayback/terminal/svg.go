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

package terminal

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/web/ttyplayback/vt10x"
)

type RGB8 struct {
	R, G, B uint8
}

type Settings struct {
	Width        int
	Height       int
	FontFamilies []string
	FontSize     int
	LineHeight   float64
	Theme        Theme
}

type ResvgRenderer struct {
	width       int
	height      int
	pixelWidth  int
	pixelHeight int
	charWidth   float64
	rowHeight   float64
	theme       Theme
	colorCache  map[vt10x.Color]string
	fontFamily  string
	fontSize    float64
}

const (
	AttrReverse = 1 << iota
	AttrUnderline
	AttrBold
	AttrGfx
	AttrItalic
	AttrBlink
	AttrWrap
	AttrFaint
)

func colorToRGB(c vt10x.Color, theme *Theme) RGB8 {
	if c < 16 {
		return theme.Colors[c]
	} else if c >= 232 {
		gray := uint8((c-232)*10 + 8)
		return RGB8{gray, gray, gray}
	} else {
		c -= 16
		r := uint8((c / 36) * 51)
		g := uint8(((c % 36) / 6) * 51)
		b := uint8((c % 6) * 51)
		return RGB8{r, g, b}
	}
}

func rgbToHex(c RGB8) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func colorToHex(color vt10x.Color, theme *Theme) string {
	return rgbToHex(colorToRGB(color, theme))
}

type textAttrs struct {
	foreground vt10x.Color
	background vt10x.Color
	bold       bool
	faint      bool
	italic     bool
	underline  bool
	blink      bool
	reverse    bool
}

func attrsAreEqual(a, b *textAttrs) bool {
	return a.foreground == b.foreground &&
		a.background == b.background &&
		a.bold == b.bold &&
		a.faint == b.faint &&
		a.italic == b.italic &&
		a.underline == b.underline &&
		a.blink == b.blink &&
		a.reverse == b.reverse
}

func attrsToClasses(attrs *textAttrs) string {
	var classes []string
	if attrs.bold {
		classes = append(classes, "b")
	}
	if attrs.italic {
		classes = append(classes, "i")
	}
	if attrs.underline {
		classes = append(classes, "u")
	}
	return strings.Join(classes, " ")
}

func NewResvgRenderer(settings Settings) *ResvgRenderer {
	cols, rows := settings.Width, settings.Height
	charWidth := 100.0 / (float64(cols) + 2.0)
	fontSize := float64(settings.FontSize)
	rowHeight := fontSize * settings.LineHeight
	fontFamily := strings.Join(settings.FontFamilies, ",")

	pixelWidth := int((float64(cols) + 2.0) * (fontSize * 0.6))
	pixelHeight := int((float64(rows) + 1.0) * rowHeight)

	r := &ResvgRenderer{
		width:       cols,
		height:      rows,
		pixelWidth:  pixelWidth,
		pixelHeight: pixelHeight,
		charWidth:   charWidth,
		rowHeight:   rowHeight,
		theme:       settings.Theme,
		colorCache:  make(map[vt10x.Color]string),
		fontFamily:  fontFamily,
		fontSize:    fontSize,
	}

	return r
}

func (r *ResvgRenderer) RenderToSVG(lines [][]vt10x.Glyph, cursor vt10x.Cursor) string {
	usedClasses := make(map[string]bool)
	r.scanUsedClasses(lines, cursor, usedClasses)

	var svg strings.Builder
	svg.Grow(r.width * r.height * 50)

	r.writeHeader(&svg, usedClasses)
	r.pushLines(&svg, lines, cursor)
	svg.WriteString("</svg></svg>")
	return svg.String()
}

func (r *ResvgRenderer) scanUsedClasses(lines [][]vt10x.Glyph, cursor vt10x.Cursor, usedClasses map[string]bool) {
	for row, line := range lines {
		for col, glyph := range line {
			ch := glyph.Char
			if ch == ' ' || ch == 0 {
				continue
			}
			attrs := getTextAttrs(&glyph, cursor, col, row, &r.theme)
			if attrs.bold {
				usedClasses["b"] = true
			}
			if attrs.italic {
				usedClasses["i"] = true
			}
			if attrs.underline {
				usedClasses["u"] = true
			}
		}
	}
}

func (r *ResvgRenderer) writeHeader(svg *strings.Builder, usedClasses map[string]bool) {
	width := float64(r.pixelWidth)
	height := float64(r.pixelHeight)
	x := 1.0 * 100.0 / (float64(r.width) + 2.0)
	y := 0.5 * 100.0 / (float64(r.height) + 1.0)

	bgColor := rgbToHex(r.theme.Background)
	fgColor := rgbToHex(r.theme.Foreground)

	fmt.Fprintf(svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-size="%.0f" font-family="%s">`,
		width, height, r.fontSize, r.fontFamily)

	if len(usedClasses) > 0 {
		svg.WriteString("<style>")
		if usedClasses["b"] {
			svg.WriteString(".b{font-weight:bold}")
		}
		if usedClasses["i"] {
			svg.WriteString(".i{font-style:italic}")
		}
		if usedClasses["u"] {
			svg.WriteString(".u{text-decoration:underline}")
		}
		svg.WriteString("</style>")
	}

	fmt.Fprintf(svg, `<rect width="100%%" height="100%%" fill="%s"/><svg x="%.3f%%" y="%.3f%%" fill="%s">`,
		bgColor, x, y, fgColor)
}

func (r *ResvgRenderer) getColorHex(color vt10x.Color) string {
	if hex, ok := r.colorCache[color]; ok {
		return hex
	}
	hex := colorToHex(color, &r.theme)
	r.colorCache[color] = hex
	return hex
}

func (r *ResvgRenderer) pushLines(svg *strings.Builder, lines [][]vt10x.Glyph, cursor vt10x.Cursor) {
	r.pushBackground(svg, lines, cursor)
	r.pushText(svg, lines, cursor)
}

func (r *ResvgRenderer) pushBackground(svg *strings.Builder, lines [][]vt10x.Glyph, cursor vt10x.Cursor) {
	cols, rows := r.width, r.height
	type bgRect struct {
		x, y, w float64
		color   string
	}
	var rects []bgRect

	for row, line := range lines {
		y := 100.0 * float64(row) / (float64(rows) + 1.0)
		var currentRect *bgRect

		for col, glyph := range line {
			attrs := getTextAttrs(&glyph, cursor, col, row, &r.theme)

			if attrs.background == vt10x.DefaultBG {
				currentRect = nil
				continue
			}

			x := 100.0 * float64(col) / (float64(cols) + 2.0)
			color := r.getColorHex(attrs.background)

			if currentRect != nil && currentRect.color == color && currentRect.y == y {
				currentRect.w += r.charWidth
			} else {
				currentRect = &bgRect{x: x, y: y, w: r.charWidth, color: color}
				rects = append(rects, *currentRect)
			}
		}
	}

	if len(rects) > 0 {
		svg.WriteString(`<g>`)
		for _, rect := range rects {
			fmt.Fprintf(svg, `<rect x="%.3f%%" y="%.3f%%" width="%.3f%%" height="%.3f" fill="%s"/>`,
				rect.x, rect.y, rect.w, r.rowHeight, rect.color)
		}
		svg.WriteString("</g>")
	}
}

func (r *ResvgRenderer) pushText(svg *strings.Builder, lines [][]vt10x.Glyph, cursor vt10x.Cursor) {
	cols, rows := r.width, r.height

	svg.WriteString(`<text>`)

	for row, line := range lines {
		if len(line) == 0 {
			continue
		}

		y := 100.0 * float64(row) / (float64(rows) + 1.0)
		hasContent := false

		for _, glyph := range line {
			if glyph.Char != ' ' && glyph.Char != 0 {
				hasContent = true
				break
			}
		}

		if !hasContent {
			continue
		}

		fmt.Fprintf(svg, `<tspan y="%.3f%%" dy="1em">`, y)

		type span struct {
			x     float64
			text  strings.Builder
			class string
			color string
		}

		var spans []span
		var currentSpan *span

		for col, glyph := range line {
			ch := glyph.Char

			if ch == ' ' || ch == 0 {
				currentSpan = nil
				continue
			}

			attrs := getTextAttrs(&glyph, cursor, col, row, &r.theme)
			x := 100.0 * float64(col) / (float64(cols) + 2.0)
			class := attrsToClasses(&attrs)
			color := r.getColorHex(attrs.foreground)

			needNewSpan := currentSpan == nil ||
				currentSpan.class != class ||
				currentSpan.color != color ||
				x > currentSpan.x+r.charWidth*1.5

			if needNewSpan {
				currentSpan = &span{x: x, class: class, color: color}
				spans = append(spans, *currentSpan)
			}

			spanPtr := &spans[len(spans)-1]
			switch ch {
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
				spanPtr.text.WriteRune(ch)
			}
		}

		for _, s := range spans {
			svg.WriteString(`<tspan x="`)
			svg.WriteString(fmt.Sprintf("%.3f%%", s.x))
			svg.WriteString(`"`)
			if s.class != "" {
				svg.WriteString(` class="`)
				svg.WriteString(s.class)
				svg.WriteString(`"`)
			}
			if s.color != r.getColorHex(vt10x.DefaultFG) {
				svg.WriteString(` fill="`)
				svg.WriteString(s.color)
				svg.WriteString(`"`)
			}
			svg.WriteString(`>`)
			svg.WriteString(s.text.String())
			svg.WriteString(`</tspan>`)
		}

		svg.WriteString("</tspan>")
	}

	svg.WriteString("</text>")
}

func getTextAttrs(glyph *vt10x.Glyph, cursor vt10x.Cursor, col, row int, theme *Theme) textAttrs {
	foreground := glyph.FG
	background := glyph.BG
	inverse := cursor.X == col && cursor.Y == row

	if glyph.Mode&AttrBold != 0 && foreground < 8 {
		foreground += 8
	}

	if glyph.Mode&AttrBlink != 0 && background < 8 {
		background += 8
	}

	if (glyph.Mode&AttrReverse != 0) != inverse {
		foreground, background = background, foreground

		if foreground == vt10x.DefaultBG {
			foreground = vt10x.Color(7)
		}
		if background == vt10x.DefaultFG {
			background = vt10x.Color(0)
		}
	}

	return textAttrs{
		foreground: foreground,
		background: background,
		bold:       glyph.Mode&AttrBold != 0,
		faint:      glyph.Mode&AttrFaint != 0,
		italic:     glyph.Mode&AttrItalic != 0,
		underline:  glyph.Mode&AttrUnderline != 0,
		blink:      glyph.Mode&AttrBlink != 0,
		reverse:    glyph.Mode&AttrReverse != 0,
	}
}

func (r *ResvgRenderer) PixelSize() (int, int) {
	return r.pixelWidth, r.pixelHeight
}
