/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package recordings

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errorPopupModel is a dismissible popup that displays an error in red. It is
// used to surface failures such as a "load more" fetch error.
type errorPopupModel struct {
	err      error
	viewport viewport.Model
	keys     summaryKeyMap
	palette  palette
	width    int
	height   int
}

func newErrorPopupModel(err error, p palette, width, height int) *errorPopupModel {
	m := &errorPopupModel{
		err:     err,
		keys:    defaultSummaryKeyMap(),
		palette: p,
		width:   width,
		height:  height,
	}
	m.resize()
	m.refresh()
	return m
}

func (m *errorPopupModel) Init() tea.Cmd { return nil }

func (m *errorPopupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refresh()
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Close) {
			return m, func() tea.Msg { return closePopupMsg{} }
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *errorPopupModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.palette.danger)
	frameStyle := lipgloss.NewStyle().
		Width(m.popupWidth()).
		Height(m.popupHeight()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.palette.danger).
		Padding(0, 1)

	header := titleStyle.Render("Failed to load more sessions")
	footer := lipgloss.NewStyle().
		Faint(true).
		Render("q/esc: close")

	bodyHeight := m.popupHeight() - lipgloss.Height(header) - lipgloss.Height(footer) - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.viewport.Height = bodyHeight

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		m.viewport.View(),
		"",
		footer,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		frameStyle.Render(content),
	)
}

func (m *errorPopupModel) popupWidth() int  { return clampPopupWidth(m.width) }
func (m *errorPopupModel) popupHeight() int { return clampPopupHeight(m.height) }

func (m *errorPopupModel) resize() {
	bodyWidth := m.popupWidth() - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	bodyHeight := m.popupHeight() - 6
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.viewport.Width = bodyWidth
	m.viewport.Height = bodyHeight
}

func (m *errorPopupModel) refresh() {
	// The header is rendered red in View; the error text itself is shown as
	// code (matching the markdown renderer's inline-code style) so it stands
	// out as a verbatim server message rather than prose.
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.AdaptiveColor{Light: "236", Dark: "236"}).
		Padding(0, 1)
	m.viewport.SetContent(codeStyle.Render(sanitize(m.err.Error())))
}
