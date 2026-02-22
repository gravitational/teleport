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

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// resourceYAMLLoadedMsg is sent when the YAML for a resource has been fetched.
type resourceYAMLLoadedMsg struct {
	yaml []byte
	err  error
}

// resourceSavedMsg is sent when a resource update completes.
type resourceSavedMsg struct {
	err error
}

// editModel provides a YAML editor for Kubernetes resources.
type editModel struct {
	client       *Client
	resourceType *ResourceType
	namespace    string
	name         string
	textarea     textarea.Model
	width        int
	height       int
	loading      bool
	saving       bool
	err          error
	feedback     string
	feedbackIsErr bool
}

func newEditModel(client *Client, rt *ResourceType, namespace, name string, width, height int) editModel {
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.Focus()

	// Height = total height minus title line, blank line, feedback line, help line
	taHeight := height - 4
	if taHeight < 1 {
		taHeight = 1
	}
	ta.SetWidth(width)
	ta.SetHeight(taHeight)

	return editModel{
		client:       client,
		resourceType: rt,
		namespace:    namespace,
		name:         name,
		textarea:     ta,
		width:        width,
		height:       height,
		loading:      true,
	}
}

func (m editModel) Init() tea.Cmd {
	return m.fetchYAML
}

func (m editModel) fetchYAML() tea.Msg {
	data, err := m.resourceType.GetYAMLFunc(context.Background(), m.client, m.namespace, m.name)
	return resourceYAMLLoadedMsg{yaml: data, err: err}
}

func (m editModel) saveYAML() tea.Msg {
	data := []byte(m.textarea.Value())
	err := m.resourceType.UpdateFunc(context.Background(), m.client, m.namespace, m.name, data)
	return resourceSavedMsg{err: err}
}

func (m editModel) Update(msg tea.Msg) (editModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		taHeight := m.height - 4
		if taHeight < 1 {
			taHeight = 1
		}
		m.textarea.SetWidth(m.width)
		m.textarea.SetHeight(taHeight)

	case resourceYAMLLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.textarea.SetValue(string(msg.yaml))
		m.textarea.CursorStart()

	case resourceSavedMsg:
		m.saving = false
		if msg.err != nil {
			m.feedback = fmt.Sprintf("Error: %v", msg.err)
			m.feedbackIsErr = true
		} else {
			m.feedback = "Saved successfully."
			m.feedbackIsErr = false
		}

	case tea.KeyMsg:
		// Clear feedback on any keypress
		if m.feedback != "" {
			m.feedback = ""
		}

		switch msg.String() {
		case "ctrl+s":
			if m.saving || m.loading {
				return m, nil
			}
			m.saving = true
			m.feedback = "Saving..."
			m.feedbackIsErr = false
			return m, m.saveYAML
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m editModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error loading YAML: %v", m.err))
	}
	if m.loading {
		return spinnerStyle.Render(fmt.Sprintf("Loading %s YAML...", strings.ToLower(m.resourceType.Name)))
	}

	var b strings.Builder

	var title string
	if m.namespace != "" {
		title = fmt.Sprintf(" Edit %s: %s/%s ", m.resourceType.Name, m.namespace, m.name)
	} else {
		title = fmt.Sprintf(" Edit %s: %s ", m.resourceType.Name, m.name)
	}
	b.WriteString(viewportTitleStyle.Render(title) + "\n")

	b.WriteString(m.textarea.View() + "\n")

	// Feedback line
	if m.feedback != "" {
		if m.feedbackIsErr {
			b.WriteString(errorStyle.Render(m.feedback))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(successColor).Render(m.feedback))
		}
	}
	b.WriteString("\n")

	b.WriteString(helpStyle.Render("ctrl+s: save  esc: cancel"))

	return b.String()
}
