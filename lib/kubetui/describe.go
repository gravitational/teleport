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

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// resourceDescribedMsg is sent when resource details have been fetched.
type resourceDescribedMsg struct {
	content string
	err     error
}

// describeModel renders detailed information about a resource.
type describeModel struct {
	client       *Client
	resourceType *ResourceType
	namespace    string
	name         string
	content      string
	viewport     viewport.Model
	width        int
	height       int
	err          error
	ready        bool

	// fetchOverride, when set, replaces DescribeFunc for fetching content.
	fetchOverride func(ctx context.Context, client *Client, namespace, name string) (string, error)
	// title overrides "Describe" in the header when set.
	title string
}

func newDescribeModel(client *Client, rt *ResourceType, namespace, name string, width, height int) describeModel {
	headerHeight := 2
	footerHeight := 2
	vpHeight := height - headerHeight - footerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	vp := viewport.New(width, vpHeight)
	return describeModel{
		client:       client,
		resourceType: rt,
		namespace:    namespace,
		name:         name,
		viewport:     vp,
		width:        width,
		height:       height,
		ready:        true,
	}
}

func (m describeModel) Init() tea.Cmd {
	return m.fetchResource
}

func (m describeModel) fetchResource() tea.Msg {
	fetchFn := m.resourceType.DescribeFunc
	if m.fetchOverride != nil {
		fetchFn = m.fetchOverride
	}
	content, err := fetchFn(context.Background(), m.client, m.namespace, m.name)
	return resourceDescribedMsg{content: content, err: err}
}

// newContentViewModel creates a describeModel that uses a custom fetch function
// and title, reusing the same read-only viewport for displaying content.
func newContentViewModel(client *Client, rt *ResourceType, namespace, name string, width, height int, title string, fetchFn func(ctx context.Context, client *Client, namespace, name string) (string, error)) describeModel {
	dm := newDescribeModel(client, rt, namespace, name, width, height)
	dm.fetchOverride = fetchFn
	dm.title = title
	return dm
}

func (m describeModel) Update(msg tea.Msg) (describeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		headerHeight := 2
		footerHeight := 2
		m.height = msg.Height - headerHeight - footerHeight
		if !m.ready {
			m.viewport = viewport.New(m.width, m.height)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = m.height
		}
		if m.content != "" {
			m.viewport.SetContent(m.content)
		}

	case resourceDescribedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.content = msg.content
		if m.ready {
			m.viewport.SetContent(m.content)
		}

	case tea.KeyMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m describeModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error describing %s: %v", strings.ToLower(m.resourceType.Name), m.err))
	}
	if m.content == "" {
		return spinnerStyle.Render(fmt.Sprintf("Loading %s details...", strings.ToLower(m.resourceType.Name)))
	}

	var b strings.Builder
	verb := "Describe"
	if m.title != "" {
		verb = m.title
	}
	var title string
	if m.namespace != "" {
		title = fmt.Sprintf(" %s %s: %s/%s ", verb, m.resourceType.Name, m.namespace, m.name)
	} else {
		title = fmt.Sprintf(" %s %s: %s ", verb, m.resourceType.Name, m.name)
	}
	b.WriteString(viewportTitleStyle.Render(title) + "\n")

	if m.ready {
		b.WriteString(m.viewport.View() + "\n")
	}

	b.WriteString(helpStyle.Render("esc: back  \u2191/\u2193: scroll  pgup/pgdn: page"))

	return b.String()
}
