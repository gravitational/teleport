// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package top

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// boxedView wraps the provided content in a rounded border,
// with the title embedded in the top. For example, if the
// content was \t\t\tHello and the title was Some Heading the
// returned content would be:
//
// ╭Some Heading────────╮
// │                    │
// │            Hello   │
// ╰────────────────────╯
func boxedView(title string, content string, width int) string {
	rounderBorder := lipgloss.RoundedBorder()

	const borderCorners = 2
	width = width - borderCorners
	availableSpace := width - lipgloss.Width(title)

	var filler string
	if availableSpace > 0 {
		filler = strings.Repeat(rounderBorder.Top, availableSpace)
	}

	titleContent := lipgloss.NewStyle().
		Foreground(selectedColor).
		Render(title)

	renderedTitle := rounderBorder.TopLeft + titleContent + filler + rounderBorder.TopRight

	// empty out the top border since it
	// is already manually applied to the title.
	rounderBorder.TopLeft = ""
	rounderBorder.Top = ""
	rounderBorder.TopRight = ""

	contentStyle := lipgloss.NewStyle().
		BorderStyle(rounderBorder).
		PaddingLeft(1).
		PaddingRight(1).
		Faint(true).
		Width(width)

	return renderedTitle + contentStyle.Render(content)
}
