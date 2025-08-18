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
	"strings"
	"testing"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminalStateToSVG_BasicText(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Hello, World!"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `<svg xmlns="http://www.w3.org/2000/svg"`)
	assert.Contains(t, svg, `font-size="14"`)
	assert.Contains(t, svg, `class="terminal"`)
	assert.Contains(t, svg, "Hello, World!")
}

func TestTerminalStateToSVG_Colors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[31mRed\x1b[0m \x1b[42mGreen BG\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `class="fg-1"`)
	assert.Contains(t, svg, "Red")
	assert.Contains(t, svg, `class="bg-2"`)
	assert.Contains(t, svg, "Green BG")
}

func TestTerminalStateToSVG_256Colors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[38;5;196mColor 196\x1b[0m"))
	vt.Write([]byte("\x1b[48;5;21m BG 21 \x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `style="fill:#ff0000"`)
	assert.Contains(t, svg, "Color 196")
	assert.Contains(t, svg, `style="fill:#0000ff"`)
	assert.Contains(t, svg, " BG 21 ")
}

func TestTerminalStateToSVG_RGBColors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[38;2;255;128;64mRGB Text\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `style="fill:#ff8040"`)
	assert.Contains(t, svg, "RGB Text")
}

func TestTerminalStateToSVG_TextAttributes(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[1mBold\x1b[0m "))
	vt.Write([]byte("\x1b[3mItalic\x1b[0m "))
	vt.Write([]byte("\x1b[4mUnderline\x1b[0m "))
	vt.Write([]byte("\x1b[5mBlink\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `class="b"`)
	assert.Contains(t, svg, "Bold")
	assert.Contains(t, svg, `class="i"`)
	assert.Contains(t, svg, "Italic")
	assert.Contains(t, svg, `class="u"`)
	assert.Contains(t, svg, "Underline")
	assert.Contains(t, svg, "Blink")
}

func TestTerminalStateToSVG_CursorPosition(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Line 1\n"))
	vt.Write([]byte("Line 2\n"))
	vt.Write([]byte("\x1b[5;10H"))
	vt.Write([]byte("At 5,10"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "Line 1")
	assert.Contains(t, svg, "Line 2")
	assert.Contains(t, svg, "At 5,10")
}

func TestTerminalStateToSVG_AlternateScreen(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Primary buffer text"))
	vt.Write([]byte("\x1b[?1049h"))
	vt.Write([]byte("Alternate buffer text"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "Alternate buffer text")
	assert.True(t, state.AltScreen)
}

func TestTerminalStateToSVG_AlternateScreenSwitch(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Main screen\n"))
	vt.Write([]byte("\x1b[2;5H"))
	vt.Write([]byte("Position saved"))

	vt.Write([]byte("\x1b[?1049h"))
	vt.Write([]byte("\x1b[2J\x1b[H"))
	vt.Write([]byte("Alt screen content"))

	vt.Write([]byte("\x1b[?1049l"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.False(t, state.AltScreen)
	assert.Contains(t, svg, "Main screen")
	assert.Contains(t, svg, "Position saved")
}

func TestTerminalStateToSVG_SpecialCharacters(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte(`<tag> & "quotes" 'apostrophes'`))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "&lt;tag&gt;")
	assert.Contains(t, svg, "&amp;")
	assert.Contains(t, svg, "&quot;quotes&quot;")
	assert.Contains(t, svg, "&#39;apostrophes&#39;")
}

func TestTerminalStateToSVG_ReverseVideo(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[7mReversed text\x1b[0m"))
	vt.Write([]byte(" Normal"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	require.Contains(t, svg, "Reversed text")
	require.Contains(t, svg, "Normal")
}

func TestTerminalStateToSVG_MultilineWithColors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 10))

	lines := []string{
		"\x1b[31mLine 1 Red\x1b[0m",
		"\x1b[32mLine 2 Green\x1b[0m",
		"\x1b[33mLine 3 Yellow\x1b[0m",
		"\x1b[34mLine 4 Blue\x1b[0m",
		"\x1b[35mLine 5 Magenta\x1b[0m",
	}

	for _, line := range lines {
		vt.Write([]byte(line + "\n"))
	}

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	for i, expectedColor := range []string{"fg-1", "fg-2", "fg-3", "fg-4", "fg-5"} {
		assert.Contains(t, svg, expectedColor)
		assert.Contains(t, svg, strings.TrimSuffix(lines[i][5:], "\x1b[0m"))
	}
}

func TestTerminalStateToSVG_EmptyGlyphs(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(10, 5))

	vt.Write([]byte("A"))
	vt.Write([]byte("\x1b[1;5H"))
	vt.Write([]byte("B"))
	vt.Write([]byte("\x1b[3;3H"))
	vt.Write([]byte("C"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "A")
	assert.Contains(t, svg, "B")
	assert.Contains(t, svg, "C")
}

func TestTerminalStateToSVG_ComplexBufferSwap(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[31mRed text in main\x1b[0m\n"))
	vt.Write([]byte("\x1b[1mBold main text\x1b[0m\n"))
	vt.Write([]byte("\x1b[3;15H"))
	vt.Write([]byte("\x1b[?1049h"))
	vt.Write([]byte("\x1b[2J\x1b[H"))
	vt.Write([]byte("\x1b[32mGreen in alt\x1b[0m\n"))
	vt.Write([]byte("\x1b[4mUnderlined alt\x1b[0m\n"))
	vt.Write([]byte("\x1b[5;20H"))

	altState := vt.DumpState()
	svgAlt := VtStateToSvg(&altState)

	assert.True(t, altState.AltScreen)
	assert.Contains(t, svgAlt, `class="fg-2"`)
	assert.Contains(t, svgAlt, "Green in alt")

	vt.Write([]byte("\x1b[?1049l"))

	mainState := vt.DumpState()
	svgMain := VtStateToSvg(&mainState)

	assert.False(t, mainState.AltScreen)
	assert.Contains(t, svgMain, `class="fg-1"`)
	assert.Contains(t, svgMain, "Red text in main")
	assert.Contains(t, svgMain, `class="b"`)
	assert.Contains(t, svgMain, "Bold main text")
}

func TestColorToHex(t *testing.T) {
	tests := []struct {
		name     string
		color    vt10x.Color
		expected string
	}{
		{
			name:     "Basic color (< 16)",
			color:    vt10x.Color(5),
			expected: "#000000",
		},
		{
			name:     "256 color palette - low",
			color:    vt10x.Color(16),
			expected: "#000000",
		},
		{
			name:     "256 color palette - red",
			color:    vt10x.Color(196),
			expected: "#ff0000",
		},
		{
			name:     "256 color palette - green",
			color:    vt10x.Color(46),
			expected: "#00ff00",
		},
		{
			name:     "256 color palette - blue",
			color:    vt10x.Color(21),
			expected: "#0000ff",
		},
		{
			name:     "256 color palette - mixed",
			color:    vt10x.Color(123),
			expected: "#66ffff",
		},
		{
			name:     "Grayscale - start",
			color:    vt10x.Color(232),
			expected: "#080808",
		},
		{
			name:     "Grayscale - middle",
			color:    vt10x.Color(243),
			expected: "#767676",
		},
		{
			name:     "Grayscale - end",
			color:    vt10x.Color(255),
			expected: "#eeeeee",
		},
		{
			name:     "RGB color",
			color:    vt10x.Color(0xFF8040),
			expected: "#ff8040",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := colorToHex(tt.color)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTerminalStateToSVG_SVGStructure(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Test"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `<svg xmlns="http://www.w3.org/2000/svg"`)
	assert.Contains(t, svg, `width="`)
	assert.Contains(t, svg, `height="`)
	assert.Contains(t, svg, `<style>`)
	assert.Contains(t, svg, `.b{font-weight:bold}`)
	assert.Contains(t, svg, `.i{font-style:italic}`)
	assert.Contains(t, svg, `.u{text-decoration:underline}`)
	assert.Contains(t, svg, `</style>`)
	assert.Contains(t, svg, `<rect width="100%" height="100%" class="bg-default"/>`)
	assert.Contains(t, svg, `<text>`)
	assert.Contains(t, svg, `</text>`)
	assert.Contains(t, svg, `</svg></svg>`)
}

func TestTerminalStateToSVG_BackgroundRects(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[41mRed BG\x1b[0m \x1b[42mGreen BG\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `<rect`)
	assert.Contains(t, svg, `class="bg-1"`)
	assert.Contains(t, svg, `class="bg-2"`)
}

func TestTerminalStateToSVG_ExtendedBackgroundColors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[48;5;196mExtended BG\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `style="fill:#ff0000"`)
	assert.Contains(t, svg, "Extended BG")
}

func TestTerminalStateToSVG_CombinedAttributes(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[1;3;4mBold Italic Underline\x1b[0m"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, `class="b i u"`)
	assert.Contains(t, svg, "Bold Italic Underline")
}

func TestTerminalStateToSVG_EmptyLines(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 5))

	vt.Write([]byte("First line\n\n\nFourth line"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "First line")
	assert.Contains(t, svg, "Fourth line")
}

func TestTerminalStateToSVG_SpacedText(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("A"))
	vt.Write([]byte("\x1b[1;10H"))
	vt.Write([]byte("B"))
	vt.Write([]byte("\x1b[1;20H"))
	vt.Write([]byte("C"))

	state := vt.DumpState()
	svg := string(VtStateToSvg(&state))

	assert.Contains(t, svg, "A")
	assert.Contains(t, svg, "B")
	assert.Contains(t, svg, "C")
	assert.Contains(t, svg, `<tspan x="`)
}
