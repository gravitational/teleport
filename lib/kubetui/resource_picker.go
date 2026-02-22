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

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// resourceTypeSelectedMsg is sent when the user picks a resource type.
type resourceTypeSelectedMsg struct {
	resourceType *ResourceType
}

// resourcePickerModel lets the user choose which resource type to browse.
type resourcePickerModel struct {
	cursor int
	filter string
	width  int
	height int
}

func newResourcePickerModel() resourcePickerModel {
	return resourcePickerModel{}
}

func (m resourcePickerModel) filteredTypes() []*ResourceType {
	if m.filter == "" {
		return AllResourceTypes
	}
	lower := strings.ToLower(m.filter)
	var result []*ResourceType
	for _, rt := range AllResourceTypes {
		if strings.Contains(strings.ToLower(rt.Name), lower) ||
			strings.Contains(strings.ToLower(rt.Command), lower) {
			result = append(result, rt)
		}
	}
	return result
}

func (m resourcePickerModel) Update(msg tea.Msg) (resourcePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		filtered := m.filteredTypes()
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if len(filtered) > 0 && m.cursor < len(filtered) {
				rt := filtered[m.cursor]
				return m, func() tea.Msg {
					return resourceTypeSelectedMsg{resourceType: rt}
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.filter != "" {
				m.filter = ""
				m.cursor = 0
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.cursor = 0
			}
		default:
			if len(msg.String()) == 1 && msg.String() >= " " {
				m.filter += msg.String()
				m.cursor = 0
			}
		}
	}
	return m, nil
}

func (m resourcePickerModel) View() string {
	var b strings.Builder

	b.WriteString(viewportTitleStyle.Render(" Select Resource Type ") + "\n\n")

	if m.filter != "" {
		b.WriteString(filterPromptStyle.Render("Filter: ") + m.filter + "\n\n")
	}

	filtered := m.filteredTypes()

	maxVisible := m.height - 6
	if m.filter != "" {
		maxVisible -= 2
	}
	if maxVisible < 1 {
		maxVisible = 10
	}

	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
	}

	for i := start; i < end; i++ {
		rt := filtered[i]
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}
		label := fmt.Sprintf("%-24s :%s", rt.Name, rt.Command)
		b.WriteString(cursor + style.Render(label) + "\n")
	}

	if len(filtered) == 0 {
		b.WriteString("  No matching resources.\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate  enter: select  type to filter  esc: clear filter  :q: quit"))

	return b.String()
}
