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

// fileCopyDoneMsg is sent when a copy operation completes.
type fileCopyDoneMsg struct {
	err error
}

// fcPhase tracks the current step in the file copy flow.
type fcPhase int

const (
	fcPhaseDirection  fcPhase = iota // pick upload or download
	fcPhaseRemotePath                // type the remote (container) path
	fcPhaseLocalPath                 // type the local path
	fcPhaseCopying                   // copy in progress
	fcPhaseDone                      // show result
)

// fileCopyModel handles the multi-step file copy flow.
type fileCopyModel struct {
	client    *Client
	namespace string
	pod       string
	container string
	width     int
	height    int
	phase     fcPhase

	// Direction selection
	download bool // true = download from pod, false = upload to pod
	cursor   int  // 0 = download, 1 = upload

	// Path inputs
	remotePath    string
	localPath     string
	activeField   int // 0 = first field, 1 = second field
	pathInputErr  string

	// Result
	err     error
	success bool
}

func newFileCopyModel(client *Client, namespace, pod, container string, width, height int) fileCopyModel {
	return fileCopyModel{
		client:    client,
		namespace: namespace,
		pod:       pod,
		container: container,
		width:     width,
		height:    height,
		phase:     fcPhaseDirection,
		download:  true,
	}
}

func (m fileCopyModel) Init() tea.Cmd {
	return nil
}

func (m fileCopyModel) Update(msg tea.Msg) (fileCopyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case fileCopyDoneMsg:
		m.phase = fcPhaseDone
		m.err = msg.err
		m.success = msg.err == nil

	case tea.KeyMsg:
		switch m.phase {
		case fcPhaseDirection:
			return m.updateDirection(msg)
		case fcPhaseRemotePath:
			return m.updateRemotePath(msg)
		case fcPhaseLocalPath:
			return m.updateLocalPath(msg)
		case fcPhaseDone:
			// Any key goes back.
			return m, nil
		}
	}
	return m, nil
}

func (m fileCopyModel) updateDirection(msg tea.KeyMsg) (fileCopyModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.cursor < 1 {
			m.cursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		m.download = m.cursor == 0
		m.phase = fcPhaseRemotePath
		m.remotePath = ""
		m.localPath = ""
		m.pathInputErr = ""
		m.activeField = 0
	}
	return m, nil
}

func (m fileCopyModel) updateRemotePath(msg tea.KeyMsg) (fileCopyModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.phase = fcPhaseDirection
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if strings.TrimSpace(m.remotePath) == "" {
			m.pathInputErr = "Path cannot be empty"
			return m, nil
		}
		m.pathInputErr = ""
		m.phase = fcPhaseLocalPath
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.remotePath) > 0 {
			m.remotePath = m.remotePath[:len(m.remotePath)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			m.remotePath += ch
		}
	}
	return m, nil
}

func (m fileCopyModel) updateLocalPath(msg tea.KeyMsg) (fileCopyModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.phase = fcPhaseRemotePath
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if strings.TrimSpace(m.localPath) == "" {
			m.pathInputErr = "Path cannot be empty"
			return m, nil
		}
		m.pathInputErr = ""
		m.phase = fcPhaseCopying
		return m, m.executeCopy()
	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.localPath) > 0 {
			m.localPath = m.localPath[:len(m.localPath)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			m.localPath += ch
		}
	}
	return m, nil
}

func (m fileCopyModel) executeCopy() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if m.download {
			err = m.client.CopyFromPod(ctx, m.namespace, m.pod, m.container, m.remotePath, m.localPath)
		} else {
			err = m.client.CopyToPod(ctx, m.namespace, m.pod, m.container, m.localPath, m.remotePath)
		}
		return fileCopyDoneMsg{err: err}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m fileCopyModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf(" Copy: %s/%s [%s] ", m.namespace, m.pod, m.container)
	b.WriteString(viewportTitleStyle.Render(title) + "\n\n")

	switch m.phase {
	case fcPhaseDirection:
		m.viewDirection(&b)
	case fcPhaseRemotePath:
		m.viewRemotePath(&b)
	case fcPhaseLocalPath:
		m.viewLocalPath(&b)
	case fcPhaseCopying:
		b.WriteString(spinnerStyle.Render("Copying...") + "\n")
	case fcPhaseDone:
		m.viewDone(&b)
	}

	return b.String()
}

func (m fileCopyModel) viewDirection(b *strings.Builder) {
	b.WriteString("  Select direction:\n\n")

	options := []string{"Download (pod -> local)", "Upload (local -> pod)"}
	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}
		b.WriteString(cursor + style.Render(opt) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate  enter: select  esc: back"))
}

func (m fileCopyModel) viewRemotePath(b *strings.Builder) {
	if m.download {
		b.WriteString("  Download from pod\n\n")
		b.WriteString("  Remote path: " + m.remotePath + "\u2588\n")
	} else {
		b.WriteString("  Upload to pod\n\n")
		b.WriteString("  Remote directory: " + m.remotePath + "\u2588\n")
	}

	if m.pathInputErr != "" {
		b.WriteString("  " + errorStyle.Render(m.pathInputErr) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: next  esc: back"))
}

func (m fileCopyModel) viewLocalPath(b *strings.Builder) {
	if m.download {
		b.WriteString("  Download from pod\n\n")
		b.WriteString("  Remote path:      " + m.remotePath + "\n")
		b.WriteString("  Local directory:   " + m.localPath + "\u2588\n")
	} else {
		b.WriteString("  Upload to pod\n\n")
		b.WriteString("  Local path:        " + m.localPath + "\u2588\n")
		b.WriteString("  Remote directory:  " + m.remotePath + "\n")
	}

	if m.pathInputErr != "" {
		b.WriteString("  " + errorStyle.Render(m.pathInputErr) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: copy  esc: back"))
}

func (m fileCopyModel) viewDone(b *strings.Builder) {
	if m.success {
		direction := "Downloaded"
		if !m.download {
			direction = "Uploaded"
		}
		b.WriteString("  " + lipgloss.NewStyle().Foreground(successColor).Bold(true).Render(direction+" successfully") + "\n\n")
		if m.download {
			b.WriteString(fmt.Sprintf("  %s -> %s\n", m.remotePath, m.localPath))
		} else {
			b.WriteString(fmt.Sprintf("  %s -> %s\n", m.localPath, m.remotePath))
		}
	} else {
		b.WriteString("  " + errorStyle.Render(fmt.Sprintf("Copy failed: %v", m.err)) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc: back"))
}
