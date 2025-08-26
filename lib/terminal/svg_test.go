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
	"testing"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestTerminalStateToSVG(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(vt vt10x.Terminal)
		width  int
		height int
	}{
		{
			name:   "BasicText",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Hello, World!"))
			},
		},
		{
			name:   "Colors",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[31mRed\x1b[0m \x1b[42mGreen BG\x1b[0m"))
			},
		},
		{
			name:   "256Colors",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[38;5;196mColor 196\x1b[0m"))
				vt.Write([]byte("\x1b[48;5;21m BG 21 \x1b[0m"))
			},
		},
		{
			name:   "RGBColors",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[38;2;255;128;64mRGB Text\x1b[0m"))
			},
		},
		{
			name:   "TextAttributes",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[1mBold\x1b[0m "))
				vt.Write([]byte("\x1b[3mItalic\x1b[0m "))
				vt.Write([]byte("\x1b[4mUnderline\x1b[0m "))
				vt.Write([]byte("\x1b[5mBlink\x1b[0m"))
			},
		},
		{
			name:   "CursorPosition",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Line 1\n"))
				vt.Write([]byte("Line 2\n"))
				vt.Write([]byte("\x1b[5;10H"))
				vt.Write([]byte("At 5,10"))
			},
		},
		{
			name:   "AlternateScreen",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Primary buffer text"))
				vt.Write([]byte("\x1b[?1049h"))
				vt.Write([]byte("Alternate buffer text"))
			},
		},
		{
			name:   "AlternateScreenSwitch",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Main screen\n"))
				vt.Write([]byte("\x1b[2;5H"))
				vt.Write([]byte("Position saved"))

				vt.Write([]byte("\x1b[?1049h"))
				vt.Write([]byte("\x1b[2J\x1b[H"))
				vt.Write([]byte("Alt screen content"))

				vt.Write([]byte("\x1b[?1049l"))
			},
		},
		{
			name:   "SpecialCharacters",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte(`<tag> & "quotes" 'apostrophes'`))
			},
		},
		{
			name:   "ReverseVideo",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[7mReversed text\x1b[0m"))
				vt.Write([]byte(" Normal"))
			},
		},
		{
			name:   "MultilineWithColors",
			width:  80,
			height: 10,
			setup: func(vt vt10x.Terminal) {
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
			},
		},
		{
			name:   "EmptyGlyphs",
			width:  10,
			height: 5,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("A"))
				vt.Write([]byte("\x1b[1;5H"))
				vt.Write([]byte("B"))
				vt.Write([]byte("\x1b[3;3H"))
				vt.Write([]byte("C"))
			},
		},
		{
			name:   "ComplexBufferSwap",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[31mRed text in main\x1b[0m\n"))
				vt.Write([]byte("\x1b[1mBold main text\x1b[0m\n"))
				vt.Write([]byte("\x1b[3;15H"))
				vt.Write([]byte("\x1b[?1049h"))
				vt.Write([]byte("\x1b[2J\x1b[H"))
				vt.Write([]byte("\x1b[32mGreen in alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[4mUnderlined alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[5;20H"))
			},
		},
		{
			name:   "SVGStructure",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Test"))
			},
		},
		{
			name:   "BackgroundRects",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[41mRed BG\x1b[0m \x1b[42mGreen BG\x1b[0m"))
			},
		},
		{
			name:   "ExtendedBackgroundColors",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[48;5;196mExtended BG\x1b[0m"))
			},
		},
		{
			name:   "CombinedAttributes",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[1;3;4mBold Italic Underline\x1b[0m"))
			},
		},
		{
			name:   "EmptyLines",
			width:  80,
			height: 5,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("First line\n\n\nFourth line"))
			},
		},
		{
			name:   "SpacedText",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("A"))
				vt.Write([]byte("\x1b[1;10H"))
				vt.Write([]byte("B"))
				vt.Write([]byte("\x1b[1;20H"))
				vt.Write([]byte("C"))
			},
		},
		{
			name:   "ComplexBufferSwapMain",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[31mRed text in main\x1b[0m\n"))
				vt.Write([]byte("\x1b[1mBold main text\x1b[0m\n"))
				vt.Write([]byte("\x1b[3;15H"))
				vt.Write([]byte("\x1b[?1049h"))
				vt.Write([]byte("\x1b[2J\x1b[H"))
				vt.Write([]byte("\x1b[32mGreen in alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[4mUnderlined alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[5;20H"))
				vt.Write([]byte("\x1b[?1049l")) // Switch back to main
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vt := vt10x.New(vt10x.WithSize(tt.width, tt.height))

			tt.setup(vt)

			state := vt.DumpState()
			svg := string(VtStateToSvg(&state))

			if golden.ShouldSet() {
				golden.Set(t, []byte(svg))
			}

			require.Equal(t, string(golden.Get(t)), svg)
		})
	}
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
