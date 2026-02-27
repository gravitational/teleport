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
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	dto "github.com/prometheus/client_model/go"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

// metricsResultMsg carries metrics from a fetch.
type metricsResultMsg map[string]*dto.MetricFamily

// metricsErrMsg carries a metrics fetch error.
type metricsErrMsg struct{ err error }

// metricsModel implements Tab 3: Prometheus metrics browser.
type metricsModel struct {
	metricsList list.Model
	lastErr     error
	lastRefresh time.Time
	width       int
	height      int
}

// metricsItem implements the list.Item interface for a metric.
type metricsItem struct {
	name string
	help string
}

func (i metricsItem) Title() string       { return i.name }
func (i metricsItem) Description() string { return i.help }
func (i metricsItem) FilterValue() string { return i.name }

func newMetricsModel() metricsModel {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Faint(true)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Faint(true)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(accentColor)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle()

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)

	return metricsModel{
		metricsList: l,
	}
}

func (m *metricsModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.metricsList.SetSize(w, h-2)
}

// fetch creates a command to fetch metrics.
func (m *metricsModel) fetch(client *debugclient.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		metrics, err := client.GetMetrics(ctx)
		if err != nil {
			return metricsErrMsg{err: err}
		}
		return metricsResultMsg(metrics)
	}
}

// Update handles messages for the metrics tab.
func (m *metricsModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case metricsResultMsg:
		m.lastErr = nil
		m.lastRefresh = time.Now()

		filterValue := m.metricsList.FilterInput.Value()
		selected := m.metricsList.Index()

		items := convertMetrics(msg)
		cmd := m.metricsList.SetItems(items)

		// Restore filter state.
		if m.metricsList.FilterState() == list.FilterApplied {
			m.metricsList.SetFilterText(filterValue)
			m.metricsList.Select(selected)
		}
		return cmd
	case metricsErrMsg:
		m.lastErr = msg.err
		return nil
	default:
		var cmd tea.Cmd
		m.metricsList, cmd = m.metricsList.Update(msg)
		return cmd
	}
}

func convertMetrics(metrics map[string]*dto.MetricFamily) []list.Item {
	names := slices.Sorted(maps.Keys(metrics))
	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		family := metrics[name]
		help := ""
		if family.Help != nil {
			help = family.GetHelp()
		}
		items = append(items, metricsItem{
			name: name,
			help: help,
		})
	}
	return items
}

// View renders the metrics tab.
func (m *metricsModel) View(width, height int) string {
	if m.lastErr != nil {
		return logErrorStyle.Render(fmt.Sprintf(" Error: %v", m.lastErr))
	}

	var b strings.Builder
	b.WriteString(m.metricsList.View())

	if !m.lastRefresh.IsZero() {
		b.WriteByte('\n')
		b.WriteString(statusBarStyle.Render(
			fmt.Sprintf(" Last refresh: %s", m.lastRefresh.Format("15:04:05")),
		))
	}

	return b.String()
}

// isFiltering returns true if the user is actively filtering.
func (m *metricsModel) isFiltering() bool {
	return m.metricsList.FilterState() == list.Filtering
}
