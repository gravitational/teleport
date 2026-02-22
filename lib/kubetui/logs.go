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
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// logLineMsg carries a single line of log output along with references to the
// stream and scanner so we can continue reading.
type logLineMsg struct {
	line    string
	stream  io.ReadCloser
	scanner *bufio.Scanner
}

// logErrorMsg is sent when log streaming encounters an error.
type logErrorMsg struct {
	err error
}

// logDoneMsg is sent when the log stream ends.
type logDoneMsg struct{}

// logModel handles streaming container logs.
type logModel struct {
	client    *Client
	namespace string
	pod       string
	container string
	viewport  viewport.Model
	lines     []string
	follow    bool
	width     int
	height    int
	err       error
	cancel    context.CancelFunc
	ready     bool
}

func newLogModel(client *Client, namespace, pod, container string, width, height int) logModel {
	headerHeight := 3
	footerHeight := 2
	vpHeight := max(height-headerHeight-footerHeight, 1)
	vp := viewport.New(width, vpHeight)
	return logModel{
		client:    client,
		namespace: namespace,
		pod:       pod,
		container: container,
		follow:    true,
		viewport:  vp,
		width:     width,
		height:    height,
		ready:     true,
	}
}

// streamLogs returns a command that opens the log stream and reads the first line.
// Subsequent lines are read via continueStreaming, chained from Update.
func (m *logModel) streamLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel

		stream, err := m.client.GetPodLogs(ctx, m.namespace, m.pod, m.container, m.follow)
		if err != nil {
			return logErrorMsg{err: err}
		}

		scanner := bufio.NewScanner(stream)
		if scanner.Scan() {
			return logLineMsg{line: scanner.Text(), stream: stream, scanner: scanner}
		}
		if err := scanner.Err(); err != nil {
			return logErrorMsg{err: err}
		}

		stream.Close()
		return logDoneMsg{}
	}
}

// continueStreaming returns a command that reads the next log line
// from an already-open stream.
func continueStreaming(stream io.ReadCloser, scanner *bufio.Scanner) tea.Cmd {
	return func() tea.Msg {
		if scanner.Scan() {
			return logLineMsg{line: scanner.Text(), stream: stream, scanner: scanner}
		}
		if err := scanner.Err(); err != nil {
			return logErrorMsg{err: err}
		}
		stream.Close()
		return logDoneMsg{}
	}
}

func (m logModel) Update(msg tea.Msg) (logModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		headerHeight := 3
		footerHeight := 2
		m.height = msg.Height - headerHeight - footerHeight
		if !m.ready {
			m.viewport = viewport.New(m.width, m.height)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = m.height
		}

	case logLineMsg:
		m.lines = append(m.lines, msg.line)
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		if m.follow {
			m.viewport.GotoBottom()
		}
		// Continue reading the next line from the same stream.
		return m, continueStreaming(msg.stream, msg.scanner)

	case logErrorMsg:
		m.err = msg.err

	case logDoneMsg:
		// Stream ended
		m.lines = append(m.lines, "--- log stream ended ---")
		m.viewport.SetContent(strings.Join(m.lines, "\n"))

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
			m.follow = !m.follow
			if m.follow {
				m.viewport.GotoBottom()
			}
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m logModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error streaming logs: %v", m.err))
	}

	var b strings.Builder

	title := fmt.Sprintf(" Logs: %s/%s ", m.namespace, m.pod)
	if m.container != "" {
		title = fmt.Sprintf(" Logs: %s/%s [%s] ", m.namespace, m.pod, m.container)
	}
	b.WriteString(viewportTitleStyle.Render(title) + "\n")

	followIndicator := " FOLLOW"
	if !m.follow {
		followIndicator = " PAUSED"
	}
	b.WriteString(statusBarActiveStyle.Render(followIndicator) + "\n")

	if m.ready {
		b.WriteString(m.viewport.View() + "\n")
	} else {
		b.WriteString(spinnerStyle.Render("Loading logs...") + "\n")
	}

	b.WriteString(helpStyle.Render("esc: back  f: toggle follow  ↑/↓: scroll  pgup/pgdn: page"))

	return b.String()
}

func (m logModel) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
}
