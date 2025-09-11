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
	"testing"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestTerminalStateToANSI(t *testing.T) {
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
				vt.Write([]byte("\x1b[38;5;196mColor 196\x1b[0m")) // 256 color foreground
				vt.Write([]byte("\x1b[48;5;21m BG 21 \x1b[0m"))    // 256 color background
			},
		},
		{
			name:   "RGBColors",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[38;2;255;128;64mRGB Text\x1b[0m")) // RGB foreground color
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
				vt.Write([]byte("\x1b[5;10H")) // Move cursor to (5, 10)
				vt.Write([]byte("At 5,10"))
			},
		},
		{
			name:   "AlternateScreen",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("Primary buffer text"))
				vt.Write([]byte("\x1b[?1049h")) // Switch to alternate screen
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

				vt.Write([]byte("\x1b[?1049h")) // Switch to alternate screen
				vt.Write([]byte("\x1b[2J\x1b[H"))
				vt.Write([]byte("Alt screen content"))

				vt.Write([]byte("\x1b[?1049l")) // Switch back to main screen
			},
		},
		{
			name:   "ScrollRegion",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[5;20r")) // Set scroll region from line 5 to 20
				vt.Write([]byte("Text in scroll region"))
			},
		},
		{
			name:   "TabStops",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				// Set some tab stops
				vt.Write([]byte("\x1b[3g"))
				vt.Write([]byte("\x1b[10G\x1bH"))
				vt.Write([]byte("\x1b[20G\x1bH"))
				vt.Write([]byte("\x1b[30G\x1bH"))
				vt.Write([]byte("\x1b[1G"))
				vt.Write([]byte("A\tB\tC\tD"))
			},
		},
		{
			name:   "Title",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				// Set terminal title
				vt.Write([]byte("\x1b]0;Teleport\x07"))
				vt.Write([]byte("Content"))
			},
		},
		{
			name:   "Modes",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[?7h"))  // Enable line wrap
				vt.Write([]byte("\x1b[4h"))   // Enable insert mode
				vt.Write([]byte("\x1b[?6h"))  // Enable origin mode
				vt.Write([]byte("\x1b[?5h"))  // Enable reverse video
				vt.Write([]byte("\x1b[?25l")) // Hide cursor
			},
		},
		{
			name:   "ComplexBufferSwap",
			width:  80,
			height: 24,
			setup: func(vt vt10x.Terminal) {
				vt.Write([]byte("\x1b[31mRed text in main\x1b[0m\n"))
				vt.Write([]byte("\x1b[1mBold main text\x1b[0m\n"))
				vt.Write([]byte("\x1b[3;15H"))    // Move cursor to (3, 15)
				vt.Write([]byte("\x1b[?1049h"))   // Switch to alternate screen
				vt.Write([]byte("\x1b[2J\x1b[H")) // Clear and home cursor in alternate screen
				vt.Write([]byte("\x1b[32mGreen in alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[4mUnderlined alt\x1b[0m\n"))
				vt.Write([]byte("\x1b[5;20H")) // Move cursor to (5, 20)
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
				vt.Write([]byte("\x1b[1;5H")) // Move cursor to (1, 5)
				vt.Write([]byte("B"))
				vt.Write([]byte("\x1b[3;3H")) // Move cursor to (3, 3)
				vt.Write([]byte("C"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vt := vt10x.New(vt10x.WithSize(tt.width, tt.height))

			tt.setup(vt)

			var buf bytes.Buffer
			state := vt.DumpState()
			VtStateToANSI(&buf, state)

			if golden.ShouldSet() {
				golden.Set(t, buf.Bytes())
			}

			require.Equal(t, string(golden.Get(t)), buf.String())
		})
	}
}
