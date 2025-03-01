// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/trace"
	"github.com/guptarohit/asciigraph"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
)

// keyMap is used to display the help text at
// the bottom of the screen.
type keyMap struct {
	quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.quit}}
}

var helpKeys = keyMap{
	quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// latencyModel is a [tea.Model] that contains the state required
// to display the latency graph.
type latencyModel struct {
	ready                        bool
	h, w                         int
	help                         help.Model
	clientLabel, serverLabel     string
	clientLegend, serverLegend   viewport.Model
	clientMax, serverMax         int64
	clientLatency, serverLatency *utils.CircularBuffer
}

// newLatencyModel constructs a new latencyModel.
func newLatencyModel(clientLabel, serverLabel string) (*latencyModel, error) {
	clientStats, err := utils.NewCircularBuffer(50)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverStats, err := utils.NewCircularBuffer(50)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &latencyModel{
		clientLabel:   clientLabel,
		serverLabel:   serverLabel,
		help:          help.New(),
		clientLegend:  viewport.New(1, 2),
		serverLegend:  viewport.New(1, 2),
		clientLatency: clientStats,
		serverLatency: serverStats,
	}, nil
}

// Init is a noop but required to implement [tea.Model].
func (m *latencyModel) Init() tea.Cmd {
	return nil
}

// Update reacts to screen resizing and new latency statistics.
func (m *latencyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.ready = true
		m.h = msg.Height
		m.w = msg.Width
		m.clientLegend.Width = m.w / 2
		m.serverLegend.Width = m.clientLegend.Width
	case latency.Statistics:
		m.clientLatency.Add(float64(msg.Client))
		m.serverLatency.Add(float64(msg.Server))
		m.clientMax = max(m.clientMax, msg.Client)
		m.serverMax = max(m.serverMax, msg.Server)
		m.clientLegend.SetContent(fmt.Sprintf("%s: last recorded %dms (max %dms)\t", m.clientLabel, msg.Client, m.clientMax))
		m.serverLegend.SetContent(fmt.Sprintf("%s: last recorded %dms (max %dms)", m.serverLabel, msg.Server, m.serverMax))
	}
	return m, nil
}

var (
	// clientColor is the color of the client plot and the client information in the legend.
	clientColor = asciigraph.Blue
	// serverColor is the color of the server plot and the client information in the legend.
	serverColor = asciigraph.Goldenrod
	clientStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", clientColor)))
	serverStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", serverColor)))
)

// View renders the current latency graph to be displayed.
func (m *latencyModel) View() string {
	if !m.ready {
		return ""
	}

	clientData := m.clientLatency.Data(150)
	serverData := m.serverLatency.Data(150)

	if clientData == nil || serverData == nil {
		return ""
	}

	legend := lipgloss.JoinHorizontal(
		lipgloss.Top,
		clientStyle.Render(m.clientLegend.View()),
		serverStyle.Render(m.serverLegend.View()),
	)

	plot := asciigraph.PlotMany(
		[][]float64{clientData, serverData},
		asciigraph.Height(m.h-4),
		asciigraph.Width(m.w),
		asciigraph.UpperBound(1),
		asciigraph.SeriesColors(clientColor, serverColor),
		asciigraph.Caption(legend),
	)

	return lipgloss.JoinVertical(lipgloss.Top, plot, lipgloss.PlaceHorizontal(m.w, lipgloss.Bottom, m.help.View(helpKeys)))
}

// showLatency creates a [latency.Monitor] using the provided [latency.Pinger]s to capture latency and
// render it to the terminal via a plot graph.
func showLatency(ctx context.Context, clientPinger, serverPinger latency.Pinger, clientLabel, serverLabel string) error {
	m, err := newLatencyModel(clientLabel, serverLabel)
	if err != nil {
		return trace.Wrap(err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithoutSignalHandler())

	monitor, err := latency.NewMonitor(latency.MonitorConfig{
		InitialPingInterval:   time.Millisecond,
		InitialReportInterval: 500 * time.Millisecond,
		PingInterval:          time.Second,
		ReportInterval:        time.Second,
		ClientPinger:          clientPinger,
		ServerPinger:          serverPinger,
		Reporter: latency.ReporterFunc(func(ctx context.Context, stats latency.Statistics) error {
			p.Send(stats)
			return nil
		}),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go monitor.Run(ctx)

	if _, err := p.Run(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
