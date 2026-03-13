// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package debuglive

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

const (
	tabLogs    = 0
	tabMetrics = 1
	tabProfile = 2
	numTabs    = 3
)

var tabNames = [numTabs]string{"Logs", "Metrics", "Profile"}

// detailModel is the right-pane tab container.
type detailModel struct {
	activeTab int
	logs      logsModel
	metrics   metricsModel
	profile   profileModel
	client    *debugclient.Client
	cleanup   func()
	serverID  string
	hostname  string
	connected bool
	width     int
	height    int
	err       error
}

func newDetailModel() detailModel {
	return detailModel{
		logs:    newLogsModel(),
		metrics: newMetricsModel(),
		profile: newProfileModel(),
	}
}

// setClient updates the debug client for all tabs.
func (m *detailModel) setClient(client *debugclient.Client, cleanup func(), serverID, hostname string) {
	// Clean up previous connection.
	if m.cleanup != nil {
		m.cleanup()
	}
	m.client = client
	m.cleanup = cleanup
	m.serverID = serverID
	m.hostname = hostname
	m.connected = true
	m.err = nil

	// Reset tab state.
	m.logs = newLogsModel()
	m.metrics = newMetricsModel()
	m.profile = newProfileModel()
}

// disconnect cleans up the current connection.
func (m *detailModel) disconnect() {
	if m.cleanup != nil {
		m.cleanup()
		m.cleanup = nil
	}
	m.client = nil
	m.connected = false
	m.serverID = ""
	m.hostname = ""
}

// setSize propagates size to subtabs.
func (m *detailModel) setSize(w, h int) {
	m.width = w
	m.height = h
	contentHeight := h - 4 // tab bar + border
	contentWidth := w - 4  // border + padding
	m.logs.setSize(contentWidth, contentHeight)
	m.metrics.setSize(contentWidth, contentHeight)
	m.profile.setSize(contentWidth, contentHeight)
}

// View renders the detail pane.
func (m *detailModel) View(focused bool) string {
	style := inactiveBorderStyle
	if focused {
		style = activeBorderStyle
	}

	contentHeight := m.height - 2 // border
	contentWidth := m.width - 2   // border

	var b strings.Builder

	// Tab bar
	tabBar := m.tabBar()
	b.WriteString(tabBar)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", contentWidth))
	b.WriteByte('\n')
	tabBarHeight := 2

	remainingHeight := contentHeight - tabBarHeight

	if !m.connected {
		msg := " Select an instance from the left pane"
		b.WriteString(statusBarStyle.Render(msg))
		b.WriteByte('\n')
		for i := 1; i < remainingHeight; i++ {
			b.WriteByte('\n')
		}
	} else if m.err != nil {
		errMsg := fmt.Sprintf(" Error: %v", m.err)
		b.WriteString(logErrorStyle.Render(errMsg))
		b.WriteByte('\n')
		for i := 1; i < remainingHeight; i++ {
			b.WriteByte('\n')
		}
	} else {
		var content string
		switch m.activeTab {
		case tabLogs:
			content = m.logs.View(contentWidth, remainingHeight)
		case tabMetrics:
			content = m.metrics.View(contentWidth, remainingHeight)
		case tabProfile:
			content = m.profile.View(contentWidth, remainingHeight)
		}
		b.WriteString(content)
	}

	return style.
		Width(contentWidth).
		Height(contentHeight).
		Render(b.String())
}

// tabBar renders the tab selector.
func (m *detailModel) tabBar() string {
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if i == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	return " " + strings.Join(tabs, tabSeparator)
}

// initCmds returns the commands to initialize tabs when connecting.
// Note: log streaming is started separately by the root model via startLogStream.
func (m *detailModel) initCmds() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return tea.Batch(
		m.logs.fetchStatus(m.client),
		m.metrics.fetch(m.client),
	)
}
