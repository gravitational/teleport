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
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

type column struct {
	width   int
	content []string
}

// tableView renders two columns in a table like view
// that has no headings. Content lengths of the columns
// is required to match.
func tableView(width int, first, second column) string {
	if len(first.content) != len(second.content) {
		panic("column content must have equal heights")
	}

	style := lipgloss.NewStyle().
		Width(width)

	leftColumn := lipgloss.NewStyle().
		Width(first.width).
		Align(lipgloss.Left)

	rightColumn := lipgloss.NewStyle().
		Width(second.width).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	var rows []string
	for i := 0; i < len(first.content); i++ {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			leftColumn.Render(first.content[i]),
			rightColumn.Render(second.content[i]),
		))
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// percentileTableView renders a dynamic table like view
// displaying the percentiles of the provided histogram.
func percentileTableView(width int, hist Histogram) string {
	firstColumn := column{
		width: width / 2,
		content: []string{
			"Percentile",
		},
	}

	secondColumn := column{
		width: width / 2,
		content: []string{
			"Latency",
		},
	}

	for p := range hist.Percentiles() {
		firstColumn.content = append(firstColumn.content, humanize.FormatFloat("#,###", p.Percentile)+"%")
		secondColumn.content = append(secondColumn.content, fmt.Sprintf("%v", p.Value))
	}

	return tableView(width, firstColumn, secondColumn)
}

// requestsTableView renders a table like view
// displaying information about backend request stats.
func requestsTableView(height, width int, stats *BackendStats) string {
	style := lipgloss.NewStyle().
		Width(width).
		MaxHeight(height - 10)

	countColumn := lipgloss.NewStyle().
		Width(10).
		Border(lipgloss.RoundedBorder(), false, false, false, false).
		Align(lipgloss.Left)

	frequencyColumn := lipgloss.NewStyle().
		Width(8).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	keyColumn := lipgloss.NewStyle().
		Width(width-18).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Left,
			countColumn.Render("Count"),
			frequencyColumn.Render("Req/Sec"),
			frequencyColumn.Render("Key"),
		),
	}

	for _, req := range stats.SortedTopRequests() {
		var key string
		if req.Key.Range {
			key = "®" + req.Key.Key
		} else {
			key = " " + req.Key.Key
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			countColumn.Render(humanize.FormatFloat("", float64(req.Count))),
			frequencyColumn.Render(humanize.FormatFloat("", req.GetFreq())),
			keyColumn.Render(key),
		))
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// eventsTableView renders a table like view
// displaying information about watcher event stats.
func eventsTableView(height, width int, stats *WatcherStats) string {
	style := lipgloss.NewStyle().
		Width(width).
		MaxHeight(height - 10)

	countColumn := lipgloss.NewStyle().
		Width(10).
		Border(lipgloss.RoundedBorder(), false, false, false, false).
		Align(lipgloss.Left)

	frequencyColumn := lipgloss.NewStyle().
		Width(8).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	sizeColumn := lipgloss.NewStyle().
		Width(8).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	resourceColumn := lipgloss.NewStyle().
		Width(width-18).
		Border(lipgloss.RoundedBorder(), false, false, false, true).
		Align(lipgloss.Left)

	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Left,
			countColumn.Render("Count"),
			frequencyColumn.Render("Req/Sec"),
			frequencyColumn.Render("Avg Size"),
			frequencyColumn.Render("Resource"),
		),
	}

	for _, event := range stats.SortedTopEvents() {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			countColumn.Render(humanize.FormatFloat("", float64(event.Count))),
			frequencyColumn.Render(humanize.FormatFloat("", event.GetFreq())),
			sizeColumn.Render(humanize.FormatFloat("", event.AverageSize())),
			resourceColumn.Render(event.Resource),
		))
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}
