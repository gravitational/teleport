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
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
	"github.com/guptarohit/asciigraph"
	dto "github.com/prometheus/client_model/go"

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
	keys            *keyMap
	refreshInterval time.Duration
	clt             MetricsClient
	metricsList     list.Model
	report          *Report
	reportError     error
	addr            string
}

func newTopModel(refreshInterval time.Duration, clt MetricsClient, addr string) *topModel {
	// A delegate is used to implement custom styling for list items.
	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Faint(true)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Faint(true)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Faint(false).Foreground(selectedColor)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Faint(false)

	metricsList := list.New(nil, delegate, 0, 0)
	metricsList.SetShowTitle(false)
	metricsList.SetShowFilter(true)
	metricsList.SetShowStatusBar(true)
	metricsList.SetShowHelp(false)

	return &topModel{
		help:            help.New(),
		clt:             clt,
		refreshInterval: refreshInterval,
		addr:            addr,
		keys:            newDefaultKeymap(),
		metricsList:     metricsList,
	}
}

// tickMsg is dispached when the refresh period expires.
type tickMsg time.Time

// metricsMsg contains new prometheus metrics.
type metricsMsg map[string]*dto.MetricFamily

// tick provides a ticker at a specified interval.
func (m *topModel) tick() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// fetchMetricsCmd fetches metrics from target and returns [metricsMsg].
func (m *topModel) fetchMetricsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		metrics, err := m.clt.GetMetrics(ctx)
		if err != nil {
			return err
		}

		return metricsMsg(metrics)
	}
}

// generateReportCmd returns a command to generate a [Report] from given [metricsMsg]
func (m *topModel) generateReportCmd(metrics metricsMsg) tea.Cmd {
	return func() tea.Msg {
		report, err := generateReport(metrics, m.report, m.refreshInterval)
		if err != nil {
			return err
		}
		return report
	}
}

// Init kickstarts the ticker to begin polling.
func (m *topModel) Init() tea.Cmd {
	return func() tea.Msg {
		return tickMsg(time.Now())
	}
}

// isMetricFilterFocused checks if the user is currently on the metric pane and filtering is in progress.
func (m *topModel) isMetricFilterFocused() bool {
	return m.selected == 5 && m.metricsList.FilterState() == list.Filtering
}

// Update processes messages in order to updated the
// view based on user input and new metrics data.
func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.height = msg.Height - v
		m.width = msg.Width - h
		m.metricsList.SetSize(m.width, m.height-6 /* account for UI height */)
	case tea.KeyMsg:
		if m.isMetricFilterFocused() {
			// Redirect all keybinds to the list until the user is done.
			var cmd tea.Cmd
			m.metricsList, cmd = m.metricsList.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Common):
				m.selected = 0
			case key.Matches(msg, m.keys.Backend):
				m.selected = 1
			case key.Matches(msg, m.keys.Cache):
				m.selected = 2
			case key.Matches(msg, m.keys.Watcher):
				m.selected = 3
			case key.Matches(msg, m.keys.Audit):
				m.selected = 4
			case key.Matches(msg, m.keys.Raw):
				m.selected = 5
			case key.Matches(msg, m.keys.Right):
				m.selected = (m.selected + 1) % len(tabs)
			case key.Matches(msg, m.keys.Left):
				m.selected = (m.selected - 1 + len(tabs)) % len(tabs)
			case key.Matches(msg, m.keys.Filter),
				key.Matches(msg, m.keys.Up),
				key.Matches(msg, m.keys.Down):
				// Only a subset of keybinds are forwarded to the list view.
				var cmd tea.Cmd
				m.metricsList, cmd = m.metricsList.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	case tickMsg:
		cmds = append(cmds, m.tick(), m.fetchMetricsCmd())
	case metricsMsg:
		cmds = append(cmds, m.generateReportCmd(msg))

		filterValue := m.metricsList.FilterInput.Value()
		selected := m.metricsList.Index()

		var cmd tea.Cmd
		cmd = m.metricsList.SetItems(convertMetricsToItems(msg))
		cmds = append(cmds, cmd)

		// There is a glitch in the list.Model view when a filter has been applied
		// the pagination status is broken after replacing all items. Workaround this by
		// manually resetting the filter and updating the selection.
		if m.metricsList.FilterState() == list.FilterApplied {
			m.metricsList.SetFilterText(filterValue)
			m.metricsList.Select(selected)
		}

	case *Report:
		m.report = msg
		m.reportError = nil
	case error:
		m.reportError = msg
	default:
		// Forward internal messages to the metrics list.
		var cmd tea.Cmd
		m.metricsList, cmd = m.metricsList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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
			leftContent = fmt.Sprintf("Could not connect to metrics service: %v", m.addr)
		} else {
			leftContent = fmt.Sprintf("Failed to generate report: %v", m.reportError)
		}
	}
	if leftContent == "" && m.report != nil {
		leftContent = fmt.Sprintf("Report generated at %s for host %s (%s)",
			m.report.Timestamp.Format(constants.HumanDateFormatSeconds),
			m.report.Hostname,
			m.addr,
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
		Render(m.help.View(m.keys))

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
	case 4:
		return renderAudit(m.report, m.height, m.width)
	case 5:
		return boxedViewWithStyle("Prometheus Metrics", m.metricsList.View(), m.width, lipgloss.NewStyle())
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

	serviceKeys := slices.Sorted(maps.Keys(report.Service))
	serviceCounts := make([]string, 0, len(serviceKeys))
	for _, k := range serviceKeys {
		serviceCounts = append(serviceCounts, humanize.FormatFloat("#.", report.Service[k]))
	}
	servicesTable := tableView(
		columnWidth,
		column{
			width:   columnWidth * 8 / 10,
			content: serviceKeys,
		},
		column{
			width:   columnWidth * 2 / 10,
			content: serviceCounts,
		},
	)
	servicesContent := boxedView("Services", servicesTable, columnWidth)

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
		style.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				servicesContent,
				certLatencyContent,
			),
		),
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
		asciigraph.UpperBound(1),
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
		asciigraph.UpperBound(1),
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

// renderAudit generates the view for the audit stats tab.
func renderAudit(report *Report, height, width int) string {
	graphHeight := height / 3
	graphWidth := width

	eventsLegend := lipgloss.JoinHorizontal(
		lipgloss.Left,
		"- Emitted",
		failedEventStyle.Render(" - Failed"),
		trimmedEventStyle.Render(" - Trimmed"),
	)

	eventsPlot := asciigraph.PlotMany(
		[][]float64{
			report.Audit.EmittedEventsBuffer.Data(graphWidth - 15),
			report.Audit.FailedEventsBuffer.Data(graphWidth - 15),
			report.Audit.TrimmedEventsBuffer.Data(graphWidth - 15),
		},
		asciigraph.Height(graphHeight),
		asciigraph.Width(graphWidth-15),
		asciigraph.UpperBound(1),
		asciigraph.SeriesColors(asciigraph.Default, asciigraph.Red, asciigraph.Goldenrod),
		asciigraph.Caption(eventsLegend),
	)
	eventGraph := boxedView("Events Emitted", eventsPlot, graphWidth)

	eventSizePlot := asciigraph.Plot(
		report.Audit.EventSizeBuffer.Data(graphWidth-15),
		asciigraph.Height(graphHeight),
		asciigraph.Width(graphWidth-15),
		asciigraph.UpperBound(1),
	)
	sizeGraph := boxedView("Event Sizes", eventSizePlot, graphWidth)

	graphStyle := lipgloss.NewStyle().
		Width(graphWidth).
		Padding(0).
		Margin(0).
		Align(lipgloss.Left)

	return lipgloss.JoinVertical(lipgloss.Left,
		graphStyle.Render(
			eventGraph,
			sizeGraph,
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

var (
	statusBarStyle = lipgloss.NewStyle()

	separator = lipgloss.NewStyle().
			Faint(true).
			Render(" • ")

	selectedColor = lipgloss.Color("4")

	failedEventStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", asciigraph.Red)))
	trimmedEventStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", asciigraph.Goldenrod)))

	tabs = []string{"Common", "Backend", "Cache", "Watcher", "Audit", "Raw Metrics"}
)
