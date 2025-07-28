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
	"sort"

	"github.com/dustin/go-humanize"
	dto "github.com/prometheus/client_model/go"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gravitational/teleport/tool/tctl/common/top/tui/common"
)

type metricItem struct {
	name        string
	description string
	value       string
}

func (m metricItem) Title() string       { return m.name }
func (m metricItem) Description() string { return m.value }
func (m metricItem) FilterValue() string { return m.name }

func getLabelString(labels []*dto.LabelPair) string {
	out := ""
	for i, label := range labels {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf(`%s="%s"`, label.GetName(), label.GetValue())
	}

	return fmt.Sprintf("{%s}", out)
}

func listItemFromPromMetric(mf *dto.MetricFamily, m *dto.Metric) list.Item {
	value := "n/a"

	switch mf.GetType() {
	case dto.MetricType_COUNTER:
		value = humanize.FormatFloat("", m.GetCounter().GetValue())
	case dto.MetricType_GAUGE:
		value = humanize.FormatFloat("", m.GetGauge().GetValue())
	case dto.MetricType_SUMMARY:
		value = fmt.Sprintf("count: %d sum: %s",
			m.GetSummary().GetSampleCount(),
			humanize.FormatFloat("", m.GetSummary().GetSampleSum()),
		)
	case dto.MetricType_HISTOGRAM:
		// List view does not allow enough space to make historgram buckets meaningful, only show sum and count.
		value = fmt.Sprintf("count: %d sum: %s",
			m.GetHistogram().GetSampleCount(),
			humanize.FormatFloat("", m.GetHistogram().GetSampleSum()),
		)
	}

	return metricItem{
		name:        mf.GetName() + getLabelString(m.GetLabel()),
		description: mf.GetHelp(),
		value:       value,
	}
}

func convertMetricsToItems(msg common.MetricsMsg) []list.Item {
	items := []list.Item{}

	for _, mf := range msg {
		for _, m := range mf.GetMetric() {
			items = append(items, listItemFromPromMetric(mf, m))
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].FilterValue() < items[j].FilterValue()
	})

	return items
}
