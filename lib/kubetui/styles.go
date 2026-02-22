/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package kubetui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7B2FBE") // Teleport purple
	secondaryColor = lipgloss.Color("#00BFA5")
	errorColor     = lipgloss.Color("#FF5252")
	warningColor   = lipgloss.Color("#FFB74D")
	successColor   = lipgloss.Color("#69F0AE")
	subtleColor    = lipgloss.Color("#666666")
	whiteColor     = lipgloss.Color("#FFFFFF")
	dimColor       = lipgloss.Color("#999999")

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor).
			Background(primaryColor).
			Padding(0, 1)

	// Status bar at the bottom
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1)

	// Active status bar segment
	statusBarActiveStyle = lipgloss.NewStyle().
				Foreground(whiteColor).
				Background(primaryColor).
				Padding(0, 1)

	// Table header
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(subtleColor)

	// Selected row in table
	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(whiteColor).
				Background(primaryColor)

	// Pod status styles
	statusRunningStyle = lipgloss.NewStyle().Foreground(successColor)
	statusPendingStyle = lipgloss.NewStyle().Foreground(warningColor)
	statusFailedStyle  = lipgloss.NewStyle().Foreground(errorColor)
	statusOtherStyle   = lipgloss.NewStyle().Foreground(dimColor)

	// Help bar at bottom
	helpStyle = lipgloss.NewStyle().Foreground(subtleColor)

	// Filter/search input
	filterPromptStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)

	// Viewport title for logs/describe
	viewportTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(whiteColor).
				Background(secondaryColor).
				Padding(0, 1)

	// Error message style
	errorStyle = lipgloss.NewStyle().Foreground(errorColor).Bold(true)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().Foreground(primaryColor)
)
