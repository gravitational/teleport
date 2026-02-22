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
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	v1 "k8s.io/api/core/v1"
)

// namespacesLoadedMsg is sent when the namespace list has been fetched.
type namespacesLoadedMsg struct {
	namespaces []v1.Namespace
	err        error
}

// namespaceModel handles namespace selection.
type namespaceModel struct {
	client     *Client
	namespaces []string
	cursor     int
	selected   string
	filter     string
	width      int
	height     int
	err        error
}

func newNamespaceModel(client *Client, currentNamespace string) namespaceModel {
	return namespaceModel{
		client:   client,
		selected: currentNamespace,
	}
}

func (m namespaceModel) Init() tea.Cmd {
	return m.fetchNamespaces
}

func (m namespaceModel) fetchNamespaces() tea.Msg {
	namespaces, err := m.client.ListNamespaces(context.Background())
	return namespacesLoadedMsg{namespaces: namespaces, err: err}
}

func (m namespaceModel) Update(msg tea.Msg) (namespaceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case namespacesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.namespaces = make([]string, 0, len(msg.namespaces)+1)
		m.namespaces = append(m.namespaces, "") // all namespaces
		for _, ns := range msg.namespaces {
			m.namespaces = append(m.namespaces, ns.Name)
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			filtered := m.filteredNamespaces()
			if m.cursor < len(filtered)-1 {
				m.cursor++
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

func (m namespaceModel) filteredNamespaces() []string {
	if m.filter == "" {
		return m.namespaces
	}
	var result []string
	for _, ns := range m.namespaces {
		if strings.Contains(ns, m.filter) {
			result = append(result, ns)
		}
	}
	return result
}

// selectedNamespace returns the namespace under the cursor.
func (m namespaceModel) selectedNamespace() string {
	filtered := m.filteredNamespaces()
	if m.cursor >= 0 && m.cursor < len(filtered) {
		return filtered[m.cursor]
	}
	return m.selected
}

func (m namespaceModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error loading namespaces: %v", m.err))
	}

	if len(m.namespaces) == 0 {
		return spinnerStyle.Render("Loading namespaces...")
	}

	var b strings.Builder

	title := viewportTitleStyle.Render(" Namespace Selector ")
	b.WriteString(title + "\n\n")

	if m.filter != "" {
		b.WriteString(filterPromptStyle.Render("Filter: ") + m.filter + "\n\n")
	}

	filtered := m.filteredNamespaces()
	maxVisible := m.height - 6
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
		ns := filtered[i]
		display := ns
		if display == "" {
			display = "<all namespaces>"
		}

		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}
		if ns == m.selected {
			display += " *"
		}
		b.WriteString(cursor + style.Render(display) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("j/k: navigate  enter: select  esc: cancel  type to filter"))

	return b.String()
}
