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

package raw

import (
	"fmt"
	"io"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitational/teleport/tool/tctl/common/top/tui/common"
)

type metricItem struct {
	name        string
	description string
	value       string
}

func (m metricItem) Title() string       { return m.name }
func (m metricItem) Description() string { return m.description + "\n" + m.value }
func (m metricItem) FilterValue() string { return m.name }

func convertMetricsToListItems(msg common.MetricsMsg) []list.Item {
	items := []list.Item{}

	for _, mf := range msg {
		for _, m := range mf.GetMetric() {
			nameWithLabels := mf.GetName()

			if len(m.Label) > 0 {
				labels := ""
				for i, label := range m.Label {
					if i > 0 {
						labels += ","
					}
					labels += fmt.Sprintf(`%s="%s"`, label.GetName(), label.GetValue())
				}
				nameWithLabels += fmt.Sprintf(`{%s}`, labels)
			}

			var value string
			switch {
			case m.Counter != nil:
				value = fmt.Sprintf("%.2f", m.Counter.GetValue())
			case m.Gauge != nil:
				value = fmt.Sprintf("%.2f", m.Gauge.GetValue())
			case m.Summary != nil:
				value = fmt.Sprintf("count: %d\nsum: %.2f", m.Summary.GetSampleCount(), m.Summary.GetSampleSum())
			case m.Histogram != nil:
				value = fmt.Sprintf("count: %d\nsum: %.2f", m.Histogram.GetSampleCount(), m.Histogram.GetSampleSum())
			default:
				value = "n/a"
			}

			items = append(items, metricItem{
				name:        nameWithLabels,
				description: mf.GetHelp(),
				value:       value,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].FilterValue() < items[j].FilterValue()
	})

	return items
}

var (
	normalDescStyle    = lipgloss.NewStyle().Faint(true).Render
	normalTitleStyle   = lipgloss.NewStyle().Faint(true).Render
	selectedTitleStyle = lipgloss.NewStyle().Faint(false).Foreground(lipgloss.Color("4")).Render
	selectedDescStyle  = lipgloss.NewStyle().Faint(false).Render
)

type itemDelegate struct{}

func (d itemDelegate) Height() int {
	return 3
}

func (d itemDelegate) Spacing() int {
	return 1
}

func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(metricItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	titleStyle := normalTitleStyle
	descStyle := normalDescStyle

	if selected {
		titleStyle = selectedTitleStyle
		descStyle = selectedDescStyle
	}

	fmt.Fprintf(w, "%s\n%s",
		titleStyle(it.Title()),
		descStyle(it.Description()))
}
