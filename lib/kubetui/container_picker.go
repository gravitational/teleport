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

// containerPickerAction identifies which action triggered the container picker.
type containerPickerAction int

const (
	containerPickerExec containerPickerAction = iota
	containerPickerLogs
	containerPickerCopy
)

// containersLoadedMsg is sent when the container list has been fetched.
type containersLoadedMsg struct {
	containers []containerInfo
	err        error
}

// containerSelectedMsg is sent when the user picks a container.
type containerSelectedMsg struct {
	action    containerPickerAction
	namespace string
	pod       string
	container string
}

// containerInfo describes a container in a pod.
type containerInfo struct {
	name  string
	image string
	ready bool
}

func (c containerInfo) String() string {
	status := "ready"
	if !c.ready {
		status = "not ready"
	}
	return fmt.Sprintf("%s  %s  (%s)", c.name, lipgloss.NewStyle().Foreground(dimColor).Render(c.image), status)
}

// containerPickerModel shows a list of containers in a pod and lets the user
// pick one. If the pod has only one container, it auto-selects immediately.
type containerPickerModel struct {
	client    *Client
	action    containerPickerAction
	namespace string
	pod       string
	width     int
	height    int

	containers []containerInfo
	cursor     int
	loading    bool
	err        error
}

func newContainerPickerModel(client *Client, action containerPickerAction, namespace, pod string, width, height int) containerPickerModel {
	return containerPickerModel{
		client:    client,
		action:    action,
		namespace: namespace,
		pod:       pod,
		width:     width,
		height:    height,
		loading:   true,
	}
}

func (m containerPickerModel) Init() tea.Cmd {
	return m.fetchContainers()
}

func (m containerPickerModel) fetchContainers() tea.Cmd {
	return func() tea.Msg {
		pod, err := m.client.DescribePod(context.Background(), m.namespace, m.pod)
		if err != nil {
			return containersLoadedMsg{err: err}
		}

		statusMap := make(map[string]bool)
		for _, cs := range pod.Status.ContainerStatuses {
			statusMap[cs.Name] = cs.Ready
		}

		var containers []containerInfo
		for _, c := range pod.Spec.Containers {
			containers = append(containers, containerInfo{
				name:  c.Name,
				image: c.Image,
				ready: statusMap[c.Name],
			})
		}
		return containersLoadedMsg{containers: containers}
	}
}

func (m containerPickerModel) Update(msg tea.Msg) (containerPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case containersLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.containers = msg.containers
		m.cursor = 0
		// Auto-select if only one container.
		if len(m.containers) == 1 {
			return m, m.selectContainer(m.containers[0].name)
		}

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m containerPickerModel) handleKey(msg tea.KeyMsg) (containerPickerModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.cursor < len(m.containers)-1 {
			m.cursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if m.cursor < len(m.containers) {
			return m, m.selectContainer(m.containers[m.cursor].name)
		}
	}
	return m, nil
}

func (m containerPickerModel) selectContainer(name string) tea.Cmd {
	return func() tea.Msg {
		return containerSelectedMsg{
			action:    m.action,
			namespace: m.namespace,
			pod:       m.pod,
			container: name,
		}
	}
}

func (m containerPickerModel) View() string {
	var b strings.Builder

	var actionLabel string
	switch m.action {
	case containerPickerExec:
		actionLabel = "Exec"
	case containerPickerLogs:
		actionLabel = "Logs"
	case containerPickerCopy:
		actionLabel = "Copy"
	}
	title := fmt.Sprintf(" %s: %s/%s — Select Container ", actionLabel, m.namespace, m.pod)
	b.WriteString(viewportTitleStyle.Render(title) + "\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
		b.WriteString(helpStyle.Render("esc: back"))
		return b.String()
	}

	if m.loading {
		b.WriteString(spinnerStyle.Render("Loading containers...") + "\n")
		return b.String()
	}

	if len(m.containers) == 0 {
		b.WriteString("  (no containers found)\n\n")
		b.WriteString(helpStyle.Render("esc: back"))
		return b.String()
	}

	for i, c := range m.containers {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}
		b.WriteString(cursor + style.Render(c.String()) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate  enter: select  esc: back"))

	return b.String()
}
