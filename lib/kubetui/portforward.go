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
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// containerPortInfo describes a single container port from a pod spec.
type containerPortInfo struct {
	container string
	port      int32
	protocol  string
	name      string
}

func (p containerPortInfo) String() string {
	s := fmt.Sprintf("%s: %d/%s", p.container, p.port, p.protocol)
	if p.name != "" {
		s += fmt.Sprintf(" (%s)", p.name)
	}
	return s
}

// containerPortsLoadedMsg is sent when the pod's container ports have been fetched.
type containerPortsLoadedMsg struct {
	ports []containerPortInfo
	err   error
}

// portForwardStartedMsg is sent when port-forwarding has started.
type portForwardStartedMsg struct {
	localPort uint16
	stopCh    chan struct{}
	err       error
}

// pfPhase tracks where we are in the port-forward flow.
type pfPhase int

const (
	pfPhaseSelectPort pfPhase = iota // pick a declared port or "Custom port..."
	pfPhaseCustomPort                // typing a custom container port
	pfPhaseLocalPort                 // typing a local port (blank = random)
	pfPhaseForwarding                // port-forward is active
)

// portForwardModel handles port selection and active forwarding status.
type portForwardModel struct {
	client    *Client
	namespace string
	pod       string
	width     int
	height    int
	phase     pfPhase

	// Port selection
	ports   []containerPortInfo
	cursor  int
	loading bool
	err     error

	// Custom container port text input
	customPort    string
	customPortErr string

	// Local port text input (empty = random)
	localPortInput    string
	localPortInputErr string

	// Active forwarding
	localPort  uint16
	remotePort int32
	stopCh     chan struct{}
}

func newPortForwardModel(client *Client, namespace, pod string, width, height int) portForwardModel {
	return portForwardModel{
		client:    client,
		namespace: namespace,
		pod:       pod,
		width:     width,
		height:    height,
		loading:   true,
		phase:     pfPhaseSelectPort,
	}
}

func (m portForwardModel) Init() tea.Cmd {
	return m.fetchPorts()
}

func (m portForwardModel) fetchPorts() tea.Cmd {
	return func() tea.Msg {
		pod, err := m.client.DescribePod(context.Background(), m.namespace, m.pod)
		if err != nil {
			return containerPortsLoadedMsg{err: err}
		}

		var ports []containerPortInfo
		for _, c := range pod.Spec.Containers {
			for _, p := range c.Ports {
				ports = append(ports, containerPortInfo{
					container: c.Name,
					port:      p.ContainerPort,
					protocol:  string(p.Protocol),
					name:      p.Name,
				})
			}
		}
		return containerPortsLoadedMsg{ports: ports}
	}
}

func (m portForwardModel) startForwarding(localPort, remotePort uint16) tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.PortForward(m.namespace, m.pod, localPort, remotePort)
		if err != nil {
			return portForwardStartedMsg{err: err}
		}
		return portForwardStartedMsg{localPort: result.LocalPort, stopCh: result.StopCh}
	}
}

// menuLen returns the number of items in the port selection menu:
// declared ports + 1 for "Custom port...".
func (m portForwardModel) menuLen() int {
	return len(m.ports) + 1
}

func (m portForwardModel) Update(msg tea.Msg) (portForwardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case containerPortsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.ports = msg.ports
		m.cursor = 0
		if len(m.ports) == 0 {
			m.phase = pfPhaseCustomPort
		}

	case portForwardStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = pfPhaseLocalPort
			return m, nil
		}
		m.localPort = msg.localPort
		m.stopCh = msg.stopCh

	case tea.KeyMsg:
		switch m.phase {
		case pfPhaseForwarding:
			return m, nil
		case pfPhaseSelectPort:
			return m.updatePortSelection(msg)
		case pfPhaseCustomPort:
			return m.updateCustomPortInput(msg)
		case pfPhaseLocalPort:
			return m.updateLocalPortInput(msg)
		}
	}
	return m, nil
}

func (m portForwardModel) updatePortSelection(msg tea.KeyMsg) (portForwardModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.cursor < m.menuLen()-1 {
			m.cursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if m.cursor < len(m.ports) {
			m.remotePort = m.ports[m.cursor].port
			m.phase = pfPhaseLocalPort
			m.localPortInput = ""
			m.localPortInputErr = ""
			m.err = nil
			return m, nil
		}
		// "Custom port..."
		m.phase = pfPhaseCustomPort
		m.customPort = ""
		m.customPortErr = ""
	}
	return m, nil
}

func (m portForwardModel) updateCustomPortInput(msg tea.KeyMsg) (portForwardModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		if len(m.ports) > 0 {
			m.phase = pfPhaseSelectPort
			m.customPort = ""
			m.customPortErr = ""
			return m, nil
		}
		// No declared ports — esc propagates to parent.
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		port, err := strconv.Atoi(m.customPort)
		if err != nil || port < 1 || port > 65535 {
			m.customPortErr = "Enter a port between 1 and 65535"
			return m, nil
		}
		m.remotePort = int32(port)
		m.phase = pfPhaseLocalPort
		m.localPortInput = ""
		m.localPortInputErr = ""
		m.err = nil
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.customPort) > 0 {
			m.customPort = m.customPort[:len(m.customPort)-1]
		}

	default:
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= '0' && ch[0] <= '9' {
			m.customPort += ch
		}
	}
	return m, nil
}

func (m portForwardModel) updateLocalPortInput(msg tea.KeyMsg) (portForwardModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Go back to port selection (or custom input if no declared ports).
		if len(m.ports) > 0 {
			m.phase = pfPhaseSelectPort
		} else {
			m.phase = pfPhaseCustomPort
		}
		m.localPortInput = ""
		m.localPortInputErr = ""
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		var local uint16
		if m.localPortInput != "" {
			port, err := strconv.Atoi(m.localPortInput)
			if err != nil || port < 1 || port > 65535 {
				m.localPortInputErr = "Enter a port between 1 and 65535, or leave blank for random"
				return m, nil
			}
			local = uint16(port)
		}
		m.localPortInputErr = ""
		m.phase = pfPhaseForwarding
		m.err = nil
		return m, m.startForwarding(local, uint16(m.remotePort))

	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.localPortInput) > 0 {
			m.localPortInput = m.localPortInput[:len(m.localPortInput)-1]
		}

	default:
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= '0' && ch[0] <= '9' {
			m.localPortInput += ch
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m portForwardModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf(" Port Forward: %s/%s ", m.namespace, m.pod)
	b.WriteString(viewportTitleStyle.Render(title) + "\n\n")

	if m.err != nil && m.phase != pfPhaseLocalPort {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
		b.WriteString(helpStyle.Render("esc: back"))
		return b.String()
	}

	if m.loading {
		b.WriteString(spinnerStyle.Render("Loading container ports...") + "\n")
		return b.String()
	}

	switch m.phase {
	case pfPhaseSelectPort:
		m.viewPortSelection(&b)
	case pfPhaseCustomPort:
		m.viewCustomPortInput(&b)
	case pfPhaseLocalPort:
		m.viewLocalPortInput(&b)
	case pfPhaseForwarding:
		m.viewForwarding(&b)
	}

	return b.String()
}

func (m portForwardModel) viewPortSelection(b *strings.Builder) {
	b.WriteString("  Select a port to forward:\n\n")
	for i, p := range m.ports {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}
		b.WriteString(cursor + style.Render(p.String()) + "\n")
	}

	cursor := "  "
	style := lipgloss.NewStyle().Foreground(dimColor)
	if m.cursor == len(m.ports) {
		cursor = "> "
		style = selectedRowStyle
	}
	b.WriteString(cursor + style.Render("Custom port...") + "\n")

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate  enter: select  esc: back"))
}

func (m portForwardModel) viewCustomPortInput(b *strings.Builder) {
	b.WriteString("  Container port: " + m.customPort + "\u2588\n")

	if m.customPortErr != "" {
		b.WriteString("  " + errorStyle.Render(m.customPortErr) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: next  esc: back"))
}

func (m portForwardModel) viewLocalPortInput(b *strings.Builder) {
	b.WriteString(fmt.Sprintf("  Container port: %d\n", m.remotePort))
	b.WriteString("  Local port:     " + m.localPortInput + "\u2588")
	b.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render("  (blank = random)") + "\n")

	if m.localPortInputErr != "" {
		b.WriteString("  " + errorStyle.Render(m.localPortInputErr) + "\n")
	}
	if m.err != nil {
		b.WriteString("  " + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: forward  esc: back"))
}

func (m portForwardModel) viewForwarding(b *strings.Builder) {
	if m.localPort == 0 {
		b.WriteString(spinnerStyle.Render("Starting port forward...") + "\n")
		return
	}

	b.WriteString(statusBarActiveStyle.Render(" ACTIVE ") + "\n\n")
	b.WriteString(fmt.Sprintf("  Forwarding 127.0.0.1:%d -> %s:%d\n\n", m.localPort, m.pod, m.remotePort))
	b.WriteString(helpStyle.Render("esc: stop and return"))
}

func (m portForwardModel) cleanup() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}
