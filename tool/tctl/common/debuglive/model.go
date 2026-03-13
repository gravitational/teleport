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
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

const (
	pickerWidthFraction = 30 // percentage of width for left pane
	statusRefreshRate   = 5 * time.Second
)

// instancesMsg carries a refreshed list of cluster instances.
type instancesMsg []instance

// instancesErrMsg carries an instance list error.
type instancesErrMsg struct{ err error }

// connectedMsg signals a successful tunnel connection.
type connectedMsg struct {
	client  *debugclient.Client
	cleanup func()
}

// connectErrMsg signals a tunnel connection failure.
type connectErrMsg struct{ err error }

// tickMsg fires periodically for refreshes.
type tickMsg time.Time

// programReadyMsg stores the program reference for goroutine sends.
type programReadyMsg struct{ p *tea.Program }

// model is the root bubbletea model for the debug live TUI.
type model struct {
	width, height int
	focusLeft     bool
	picker        pickerModel
	detail        detailModel
	keys          *keyMap
	authClient    *authclient.Client
	program       *tea.Program
	logCtxCancel  context.CancelFunc
	connectGen    uint64 // incremented on each connection to detect stale messages
	err           error
}

func newModel(authClient *authclient.Client) *model {
	return &model{
		focusLeft:  true,
		picker:     newPickerModel(),
		detail:     newDetailModel(),
		keys:       newKeyMap(),
		authClient: authClient,
	}
}

// Init starts the initial fetch and tick.
func (m *model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchInstances(),
		m.tick(),
	)
}

// Update processes all messages.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case programReadyMsg:
		m.program = msg.p

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tickMsg:
		cmds = append(cmds, m.tick())
		cmds = append(cmds, m.fetchInstances())
		if m.detail.connected {
			cmds = append(cmds, m.detail.logs.fetchStatus(m.detail.client))
			cmds = append(cmds, m.detail.metrics.fetch(m.detail.client))
		}

	case instancesMsg:
		m.picker.updateInstances(msg)
		m.err = nil

	case instancesErrMsg:
		m.err = msg.err

	case connectedMsg:
		m.connectGen++
		m.detail.setClient(msg.client, msg.cleanup, m.detail.serverID, m.detail.hostname)
		m.updateLayout()
		cmds = append(cmds, m.detail.initCmds())
		m.startLogStream()

	case connectErrMsg:
		m.detail.err = msg.err

	case logEntriesMsg, logStreamErrMsg, readinessMsg, readinessErrMsg, logLevelSetMsg:
		cmd := m.detail.logs.Update(msg, m.connectGen)
		cmds = append(cmds, cmd)

	case metricsResultMsg, metricsErrMsg:
		cmd := m.detail.metrics.Update(msg)
		cmds = append(cmds, cmd)

	case profileDoneMsg, profileErrMsg:
		cmd := m.detail.profile.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey processes keyboard input.
func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// If the metrics list is filtering, forward all keys to it.
	if !m.focusLeft && m.detail.activeTab == tabMetrics && m.detail.metrics.isFiltering() {
		cmd := m.detail.metrics.Update(msg)
		return cmd
	}

	// If picker is in filter mode, handle filter input.
	if m.focusLeft && m.picker.filtering {
		return m.handleFilterKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cleanupAll()
		return tea.Quit

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.ShiftTab):
		m.focusLeft = !m.focusLeft
		return nil

	case key.Matches(msg, m.keys.Tab1):
		if !m.focusLeft {
			m.detail.activeTab = tabLogs
		}
		return nil
	case key.Matches(msg, m.keys.Tab2):
		if !m.focusLeft {
			m.detail.activeTab = tabMetrics
		}
		return nil
	case key.Matches(msg, m.keys.Tab3):
		if !m.focusLeft {
			m.detail.activeTab = tabProfile
		}
		return nil
	}

	if m.focusLeft {
		return m.handlePickerKey(msg)
	}
	return m.handleDetailKey(msg)
}

// handlePickerKey processes keys when left pane is focused.
func (m *model) handlePickerKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.picker.moveUp()
	case key.Matches(msg, m.keys.Down):
		m.picker.moveDown()
	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.Right):
		m.picker.toggleCollapse()
	case key.Matches(msg, m.keys.Enter):
		return m.connectToSelected()
	case key.Matches(msg, m.keys.Filter):
		m.picker.filtering = true
		m.picker.filter = ""
	}
	return nil
}

// handleFilterKey processes keys during filter input.
func (m *model) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.picker.filtering = false
		m.picker.filter = ""
	case tea.KeyEnter:
		m.picker.filtering = false
	case tea.KeyBackspace:
		if len(m.picker.filter) > 0 {
			m.picker.filter = m.picker.filter[:len(m.picker.filter)-1]
		}
	default:
		if len(msg.Runes) > 0 {
			m.picker.filter += string(msg.Runes)
		}
	}
	return nil
}

// handleDetailKey processes keys when right pane is focused.
func (m *model) handleDetailKey(msg tea.KeyMsg) tea.Cmd {
	switch m.detail.activeTab {
	case tabLogs:
		return m.handleLogsKey(msg)
	case tabMetrics:
		return m.handleMetricsKey(msg)
	case tabProfile:
		return m.handleProfileKey(msg)
	}
	return nil
}

func (m *model) handleLogsKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.LogLevel):
		m.detail.logs.cycleLevelFilter()
		m.restartLogStream()
	case key.Matches(msg, m.keys.Pause):
		m.detail.logs.togglePause()
	case key.Matches(msg, m.keys.Up):
		m.detail.logs.scrollUp()
	case key.Matches(msg, m.keys.Down):
		m.detail.logs.scrollDown()
	case key.Matches(msg, m.keys.Left):
		m.detail.logs.cycleLogLevel(-1)
	case key.Matches(msg, m.keys.Right):
		m.detail.logs.cycleLogLevel(1)
	case key.Matches(msg, m.keys.Enter):
		if m.detail.client != nil {
			return m.detail.logs.setLogLevel(m.detail.client, m.detail.logs.selectedLogLevel())
		}
	}
	return nil
}

func (m *model) handleMetricsKey(msg tea.KeyMsg) tea.Cmd {
	cmd := m.detail.metrics.Update(msg)
	return cmd
}

func (m *model) handleProfileKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Space):
		m.detail.profile.toggleCurrent()
	case key.Matches(msg, m.keys.Up):
		m.detail.profile.moveUp()
	case key.Matches(msg, m.keys.Down):
		m.detail.profile.moveDown()
	case key.Matches(msg, m.keys.Left):
		m.detail.profile.adjustSeconds(-5)
	case key.Matches(msg, m.keys.Right):
		m.detail.profile.adjustSeconds(5)
	case key.Matches(msg, m.keys.Enter):
		if m.detail.client != nil {
			return m.detail.profile.collect(m.detail.client, m.detail.serverID)
		}
	}
	return nil
}

// connectToSelected initiates a tunnel connection to the selected instance.
func (m *model) connectToSelected() tea.Cmd {
	inst := m.picker.selectedInstance()
	if inst == nil {
		return nil
	}

	gIdx := m.picker.selectedGlobalIndex()
	if gIdx == m.picker.connected {
		return nil // already connected
	}

	// Stop existing log stream and disconnect.
	m.stopLogStream()
	m.detail.disconnect()

	// Store target info before async connect.
	m.detail.serverID = inst.serverID
	m.detail.hostname = inst.hostname
	m.picker.connected = gIdx

	return m.connectCmd(inst.serverID)
}

// connectCmd opens a Debug.Connect tunnel to the target.
// Each HTTP request gets its own gRPC tunnel via DialContext, so the
// long-lived log stream doesn't block short-lived status/metrics requests.
func (m *model) connectCmd(serverID string) tea.Cmd {
	authClient := m.authClient
	return func() tea.Msg {
		grpcClt := debugpb.NewDebugServiceClient(authClient.GetConnection())

		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				stream, err := grpcClt.Connect(ctx)
				if err != nil {
					return nil, err
				}
				if err := stream.Send(&debugpb.Frame{
					Payload: &debugpb.Frame_ServerId{ServerId: serverID},
				}); err != nil {
					return nil, err
				}
				return newStreamConn(stream), nil
			},
			DisableKeepAlives: true,
		}

		httpClient := &http.Client{Transport: transport}
		debugClt := debugclient.NewClientWithHTTPClient(httpClient)
		cleanup := func() {
			transport.CloseIdleConnections()
		}
		return connectedMsg{client: debugClt, cleanup: cleanup}
	}
}

// fetchInstances fetches the instance list from the cluster.
func (m *model) fetchInstances() tea.Cmd {
	authClient := m.authClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		instances := authClient.GetInstances(ctx, types.InstanceFilter{})
		var result []instance
		for instances.Next() {
			inst := instances.Item()
			services := make([]string, 0, len(inst.GetServices()))
			for _, s := range inst.GetServices() {
				services = append(services, string(s))
			}
			result = append(result, instance{
				serverID: inst.GetName(),
				hostname: inst.GetHostname(),
				services: services,
			})
		}
		if err := instances.Done(); err != nil {
			return instancesErrMsg{err: err}
		}
		return instancesMsg(result)
	}
}

// tick returns a periodic tick command.
func (m *model) tick() tea.Cmd {
	return tea.Tick(statusRefreshRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// startLogStream launches the log streaming goroutine.
// It uses m.program.Send to push log entries back to the model.
func (m *model) startLogStream() {
	if m.detail.client == nil || m.program == nil {
		return
	}
	m.stopLogStream()
	ctx, cancel := context.WithCancel(context.Background())
	m.logCtxCancel = cancel
	client := m.detail.client
	level := logLevels[m.detail.logs.levelIdx]
	p := m.program
	gen := m.connectGen

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			body, err := client.GetLogStream(ctx, level)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				p.Send(logStreamErrMsg{err: err, gen: gen})
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}

			scanner := bufio.NewScanner(body)
			var batch []logEntry
			for scanner.Scan() {
				if ctx.Err() != nil {
					body.Close()
					return
				}
				line := scanner.Text()
				if line == "" {
					continue
				}
				batch = append(batch, parseLogLine(line))
				if len(batch) >= 50 {
					p.Send(logEntriesMsg{entries: batch, gen: gen})
					batch = nil
				}
			}
			body.Close()
			if len(batch) > 0 {
				p.Send(logEntriesMsg{entries: batch, gen: gen})
			}

			if ctx.Err() != nil {
				return
			}
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// restartLogStream stops and restarts the log stream with new filters.
func (m *model) restartLogStream() {
	m.startLogStream()
}

// stopLogStream cancels the log streaming goroutine.
func (m *model) stopLogStream() {
	if m.logCtxCancel != nil {
		m.logCtxCancel()
		m.logCtxCancel = nil
	}
}

// cleanupAll cleans up all resources.
func (m *model) cleanupAll() {
	m.stopLogStream()
	m.detail.disconnect()
}

// updateLayout recalculates pane sizes.
func (m *model) updateLayout() {
	pickerWidth := m.width * pickerWidthFraction / 100
	detailWidth := m.width - pickerWidth
	paneHeight := m.height - 2 // help bar

	m.picker.width = pickerWidth
	m.picker.height = paneHeight
	m.detail.setSize(detailWidth, paneHeight)
}

// View renders the full TUI.
func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	m.updateLayout()

	leftPane := m.picker.View(m.focusLeft)
	rightPane := m.detail.View(!m.focusLeft)
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	helpBar := m.helpBar()

	return lipgloss.JoinVertical(lipgloss.Left, mainContent, helpBar)
}

// helpBar renders the bottom help bar.
func (m *model) helpBar() string {
	var parts []string
	parts = append(parts, "tab: switch pane")
	parts = append(parts, "↑↓: navigate")
	if m.focusLeft {
		parts = append(parts, "/: filter")
		parts = append(parts, "enter: connect")
		parts = append(parts, "←→: collapse")
	} else {
		parts = append(parts, "1-3: tabs")
		switch m.detail.activeTab {
		case tabLogs:
			parts = append(parts, "l: level filter")
			parts = append(parts, "p: pause")
			parts = append(parts, "←→: set level")
			parts = append(parts, "enter: apply")
		case tabMetrics:
			parts = append(parts, "/: filter")
		case tabProfile:
			parts = append(parts, "space: toggle")
			parts = append(parts, "enter: collect")
			parts = append(parts, "←→: duration")
		}
	}
	parts = append(parts, "q: quit")

	if m.err != nil {
		parts = append(parts, logErrorStyle.Render("Error: "+m.err.Error()))
	}

	return helpBarStyle.Render(" " + strings.Join(parts, " | "))
}

// streamConn wraps a gRPC bidirectional stream as a net.Conn.
type streamConn struct {
	stream debugpb.DebugService_ConnectClient
	buf    bytes.Buffer
}

func newStreamConn(stream debugpb.DebugService_ConnectClient) *streamConn {
	return &streamConn{stream: stream}
}

func (c *streamConn) Read(p []byte) (int, error) {
	if c.buf.Len() > 0 {
		return c.buf.Read(p)
	}
	frame, err := c.stream.Recv()
	if err != nil {
		return 0, err
	}
	data := frame.GetData()
	n := copy(p, data)
	if n < len(data) {
		c.buf.Write(data[n:])
	}
	return n, nil
}

func (c *streamConn) Write(p []byte) (int, error) {
	cp := make([]byte, len(p))
	copy(cp, p)
	if err := c.stream.Send(&debugpb.Frame{
		Payload: &debugpb.Frame_Data{Data: cp},
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *streamConn) Close() error                     { return c.stream.CloseSend() }
func (c *streamConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *streamConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *streamConn) SetDeadline(time.Time) error      { return nil }
func (c *streamConn) SetReadDeadline(time.Time) error  { return nil }
func (c *streamConn) SetWriteDeadline(time.Time) error { return nil }
