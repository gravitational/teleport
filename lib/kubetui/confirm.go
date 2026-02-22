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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// resourceDeletedMsg is sent when a delete operation completes.
type resourceDeletedMsg struct {
	err error
}

// confirmModel presents a y/n confirmation prompt before deleting a resource.
type confirmModel struct {
	client       *Client
	resourceType *ResourceType
	namespace    string
	name         string
	width        int
	height       int
	deleting     bool
	err          error
}

func newConfirmModel(client *Client, rt *ResourceType, namespace, name string, width, height int) confirmModel {
	return confirmModel{
		client:       client,
		resourceType: rt,
		namespace:    namespace,
		name:         name,
		width:        width,
		height:       height,
	}
}

func (m confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case resourceDeletedMsg:
		m.deleting = false
		if msg.err != nil {
			m.err = msg.err
		}

	case tea.KeyMsg:
		if m.err != nil {
			// After an error, any key returns to resource list (handled by esc in model.go).
			return m, nil
		}
		if m.deleting {
			return m, nil
		}

		switch msg.String() {
		case "y", "Y", "enter":
			m.deleting = true
			return m, m.performDelete
		}
	}

	return m, nil
}

func (m confirmModel) performDelete() tea.Msg {
	err := m.resourceType.DeleteFunc(context.Background(), m.client, m.namespace, m.name)
	return resourceDeletedMsg{err: err}
}

func (m confirmModel) View() string {
	var b strings.Builder

	var title string
	if m.namespace != "" {
		title = fmt.Sprintf(" Delete %s: %s/%s ", m.resourceType.Name, m.namespace, m.name)
	} else {
		title = fmt.Sprintf(" Delete %s: %s ", m.resourceType.Name, m.name)
	}
	b.WriteString(viewportTitleStyle.Render(title) + "\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
		b.WriteString(helpStyle.Render("esc: back"))
		return b.String()
	}

	if m.deleting {
		b.WriteString(spinnerStyle.Render("Deleting..."))
		return b.String()
	}

	prompt := fmt.Sprintf("Are you sure you want to delete %s", m.name)
	if m.namespace != "" {
		prompt += fmt.Sprintf(" in namespace %s", m.namespace)
	}
	prompt += "?"

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(warningColor).Render(prompt) + "\n\n")
	b.WriteString(helpStyle.Render("y/enter: confirm  n/esc: cancel"))

	return b.String()
}
