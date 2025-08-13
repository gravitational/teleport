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

package ttyplayback

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gravitational/vt10x"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminalStateToANSI_BasicText(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Hello, World!"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[!p")
	assert.Contains(t, ansi, "\x1b[2J")
	assert.Contains(t, ansi, "\x1b[H")
	assert.Contains(t, ansi, "\x1b[8;24;80t")
	assert.Contains(t, ansi, "Hello, World!")
	assert.Contains(t, ansi, "\x1b[1;14H")
}

func TestTerminalStateToANSI_Colors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[31mRed\x1b[0m \x1b[42mGreen BG\x1b[0m"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[31m")
	assert.Contains(t, ansi, "Red")
	assert.Contains(t, ansi, "\x1b[42m")
	assert.Contains(t, ansi, "Green BG")
}

func TestTerminalStateToANSI_256Colors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[38;5;196mColor 196\x1b[0m"))
	vt.Write([]byte("\x1b[48;5;21m BG 21 \x1b[0m"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[38;5;196m")
	assert.Contains(t, ansi, "Color 196")
	assert.Contains(t, ansi, "\x1b[48;5;21m")
	assert.Contains(t, ansi, " BG 21 ")
}

func TestTerminalStateToANSI_RGBColors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[38;2;255;128;64mRGB Text\x1b[0m"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[38;2;255;128;64m")
	assert.Contains(t, ansi, "RGB Text")
}

func TestTerminalStateToANSI_TextAttributes(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[1mBold\x1b[0m "))
	vt.Write([]byte("\x1b[3mItalic\x1b[0m "))
	vt.Write([]byte("\x1b[4mUnderline\x1b[0m "))
	vt.Write([]byte("\x1b[5mBlink\x1b[0m"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[1m")
	assert.Contains(t, ansi, "Bold")
	assert.Contains(t, ansi, "\x1b[3m")
	assert.Contains(t, ansi, "Italic")
	assert.Contains(t, ansi, "\x1b[4m")
	assert.Contains(t, ansi, "Underline")
	assert.Contains(t, ansi, "\x1b[5m")
	assert.Contains(t, ansi, "Blink")
}

func TestTerminalStateToANSI_CursorPosition(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Line 1\n"))
	vt.Write([]byte("Line 2\n"))
	vt.Write([]byte("\x1b[5;10H"))
	vt.Write([]byte("At 5,10"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "Line 1")
	assert.Contains(t, ansi, "Line 2")
	assert.Contains(t, ansi, "At 5,10")
	assert.Contains(t, ansi, "\x1b[5;17H")
}

func TestTerminalStateToANSI_AlternateScreen(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Primary buffer text"))
	vt.Write([]byte("\x1b[?1049h"))
	vt.Write([]byte("Alternate buffer text"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "Primary buffer text")
	assert.Contains(t, ansi, "\x1b[?1049h")
	assert.Contains(t, ansi, "Alternate buffer text")
	assert.True(t, state.AltScreen)
}

func TestTerminalStateToANSI_AlternateScreenSwitch(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Main screen\n"))
	vt.Write([]byte("\x1b[2;5H"))
	vt.Write([]byte("Position saved"))

	vt.Write([]byte("\x1b[?1049h"))
	vt.Write([]byte("\x1b[2J\x1b[H"))
	vt.Write([]byte("Alt screen content"))

	vt.Write([]byte("\x1b[?1049l"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.False(t, state.AltScreen)
	assert.Contains(t, ansi, "Main screen")
	assert.Contains(t, ansi, "Position saved")
}

func TestTerminalStateToANSI_ScrollRegion(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[5;20r"))
	vt.Write([]byte("Text in scroll region"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[5;20r")
	assert.Contains(t, ansi, "Text in scroll region")
	assert.Equal(t, 4, state.ScrollTop)
	assert.Equal(t, 19, state.ScrollBottom)
}

func TestTerminalStateToANSI_TabStops(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[3g"))
	vt.Write([]byte("\x1b[10G\x1bH"))
	vt.Write([]byte("\x1b[20G\x1bH"))
	vt.Write([]byte("\x1b[30G\x1bH"))
	vt.Write([]byte("\x1b[1G"))
	vt.Write([]byte("A\tB\tC\tD"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[3g")
	assert.Contains(t, ansi, "\x1b[10G")
	assert.Contains(t, ansi, "\x1b[20G")
	assert.Contains(t, ansi, "\x1b[30G")
}

func TestTerminalStateToANSI_Title(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b]0;Teleport\x07"))
	vt.Write([]byte("Content"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b]0;Teleport\x07")
	assert.Equal(t, "Teleport", state.Title)
}

func TestTerminalStateToANSI_Modes(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[?7h"))
	vt.Write([]byte("\x1b[4h"))
	vt.Write([]byte("\x1b[?6h"))
	vt.Write([]byte("\x1b[?5h"))
	vt.Write([]byte("\x1b[?25l"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[?7h")
	assert.Contains(t, ansi, "\x1b[4h")
	assert.Contains(t, ansi, "\x1b[?6h")
	assert.Contains(t, ansi, "\x1b[?5h")
	assert.Contains(t, ansi, "\x1b[?25l")

	assert.True(t, state.Wrap)
	assert.True(t, state.Insert)
	assert.True(t, state.Origin)
	assert.True(t, state.ReverseVideo)
	assert.False(t, state.CursorVisible)
}

func TestTerminalStateToANSI_ComplexBufferSwap(t *testing.T) {
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
	ansiAlt := terminalStateToANSI(altState)

	assert.True(t, altState.AltScreen)
	assert.Contains(t, ansiAlt, "\x1b[32m")
	assert.Contains(t, ansiAlt, "Green in alt")
	assert.Contains(t, ansiAlt, "\x1b[?1049h")

	vt.Write([]byte("\x1b[?1049l"))

	mainState := vt.DumpState()
	ansiMain := terminalStateToANSI(mainState)

	assert.False(t, mainState.AltScreen)
	assert.Contains(t, ansiMain, "\x1b[31m")
	assert.Contains(t, ansiMain, "Red text in main")
	assert.Contains(t, ansiMain, "\x1b[1m")
	assert.Contains(t, ansiMain, "Bold main text")
}

func TestTerminalStateToANSI_ReverseVideo(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[7mReversed text\x1b[0m"))
	vt.Write([]byte(" Normal"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	buf := bytes.NewBufferString(ansi)
	content := buf.String()

	require.Contains(t, content, "Reversed text")
	require.Contains(t, content, "Normal")
}

func TestTerminalStateToANSI_MultilineWithColors(t *testing.T) {
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
	ansi := terminalStateToANSI(state)

	for i, expectedColor := range []string{"\x1b[31m", "\x1b[32m", "\x1b[33m", "\x1b[34m", "\x1b[35m"} {
		assert.Contains(t, ansi, expectedColor)
		assert.Contains(t, ansi, strings.TrimSuffix(lines[i][5:], "\x1b[0m"))
	}
}

func TestTerminalStateToANSI_EmptyGlyphs(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(10, 5))

	vt.Write([]byte("A"))
	vt.Write([]byte("\x1b[1;5H"))
	vt.Write([]byte("B"))
	vt.Write([]byte("\x1b[3;3H"))
	vt.Write([]byte("C"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "A")
	assert.Contains(t, ansi, "B")
	assert.Contains(t, ansi, "C")
}
