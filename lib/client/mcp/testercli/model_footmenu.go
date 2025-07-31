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

package testercli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	noStyle      = lipgloss.NewStyle()
)

type footMenuAction int

func (f footMenuAction) String() string {
	switch f {
	case footMenuSubmit:
		return "Submit"
	case footMenuSelect:
		return "Select"
	case footMenuBack:
		return "Back"
	case footMenuQuit:
		return "Quit"
	default:
		return "<unknown>"
	}
}

const (
	footMenuNone footMenuAction = iota
	footMenuSelect
	footMenuBack
	footMenuQuit
	footMenuSubmit
)

type footMenu struct {
	actions []footMenuAction
	cur     int
	focused bool
}

func (m *footMenu) update(key tea.KeyMsg) footMenuAction {
	if !m.focused {
		switch key.String() {
		case "q", "ctrl+c":
			return footMenuQuit
		case "b":
			return footMenuBack
		}
		return footMenuNone
	}

	switch key.Type {
	case tea.KeyLeft:
		m.cur = max(m.cur-1, 0)
	case tea.KeyRight:
		m.cur = min(m.cur+1, len(m.actions)-1)
	case tea.KeyUp, tea.KeyDown:
		m.maybeResetToSelect()
	case tea.KeyEnter:
		if len(m.actions) > 0 {
			return m.actions[m.cur]
		}
	default:
		switch key.String() {
		case "q", "ctrl+c":
			return footMenuQuit
		case "b":
			return footMenuBack
		}
	}
	return footMenuNone
}

func (m *footMenu) maybeResetToSelect() {
	if len(m.actions) != 0 &&
		m.actions[0] == footMenuSelect {
		m.cur = 0
	}
}

func (m *footMenu) view() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for i, action := range m.actions {
		if i == m.cur && m.focused {
			sb.WriteString(focusedStyle.Render(fmt.Sprintf("[ %s ] ", action.String())))
		} else {
			sb.WriteString(blurredStyle.Render(fmt.Sprintf("[ %s ] ", action.String())))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
