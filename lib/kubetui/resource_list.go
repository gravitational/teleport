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
)

// resourcesLoadedMsg is sent when the resource list has been fetched.
type resourcesLoadedMsg struct {
	resources []Resource
	err       error
}

// resourceActionMsg is sent when the user selects an action on a resource.
type resourceActionMsg struct {
	action       string
	namespace    string
	name         string
	resourceType *ResourceType
}

// backToPickerMsg is sent when the user presses Esc from the resource list.
type backToPickerMsg struct{}

// resourceListModel renders a table of resources.
type resourceListModel struct {
	client          *Client
	resourceType    *ResourceType
	resources       []Resource
	cursor          int
	namespace       string
	filter          string
	filtering       bool
	width           int
	height          int
	err             error
	loading         bool
	disabledActions map[string]bool

	showActions  bool
	actionCursor int
}

func newResourceListModel(client *Client, rt *ResourceType) resourceListModel {
	return resourceListModel{
		client:       client,
		resourceType: rt,
		loading:      true,
	}
}

func (m resourceListModel) effectiveActions() []string {
	if len(m.disabledActions) == 0 {
		return m.resourceType.Actions
	}
	var actions []string
	for _, a := range m.resourceType.Actions {
		if !m.disabledActions[a] {
			actions = append(actions, a)
		}
	}
	return actions
}

func (m resourceListModel) Init() tea.Cmd {
	return m.fetchResources
}

func (m resourceListModel) fetchResources() tea.Msg {
	resources, err := m.resourceType.FetchFunc(context.Background(), m.client, m.namespace)
	return resourcesLoadedMsg{resources: resources, err: err}
}

func (m resourceListModel) filteredResources() []Resource {
	if m.filter == "" {
		return m.resources
	}
	var result []Resource
	lower := strings.ToLower(m.filter)
	for _, r := range m.resources {
		if strings.Contains(strings.ToLower(r.Name), lower) ||
			strings.Contains(strings.ToLower(r.Namespace), lower) {
			result = append(result, r)
		}
	}
	return result
}

func (m resourceListModel) Update(msg tea.Msg) (resourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case resourcesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.resources = msg.resources
		m.cursor = 0
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		if m.showActions {
			return m.updateActions(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m resourceListModel) updateList(msg tea.KeyMsg) (resourceListModel, tea.Cmd) {
	filtered := m.filteredResources()
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
		m.filtering = true
		m.filter = ""
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if len(filtered) > 0 && m.cursor < len(filtered) {
			m.showActions = true
			m.actionCursor = 0
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		m.loading = true
		return m, m.fetchResources
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		if m.filter != "" {
			m.filter = ""
			m.cursor = 0
		} else {
			return m, func() tea.Msg { return backToPickerMsg{} }
		}
	}
	return m, nil
}

func (m resourceListModel) updateActions(msg tea.KeyMsg) (resourceListModel, tea.Cmd) {
	actions := m.effectiveActions()
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.actionCursor < len(actions)-1 {
			m.actionCursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.showActions = false
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		filtered := m.filteredResources()
		if m.cursor < len(filtered) {
			r := filtered[m.cursor]
			m.showActions = false
			return m, func() tea.Msg {
				return resourceActionMsg{
					action:       actions[m.actionCursor],
					namespace:    r.Namespace,
					name:         r.Name,
					resourceType: m.resourceType,
				}
			}
		}
	}
	return m, nil
}

func (m resourceListModel) updateFilter(msg tea.KeyMsg) (resourceListModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.filtering = false
		m.filter = ""
		m.cursor = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		m.filtering = false
		m.cursor = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	default:
		if len(msg.String()) == 1 && msg.String() >= " " {
			m.filter += msg.String()
		}
	}
	return m, nil
}

func (m resourceListModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error loading %s: %v", strings.ToLower(m.resourceType.Name), m.err))
	}
	if m.loading {
		return spinnerStyle.Render(fmt.Sprintf("Loading %s...", strings.ToLower(m.resourceType.Name)))
	}

	var b strings.Builder
	filtered := m.filteredResources()

	// Header
	nsDisplay := m.namespace
	if nsDisplay == "" {
		nsDisplay = "all"
	}
	header := fmt.Sprintf(" %s [%s] (%d) ", m.resourceType.Name, nsDisplay, len(filtered))
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	if m.filtering {
		b.WriteString(filterPromptStyle.Render("/") + m.filter + "\u2588\n\n")
	} else if m.filter != "" {
		b.WriteString(filterPromptStyle.Render("filter: ") + m.filter + "\n\n")
	}

	// Calculate column widths
	colWidths := m.calculateColumnWidths()

	// Table header
	headerLine := m.renderHeaderLine(colWidths)
	b.WriteString(tableHeaderStyle.Render(headerLine) + "\n")

	// Table rows
	maxVisible := m.height - 8
	if m.filtering || m.filter != "" {
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
		r := filtered[i]
		row := m.renderRow(r, colWidths)

		if i == m.cursor {
			row = selectedRowStyle.Width(m.width).Render(row)
		} else if m.resourceType.RowStyleFunc != nil {
			row = m.resourceType.RowStyleFunc(r).Render(row)
		}
		b.WriteString(row + "\n")
	}

	if len(filtered) == 0 {
		b.WriteString(fmt.Sprintf("\n  No %s found.\n", strings.ToLower(m.resourceType.Name)))
	}

	// Action menu overlay
	if m.showActions {
		b.WriteString("\n")
		if m.cursor < len(filtered) {
			r := filtered[m.cursor]
			actionTitle := r.Name
			if r.Namespace != "" {
				actionTitle = r.Namespace + "/" + r.Name
			}
			b.WriteString(viewportTitleStyle.Render(fmt.Sprintf(" Actions: %s ", actionTitle)) + "\n")
		}
		for i, action := range m.effectiveActions() {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.actionCursor {
				cursor = "> "
				style = selectedRowStyle
			}
			b.WriteString(cursor + style.Render(action) + "\n")
		}
	}

	// Help
	b.WriteString("\n")
	if m.showActions {
		b.WriteString(helpStyle.Render("j/k: navigate  enter: select  esc: back"))
	} else {
		b.WriteString(helpStyle.Render("j/k: navigate  enter: actions  /: filter  r: refresh  :ns: namespaces  esc: resources  :q: quit"))
	}

	return b.String()
}

func (m resourceListModel) calculateColumnWidths() []int {
	cols := m.resourceType.Columns
	widths := make([]int, len(cols))
	flexIdx := -1
	usedWidth := 0

	for i, col := range cols {
		if col.Flexible {
			flexIdx = i
		} else {
			widths[i] = col.Width
			usedWidth += col.Width
		}
	}

	// Add spacing between columns
	usedWidth += (len(cols) - 1) * 1 // 1 space between each column

	if flexIdx >= 0 {
		flexWidth := m.width - usedWidth
		if flexWidth < 10 {
			flexWidth = 10
		}
		widths[flexIdx] = flexWidth
	}

	return widths
}

func (m resourceListModel) renderHeaderLine(colWidths []int) string {
	cols := m.resourceType.Columns
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = fmt.Sprintf("%-*s", colWidths[i], col.Header)
	}
	return strings.Join(parts, " ")
}

func (m resourceListModel) renderRow(r Resource, colWidths []int) string {
	cols := m.resourceType.Columns
	parts := make([]string, len(cols))

	if m.resourceType.Namespaced {
		// Column 0 = Namespace, Column 1 = Name, Column 2+ = Cells[i-2]
		for i := range cols {
			var val string
			switch i {
			case 0:
				val = truncate(r.Namespace, colWidths[i])
			case 1:
				val = truncate(r.Name, colWidths[i])
			default:
				cellIdx := i - 2
				if cellIdx < len(r.Cells) {
					val = truncate(r.Cells[cellIdx], colWidths[i])
				}
			}
			parts[i] = fmt.Sprintf("%-*s", colWidths[i], val)
		}
	} else {
		// Cluster-scoped: Column 0 = Name, Column 1+ = Cells[i-1]
		for i := range cols {
			var val string
			if i == 0 {
				val = truncate(r.Name, colWidths[i])
			} else {
				cellIdx := i - 1
				if cellIdx < len(r.Cells) {
					val = truncate(r.Cells[cellIdx], colWidths[i])
				}
			}
			parts[i] = fmt.Sprintf("%-*s", colWidths[i], val)
		}
	}
	return strings.Join(parts, " ")
}
