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
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/list"
	"github.com/dustin/go-humanize"
	dto "github.com/prometheus/client_model/go"
)

// metricItem implements [list.DefaultItem] interface to allow rendering via [list.DefaultDelegate]
type metricItem struct {
	name  string
	value string
}

func (m metricItem) Title() string       { return m.name }
func (m metricItem) Description() string { return m.value }
func (m metricItem) FilterValue() string { return m.name }

// getMetricLabelString formats prometheus labels into a readable string.
//
// Example for multiple labels:
//
//	"{label_key1=\"value1\",labelkey2=\"value2\"}"
//
// Empty slice will return:
//
//	""
func getMetricLabelString(labels []*dto.LabelPair) string {
	var out string

	if len(labels) == 0 {
		return ""
	}

	for i, label := range labels {
		if i > 0 {
			out += ","
		}
		out += label.GetName() + "=\"" + label.GetValue() + "\""
	}

	return "{" + out + "}"
}

// metricItemFromPromMetric returns a single [list.Item] from a prometheus metric family and a single metric.
func metricItemFromPromMetric(mf *dto.MetricFamily, m *dto.Metric) list.Item {
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
		name:  mf.GetName() + getMetricLabelString(m.GetLabel()),
		value: value,
	}
}

// convertMetricsToItems converts a [metricsMsg] into a sorted slice of [list.Item] to be used in a list view.
func convertMetricsToItems(msg metricsMsg) []list.Item {

	var itemCount int
	for _, mf := range msg {
		itemCount += len(mf.GetMetric())
	}

	items := make([]list.Item, 0, itemCount)

	for _, mf := range msg {
		for _, m := range mf.GetMetric() {
			items = append(items, metricItemFromPromMetric(mf, m))
		}
	}

	// Sort the item list to keep the display order consistent.
	slices.SortFunc(items, func(i, j list.Item) int {
		return cmp.Compare(i.FilterValue(), j.FilterValue())
	})

	return items
}
