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

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminalStateToANSI_BasicText(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Hello, World!"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[!p")       // Reset terminal
	assert.Contains(t, ansi, "\x1b[2J")       // Clear screen
	assert.Contains(t, ansi, "\x1b[H")        // Home cursor
	assert.Contains(t, ansi, "\x1b[8;24;80t") // Set terminal size
	assert.Contains(t, ansi, "Hello, World!")
	assert.Contains(t, ansi, "\x1b[1;14H") // Cursor position after writing text
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

	vt.Write([]byte("\x1b[38;5;196mColor 196\x1b[0m")) // 256 color foreground
	vt.Write([]byte("\x1b[48;5;21m BG 21 \x1b[0m"))    // 256 color background

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b[38;5;196m")
	assert.Contains(t, ansi, "Color 196")
	assert.Contains(t, ansi, "\x1b[48;5;21m")
	assert.Contains(t, ansi, " BG 21 ")
}

func TestTerminalStateToANSI_RGBColors(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[38;2;255;128;64mRGB Text\x1b[0m")) // RGB foreground color

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
	vt.Write([]byte("\x1b[5;10H")) // Move cursor to (5, 10)
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
	vt.Write([]byte("\x1b[?1049h")) // Switch to alternate screen
	vt.Write([]byte("Alternate buffer text"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "Primary buffer text")
	assert.Contains(t, ansi, "\x1b[?1049h")
	assert.Contains(t, ansi, "Alternate buffer text")
	assert.True(t, state.AltScreen)

	// Feed the ANSI output back into a new terminal to verify the correct buffer is displayed
	vt2 := vt10x.New(vt10x.WithSize(80, 24))
	vt2.Write([]byte(ansi))

	assert.True(t, checkTextInTerminal(vt2, "Alternate"), "Alternate buffer text should be visible in the reproduced terminal")
}

func TestTerminalStateToANSI_AlternateScreenSwitch(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("Main screen\n"))
	vt.Write([]byte("\x1b[2;5H"))
	vt.Write([]byte("Position saved"))

	vt.Write([]byte("\x1b[?1049h")) // Switch to alternate screen
	vt.Write([]byte("\x1b[2J\x1b[H"))
	vt.Write([]byte("Alt screen content"))

	vt.Write([]byte("\x1b[?1049l")) // Switch back to main screen

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.False(t, state.AltScreen)
	assert.Contains(t, ansi, "Main screen")
	assert.Contains(t, ansi, "Position saved")

	// Feed the ANSI output back into a new terminal to verify the correct buffer is displayed
	vt2 := vt10x.New(vt10x.WithSize(80, 24))
	vt2.Write([]byte(ansi))

	foundMain := checkTextInTerminal(vt2, "Main screen")
	foundAlt := checkTextInTerminal(vt2, "Alt")
	assert.True(t, foundMain, "Main buffer text should be visible in the reproduced terminal")
	assert.False(t, foundAlt, "Alternate buffer text should NOT be visible after switching back")
}

func TestTerminalStateToANSI_ScrollRegion(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[5;20r")) // Set scroll region from line 5 to 20
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

	// Set some tab stops
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

	// Set terminal title
	vt.Write([]byte("\x1b]0;Teleport\x07"))
	vt.Write([]byte("Content"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "\x1b]0;Teleport\x07")
	assert.Equal(t, "Teleport", state.Title)
}

func TestTerminalStateToANSI_Modes(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(80, 24))

	vt.Write([]byte("\x1b[?7h"))  // Enable line wrap
	vt.Write([]byte("\x1b[4h"))   // Enable insert mode
	vt.Write([]byte("\x1b[?6h"))  // Enable origin mode
	vt.Write([]byte("\x1b[?5h"))  // Enable reverse video
	vt.Write([]byte("\x1b[?25l")) // Hide cursor

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
	vt.Write([]byte("\x1b[3;15H"))    // Move cursor to (3, 15)
	vt.Write([]byte("\x1b[?1049h"))   // Switch to alternate screen
	vt.Write([]byte("\x1b[2J\x1b[H")) // Clear and home cursor in alternate screen
	vt.Write([]byte("\x1b[32mGreen in alt\x1b[0m\n"))
	vt.Write([]byte("\x1b[4mUnderlined alt\x1b[0m\n"))
	vt.Write([]byte("\x1b[5;20H")) // Move cursor to (5, 20)

	altState := vt.DumpState()
	ansiAlt := terminalStateToANSI(altState)

	assert.True(t, altState.AltScreen)
	assert.Contains(t, ansiAlt, "\x1b[32m")
	assert.Contains(t, ansiAlt, "Green in alt")
	assert.Contains(t, ansiAlt, "\x1b[?1049h")

	// Feed the alternate screen ANSI back into a new terminal to verify the
	// alternate buffer content is being displayed correctly
	vtAlt := vt10x.New(vt10x.WithSize(80, 24))
	vtAlt.Write([]byte(ansiAlt))

	assert.True(t, checkTextInTerminal(vtAlt, "Green"), "Green text should be visible in alternate buffer")
	assert.True(t, checkTextInTerminal(vtAlt, "Underlined"), "Underlined text should be visible in alternate buffer")

	vt.Write([]byte("\x1b[?1049l")) // Switch back to main screen

	mainState := vt.DumpState()
	ansiMain := terminalStateToANSI(mainState)

	assert.False(t, mainState.AltScreen)
	assert.Contains(t, ansiMain, "\x1b[31m")
	assert.Contains(t, ansiMain, "Red text in main")
	assert.Contains(t, ansiMain, "\x1b[1m")
	assert.Contains(t, ansiMain, "Bold main text")

	// Feed the main screen ANSI back into a new terminal to verify the
	// main buffer content is being displayed correctly
	vtMain := vt10x.New(vt10x.WithSize(80, 24))
	vtMain.Write([]byte(ansiMain))

	assert.True(t, checkTextInTerminal(vtMain, "Red"), "Red text should be visible in main buffer")
	assert.True(t, checkTextInTerminal(vtMain, "Bold"), "Bold text should be visible in main buffer")

	foundAltContent := checkTextInTerminal(vtMain, "Green in alt")
	assert.False(t, foundAltContent, "Alternate buffer content should NOT be visible in main buffer")
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
	vt.Write([]byte("\x1b[1;5H")) // Move cursor to (1, 5)
	vt.Write([]byte("B"))
	vt.Write([]byte("\x1b[3;3H")) // Move cursor to (3, 3)
	vt.Write([]byte("C"))

	state := vt.DumpState()
	ansi := terminalStateToANSI(state)

	assert.Contains(t, ansi, "A")
	assert.Contains(t, ansi, "B")
	assert.Contains(t, ansi, "C")
}

func checkTextInTerminal(vt vt10x.Terminal, searchText string) bool {
	for y := 0; y < 24; y++ {
		var lineText strings.Builder

		for x := 0; x < 80; x++ {
			cell := vt.Cell(x, y)
			if cell.Char != 0 {
				lineText.WriteRune(cell.Char)
			}
		}

		if strings.Contains(lineText.String(), searchText) {
			return true
		}
	}

	return false
}
