// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package debuglive

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	accentColor  = lipgloss.Color("4")  // blue
	errorColor   = lipgloss.Color("1")  // red
	warnColor    = lipgloss.Color("3")  // yellow
	successColor = lipgloss.Color("2")  // green
	faintColor   = lipgloss.Color("8")  // gray
	whiteColor   = lipgloss.Color("15") // white

	// Pane border styles
	activeBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)

	inactiveBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(faintColor)

	// Section header in the picker
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)

	// Instance items in the picker
	normalItemStyle   = lipgloss.NewStyle().Faint(true)
	selectedItemStyle = lipgloss.NewStyle().Foreground(accentColor)
	activeItemStyle   = lipgloss.NewStyle().Foreground(successColor).Bold(true)

	// Tab styles
	activeTabStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	inactiveTabStyle = lipgloss.NewStyle().Faint(true)

	tabSeparator = lipgloss.NewStyle().
			Faint(true).
			Render(" | ")

	// Log level colors
	logErrorStyle = lipgloss.NewStyle().Foreground(errorColor)
	logWarnStyle  = lipgloss.NewStyle().Foreground(warnColor)
	logInfoStyle  = lipgloss.NewStyle().Foreground(successColor)
	logDebugStyle = lipgloss.NewStyle().Faint(true)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().Faint(true)

	// Help bar
	helpBarStyle = lipgloss.NewStyle().Faint(true)
)
