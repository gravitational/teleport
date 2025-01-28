// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package top

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/guptarohit/asciigraph"

	"github.com/gravitational/teleport/api/constants"
)

// topModel is a [tea.Model] implementation which
// displays various tabs and content displayed by
// the tctl top command.
type topModel struct {
	width           int
	height          int
	selected        int
	help            help.Model
	refreshInterval time.Duration
	clt             *roundtrip.Client
	report          *Report
	reportError     error
}

func newTopModel(refreshInterval time.Duration, clt *roundtrip.Client) *topModel {
	return &topModel{
		help:            help.New(),
		clt:             clt,
		refreshInterval: refreshInterval,
	}
}

// refresh pulls metrics from Teleport and builds
// a [Report] according to the configured refresh
// interval.
func (m *topModel) refresh() tea.Cmd {
	return func() tea.Msg {
		if m.report != nil {
			<-time.After(m.refreshInterval)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		report, err := fetchAndGenerateReport(ctx, m.clt, m.report, m.refreshInterval)
		if err != nil {
			return err
		}

		return report
	}
}

// Init is a noop but required to implement [tea.Model].
func (m *topModel) Init() tea.Cmd {
	return m.refresh()
}

// Update processes messages in order to updated the
// view based on user input and new metrics data.
func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.height = msg.Height - v
		m.width = msg.Width - h
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.selected = 0
		case "2":
			m.selected = 1
		case "3":
			m.selected = 2
		case "4":
			m.selected = 3
		case "right":
			m.selected = min(m.selected+1, len(tabs)-1)
		case "left":
			m.selected = max(m.selected-1, 0)
		}
	case *Report:
		m.report = msg
		m.reportError = nil
		return m, m.refresh()
	case error:
		m.reportError = msg
		return m, m.refresh()
	}
	return m, nil
}

// View formats the metrics and draws them to
// the screen.
func (m *topModel) View() string {
	availableHeight := m.height
	header := headerView(m.selected, m.width)
	availableHeight -= lipgloss.Height(header)

	footer := m.footerView()
	availableHeight -= lipgloss.Height(footer)

	content := lipgloss.NewStyle().
		Height(availableHeight).
		Width(m.width).
		Render(m.contentView())

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		footer,
	)
}

// headerView generates the tab bar displayed at
// the top of the screen. The selectedTab will be
// rendered a different color to indicate as such.
func headerView(selectedTab int, width int) string {
	tabs := tabView(selectedTab)

	availableSpace := width - lipgloss.Width(tabs)

	var filler string
	if availableSpace > 0 {
		filler = strings.Repeat(" ", availableSpace)
	}

	return tabs + lipgloss.NewStyle().Render(filler) + "\n" + strings.Repeat("‾", width)
}

// footerView generates the help text displayed at the
// bottom of the screen.
func (m *topModel) footerView() string {
	underscore := lipgloss.NewStyle().Underline(true).Render(" ")
	underline := strings.Repeat(underscore, m.width)

	var leftContent string

	if m.reportError != nil {
		if trace.IsConnectionProblem(m.reportError) {
			leftContent = fmt.Sprintf("Could not connect to metrics service: %v", m.clt.Endpoint())
		} else {
			leftContent = fmt.Sprintf("Failed to generate report: %v", m.reportError)
		}
	}
	if leftContent == "" && m.report != nil {
		leftContent = fmt.Sprintf("Report generated at %s for host %s",
			m.report.Timestamp.Format(constants.HumanDateFormatSeconds),
			m.report.Hostname,
		)
	}
	left := lipgloss.NewStyle().
		Inline(true).
		Width(len(leftContent)).
		MaxWidth(100).
		Render(leftContent)

	right := lipgloss.NewStyle().
		Inline(true).
		Width(35).
		Align(lipgloss.Center).
		Render(m.help.View(helpKeys))

	center := lipgloss.NewStyle().
		Inline(true).
		Width(m.width - len(leftContent) - 35).
		Align(lipgloss.Center).
		Render("")

	return underline + "\n" +
		statusBarStyle.Render(left) +
		statusBarStyle.Render(center) +
		statusBarStyle.Render(right)
}

// contentView generates the appropriate content
// based on which tab is selected.
func (m *topModel) contentView() string {
	if m.report == nil {
		return ""
	}

	switch m.selected {
	case 0:
		return renderCommon(m.report, m.width)
	case 1:
		return renderBackend(m.report, m.height, m.width)
	case 2:
		return renderCache(m.report, m.height, m.width)
	case 3:
		return renderWatcher(m.report, m.height, m.width)
	default:
		return ""
	}
}

// renderCommon generates the view for the cluster stats tab.
func renderCommon(report *Report, width int) string {
	columnWidth := width / 2

	clusterTable := tableView(
		columnWidth,
		column{
			width: width / 3,
			content: []string{
				"Interactive Sessions",
				"Cert Gen Active Requests",
				"Cert Gen Requests/sec",
				"Cert Gen Throttled Requests/sec",
				"Auth Watcher Queue Size",
				"Roles",
				"Active Migrations",
			},
		},
		column{
			width: columnWidth / 3,
			content: []string{
				humanize.FormatFloat("", report.Cluster.InteractiveSessions),
				humanize.FormatFloat("", report.Cluster.GenerateRequests),
				humanize.FormatFloat("", report.Cluster.GenerateRequestsCount.GetFreq()),
				humanize.FormatFloat("", report.Cluster.GenerateRequestsThrottledCount.GetFreq()),
				humanize.FormatFloat("", report.Cache.QueueSize),
				humanize.FormatFloat("", report.Cluster.Roles),
				cmp.Or(strings.Join(report.Cluster.ActiveMigrations, ", "), "None"),
			},
		},
	)
	clusterContent := boxedView("Cluster Stats", clusterTable, columnWidth)

	processTable := tableView(
		columnWidth,
		column{
			width: columnWidth * 4 / 10,
			content: []string{
				"Start Time",
				"Resident Memory",
				"CPU Seconds Total",
				"Open FDs",
				"Max FDs",
			},
		},
		column{
			width: columnWidth * 6 / 10,
			content: []string{
				report.Process.StartTime.Format(constants.HumanDateFormatSeconds),
				humanize.Bytes(uint64(report.Process.ResidentMemoryBytes)),
				humanize.FormatFloat("", report.Process.CPUSecondsTotal),
				humanize.FormatFloat("", report.Process.OpenFDs),
				humanize.FormatFloat("", report.Process.MaxFDs),
			},
		},
	)
	processContent := boxedView("Process Stats", processTable, columnWidth)

	runtimeTable := tableView(
		columnWidth,
		column{
			width: columnWidth / 2,
			content: []string{
				"Allocated Memory",
				"Goroutines",
				"Threads",
				"Heap Objects",
				"Heap Allocated Memory",
				"Info",
			},
		},
		column{
			width: columnWidth / 2,
			content: []string{
				humanize.Bytes(uint64(report.Go.AllocBytes)),
				humanize.FormatFloat("", report.Go.Goroutines),
				humanize.FormatFloat("", report.Go.Threads),
				humanize.FormatFloat("", report.Go.HeapObjects),
				humanize.Bytes(uint64(report.Go.HeapAllocBytes)),
				report.Go.Info,
			},
		},
	)
	runtimeContent := boxedView("Go Runtime Stats", runtimeTable, columnWidth)

	certLatencyContent := boxedView("Generate Server Certificates Percentiles", "No data", columnWidth)

	style := lipgloss.NewStyle().
		Width(columnWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		style.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				clusterContent,
				processContent,
				runtimeContent,
			),
		),
		style.Render(certLatencyContent),
	)
}

// renderBackend generates the view for the backend stats tab.
func renderBackend(report *Report, height, width int) string {
	latencyWidth := width / 3
	requestsWidth := width * 2 / 3
	topRequestsContent := boxedView("Top Backend Requests", requestsTableView(height, requestsWidth, report.Backend), requestsWidth)
	readLatentcyContent := boxedView("Backend Read Percentiles", percentileTableView(latencyWidth, report.Backend.Read), latencyWidth)
	batchReadLatentyContent := boxedView("Backend Batch Read Percentiles", percentileTableView(latencyWidth, report.Backend.BatchRead), latencyWidth)
	writeLatencyContent := boxedView("Backend Write Percentiles", percentileTableView(latencyWidth, report.Backend.Write), latencyWidth)

	latencyStyle := lipgloss.NewStyle().
		Width(latencyWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	requestsStyle := lipgloss.NewStyle().
		Width(requestsWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		requestsStyle.Render(topRequestsContent),
		latencyStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				readLatentcyContent,
				batchReadLatentyContent,
				writeLatencyContent,
			),
		),
	)
}

// renderCache generates the view for the cache stats tab.
func renderCache(report *Report, height, width int) string {
	latencyWidth := width / 3
	requestsWidth := width * 2 / 3

	topRequestsContent := boxedView("Top Cache Requests", requestsTableView(height, requestsWidth, report.Cache), requestsWidth)
	readLatentcyContent := boxedView("Cache Read Percentiles", percentileTableView(latencyWidth, report.Cache.Read), latencyWidth)
	batchReadLatentyContent := boxedView("Cache Batch Read Percentiles", percentileTableView(latencyWidth, report.Cache.BatchRead), latencyWidth)
	writeLatencyContent := boxedView("Cache Write Percentiles", percentileTableView(latencyWidth, report.Cache.Write), latencyWidth)

	latencyStyle := lipgloss.NewStyle().
		Width(latencyWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	requestsStyle := lipgloss.NewStyle().
		Width(requestsWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		requestsStyle.Render(topRequestsContent),
		latencyStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				readLatentcyContent,
				batchReadLatentyContent,
				writeLatencyContent,
			),
		),
	)
}

// renderWatcher generates the view for the watcher stats tab.
func renderWatcher(report *Report, height, width int) string {
	graphWidth := width * 40 / 100
	graphHeight := height / 3
	eventsWidth := width * 60 / 100

	topEventsContent := boxedView("Top Events Emitted", eventsTableView(height, eventsWidth, report.Watcher), eventsWidth)

	dataCount := (graphWidth / 2)
	eventData := report.Watcher.EventsPerSecond.Data(dataCount)
	if len(eventData) < 1 {
		eventData = []float64{0, 0}
	}
	countPlot := asciigraph.Plot(
		eventData,
		asciigraph.Height(graphHeight),
		asciigraph.Width(graphWidth-15),
	)
	eventCountContent := boxedView("Events/Sec", countPlot, graphWidth)

	sizeData := report.Watcher.BytesPerSecond.Data(dataCount)
	if len(sizeData) < 1 {
		sizeData = []float64{0, 0}
	}
	sizePlot := asciigraph.Plot(
		sizeData,
		asciigraph.Height(graphHeight),
		asciigraph.Width(graphWidth-15),
	)
	eventSizeContent := boxedView("Bytes/Sec", sizePlot, graphWidth)

	graphStyle := lipgloss.NewStyle().
		Width(graphWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	eventsStyle := lipgloss.NewStyle().
		Width(eventsWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		eventsStyle.Render(topEventsContent),
		graphStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				eventCountContent,
				eventSizeContent,
			),
		),
	)
}

// tabView renders the tabbed content in the header.
func tabView(selectedTab int) string {
	output := lipgloss.NewStyle().
		Underline(true).
		Render("")

	for i, tab := range tabs {
		lastItem := i == len(tabs)-1
		selected := i == selectedTab

		var color lipgloss.Color
		if selected {
			color = selectedColor
		}

		output += lipgloss.NewStyle().
			Foreground(color).
			Faint(!selected).
			Render(tab)

		if !lastItem {
			output += separator
		}
	}

	return output
}

// keyMap is used to display the help text at
// the bottom of the screen.
type keyMap struct {
	quit  key.Binding
	right key.Binding
	left  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.left, k.right, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.left, k.right},
		{k.quit},
	}
}

var (
	helpKeys = keyMap{
		quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		left: key.NewBinding(
			key.WithKeys("left", "esc"),
			key.WithHelp("left", "previous"),
		),
		right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("right", "next"),
		),
	}

	statusBarStyle = lipgloss.NewStyle()

	separator = lipgloss.NewStyle().
			Faint(true).
			Render(" • ")

	selectedColor = lipgloss.Color("4")

	tabs = []string{"Common", "Backend", "Cache", "Watcher"}
)
