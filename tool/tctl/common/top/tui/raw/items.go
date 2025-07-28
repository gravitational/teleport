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
	"strings"

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

func updateListPreserveFilterAndSelection(l *list.Model, newItems []list.Item) {
	// 1. Save the current filter input and selection
	// currentFilter := l.FilterValue()
	selectedItem := l.SelectedItem()

	// 2. Build filter terms from new items
	terms := make([]string, len(newItems))
	for i, item := range newItems {
		terms[i] = item.FilterValue()
	}

	// 3. Apply filter with new terms
	// l.SetFilterText(currentFilter)

	// 4. Set new items (needed to update full item list)
	l.SetItems(newItems)

	// 5. Reselect previously selected item if it still exists
	for i, item := range l.VisibleItems() {
		if selectedItem != nil && item.FilterValue() == selectedItem.FilterValue() {
			l.Select(i)
			break
		}
	}
}

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
	titleStyle        = lipgloss.NewStyle().Bold(true)
	descStyle         = lipgloss.NewStyle().Faint(true)
	selectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	selectedDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type itemDelegate struct{}

func (d itemDelegate) Height() int {
	// Height will be dynamic in Spacing; use DelegateWithHeightFunc for per-metricItem height
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

	title := it.Title()
	desc := it.Description()
	descLines := strings.Split(desc, "\n")
	renderedDesc := descStyle.Render(strings.Join(descLines, "\n"))

	if index == m.Index() {
		title = selectedStyle.Render(title)
		renderedDesc = selectedDescStyle.Render(strings.Join(descLines, "\n"))
	}

	w.Write([]byte(fmt.Sprintf("%s\n%s", title, renderedDesc)))
}
