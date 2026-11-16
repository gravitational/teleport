/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package asciitable implements a simple ASCII table formatter for printing
// tabular values into a text terminal.
package asciitable

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"golang.org/x/term"
)

// Layout constants, for callers sizing columns against a terminal width: a
// column occupies max(MinColumnWidth, widest cell+ColumnPadding).
const (
	// MinColumnWidth is the narrowest a column renders, whatever its contents.
	MinColumnWidth = 5
	// ColumnPadding follows every cell, including the last one on a row.
	ColumnPadding = 1
	// TruncationSuffix is appended to truncated cells.
	TruncationSuffix = "..."
)

// Column represents a column in the table.
type Column struct {
	Title         string
	MaxCellLength int
	FootnoteLabel string
	width         int

	// Priority sets truncation order, highest first.
	// Equal priorities share a width limit.
	Priority int
	// MinWidth is the minimum content width in bytes, capped at full width.
	// Zero uses the larger of the header and truncation minimum widths.
	MinWidth int
	// OmitTruncated hides the column if all nonempty cells would be truncated.
	OmitTruncated bool
	// Hidden hides the column in all layouts.
	Hidden bool

	maxContentWidth int // bytes including suffix and footnote label; zero uses MaxCellLength
	omitted         bool
}

// Table holds tabular values in a rows and columns format.
type Table struct {
	columns   []Column
	rows      [][]string
	footnotes map[string]string
}

// MakeHeadlessTable creates a new instance of the table without any column names.
// The number of columns is required.
func MakeHeadlessTable(columnCount int) Table {
	return Table{
		columns:   make([]Column, columnCount),
		rows:      make([][]string, 0),
		footnotes: make(map[string]string),
	}
}

// MakeTable creates a new instance of the table with given column
// names. Optionally rows to be added to the table may be included.
func MakeTable(headers []string, rows ...[]string) Table {
	t := MakeHeadlessTable(len(headers))
	for i := range t.columns {
		t.columns[i].Title = headers[i]
		t.columns[i].width = len(headers[i])
	}
	for _, row := range rows {
		t.AddRow(row)
	}
	return t
}

// MakeTableWithTruncatedColumn creates a table where the column
// matching truncatedColumn will be shortened to account for terminal
// width.
func MakeTableWithTruncatedColumn(columnOrder []string, rows [][]string, truncatedColumn string) Table {
	width, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil || width == 0 {
		width = 80
	}
	truncatedColMinSize := 16
	maxColWidth := (width - truncatedColMinSize) / (len(columnOrder) - 1)
	t := MakeTable([]string{})
	totalLen := 0
	columns := []Column{}

	for collIndex, colName := range columnOrder {
		column := Column{
			Title:         colName,
			MaxCellLength: len(colName),
		}
		if colName == truncatedColumn { // truncated column is handled separately in next loop
			columns = append(columns, column)
			continue
		}
		for _, row := range rows {
			cellLen := row[collIndex]
			if len(cellLen) > column.MaxCellLength {
				column.MaxCellLength = len(cellLen)
			}
		}
		if column.MaxCellLength > maxColWidth {
			column.MaxCellLength = maxColWidth
			totalLen += column.MaxCellLength + len(TruncationSuffix) + ColumnPadding // "...<space>"
		} else {
			totalLen += column.MaxCellLength + ColumnPadding // +1 for column separator
		}
		columns = append(columns, column)
	}

	for _, column := range columns {
		if column.Title == truncatedColumn {
			column.MaxCellLength = max(width-totalLen-len(TruncationSuffix)-ColumnPadding, 0)
		}
		t.AddColumn(column)
	}

	for _, row := range rows {
		t.AddRow(row)
	}
	return t
}

// FitToWidth truncates or omits columns to fit a byte budget.
// Headers and minimum widths can exceed the budget.
func (t *Table) FitToWidth(width int) {
	measurements := t.measureColumns()
	for level := len(measurements.priorities) - 1; level >= 0 && measurements.totalWidth > width; level-- {
		t.shrinkPriorityLevel(measurements.priorities[level], width, &measurements)
	}
	for i, columnWidth := range measurements.widths {
		t.columns[i].omitted = columnWidth == 0
		t.columns[i].maxContentWidth = columnWidth
		t.columns[i].width = columnWidth
	}
}

type columnMeasurements struct {
	widths           []int
	floors           []int
	minContentWidths []int
	priorities       []int
	totalWidth       int
}

// measureColumns measures visible columns in bytes and sorts any truncation
// priorities. Its reported total width is inclusive of column padding.
func (t *Table) measureColumns() columnMeasurements {
	widths := make([]int, len(t.columns))
	floors := make([]int, len(t.columns))
	minContentWidths := make([]int, len(t.columns))
	var priorities []int
	total := 0
	for i, column := range t.columns {
		if column.Hidden {
			continue
		}
		column.maxContentWidth = 0
		contentWidth := 0
		for _, row := range t.rows {
			if i < len(row) && row[i] != "" {
				cell, _ := column.truncateCell(row[i])
				cellWidth := len(cell)
				contentWidth = max(contentWidth, cellWidth)
				if minContentWidths[i] == 0 || cellWidth < minContentWidths[i] {
					minContentWidths[i] = cellWidth
				}
			}
		}
		widths[i] = max(contentWidth, len(column.Title), MinColumnWidth-ColumnPadding)
		truncationWidth := len(TruncationSuffix) + 1
		if column.FootnoteLabel != "" {
			truncationWidth += len(column.FootnoteLabel) + 1
		}
		floors[i] = min(widths[i], max(len(column.Title), MinColumnWidth-ColumnPadding, truncationWidth, column.MinWidth))
		priorities = append(priorities, column.Priority)
		total += widths[i] + ColumnPadding
	}
	slices.Sort(priorities)
	priorities = slices.Compact(priorities)
	return columnMeasurements{
		widths:           widths,
		floors:           floors,
		minContentWidths: minContentWidths,
		priorities:       priorities,
		totalWidth:       total,
	}
}

// shrinkPriorityLevel shrinks or omits columns at one priority to fit the budget.
// After omissions it re-measures widths and updates the total.
func (t *Table) shrinkPriorityLevel(priority, width int, measurements *columnMeasurements) {
	widths, floors := measurements.widths, measurements.floors
	for measurements.totalWidth > width {
		// find the largest fitting shared width limit
		limit := sort.Search(slices.Max(widths), func(limit int) bool {
			candidateWidth := measurements.totalWidth
			for i, column := range t.columns {
				if column.Priority == priority {
					candidateWidth -= widths[i] - max(floors[i], min(widths[i], limit))
				}
			}
			return candidateWidth > width
		})
		limit = max(0, limit-1)
		removed := false
		for i, column := range t.columns {
			if column.Priority == priority && column.OmitTruncated && widths[i] > 0 && measurements.minContentWidths[i] > max(floors[i], limit) {
				measurements.totalWidth -= widths[i] + ColumnPadding
				widths[i], floors[i] = 0, 0
				removed = true
			}
		}
		if removed {
			continue
		}
		for i, column := range t.columns {
			if column.Priority == priority {
				nextWidth := max(floors[i], min(widths[i], limit))
				measurements.totalWidth -= widths[i] - nextWidth
				widths[i] = nextWidth
			}
		}
		break
	}
}

// AddColumn adds a column to the table's structure.
func (t *Table) AddColumn(c Column) {
	c.width = len(c.Title)
	t.columns = append(t.columns, c)
}

// AddRow adds a row of cells to the table.
func (t *Table) AddRow(row []string) {
	limit := min(len(row), len(t.columns))
	for i := range limit {
		cell, _ := t.columns[i].truncateCell(row[i])
		t.columns[i].width = max(len(cell), t.columns[i].width)
	}
	t.rows = append(t.rows, row[:limit])
}

// AddFootnote adds a footnote for referencing from truncated cells.
func (t *Table) AddFootnote(label string, note string) {
	t.footnotes[label] = note
}

func (column Column) truncateCell(cell string) (string, bool) {
	limit := column.MaxCellLength
	if column.maxContentWidth > 0 {
		if (limit == 0 || len(cell) <= limit) && len(cell) <= column.maxContentWidth {
			return cell, false
		}
		limit = column.maxContentWidth - len(TruncationSuffix)
		if column.FootnoteLabel != "" {
			limit -= len(column.FootnoteLabel) + 1
		}
		if column.MaxCellLength > 0 {
			limit = min(limit, column.MaxCellLength)
		}
		limit = max(limit, 0)
		for limit > 0 && !utf8.RuneStart(cell[limit]) {
			limit--
		}
	} else if limit == 0 || len(cell) <= limit {
		return cell, false
	}
	truncatedCell := cell[:limit] + TruncationSuffix
	footnoteLabel := column.FootnoteLabel
	if footnoteLabel == "" {
		return truncatedCell, false
	}
	return fmt.Sprintf("%v %v", truncatedCell, footnoteLabel), true
}

// AsBuffer returns a *bytes.Buffer with the printed output of the table.
//
// TODO(nklaassen): delete this, all calls either immediately copy the buffer to
// another writer or just call .String() once.
func (t *Table) AsBuffer() *bytes.Buffer {
	var buffer bytes.Buffer
	// Writes to bytes.Buffer never return an error.
	_ = t.WriteTo(&buffer)
	return &buffer
}

func (t *Table) String() string {
	var sb strings.Builder
	// Writes to strings.Builder never return an error.
	_ = t.WriteTo(&sb)
	return sb.String()
}

// WriteTo writes the full table to [w] or else returns an error.
func (t *Table) WriteTo(w io.Writer) error {
	writer := tabwriter.NewWriter(w, MinColumnWidth, 0, ColumnPadding, ' ', 0)
	visible := 0
	for _, column := range t.columns {
		if !column.Hidden && !column.omitted {
			visible++
		}
	}
	template := strings.Repeat("%v\t", visible)

	// Header and separator.
	if !t.IsHeadless() {
		var colh []any
		var cols []any

		for _, col := range t.columns {
			if col.Hidden || col.omitted {
				continue
			}
			colh = append(colh, col.Title)
			cols = append(cols, strings.Repeat("-", col.width))
		}
		if _, err := fmt.Fprintf(writer, template+"\n", colh...); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintf(writer, template+"\n", cols...); err != nil {
			return trace.Wrap(err)
		}
	}

	// Body.
	footnoteLabels := make(map[string]struct{})
	for _, row := range t.rows {
		var rowi []any
		for i := range row {
			if t.columns[i].Hidden || t.columns[i].omitted {
				continue
			}
			cell, addFootnote := t.columns[i].truncateCell(row[i])
			if addFootnote {
				footnoteLabels[t.columns[i].FootnoteLabel] = struct{}{}
			}
			rowi = append(rowi, cell)
		}
		if _, err := fmt.Fprintf(writer, template+"\n", rowi...); err != nil {
			return trace.Wrap(err)
		}
	}

	// Footnotes.
	for label := range footnoteLabels {
		if _, err := fmt.Fprintln(writer); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(writer, label, t.footnotes[label]); err != nil {
			return trace.Wrap(err)
		}
	}

	writer.Flush()
	return nil
}

// IsHeadless returns true if none of the table title cells contains any text.
func (t *Table) IsHeadless() bool {
	for i := range t.columns {
		if !t.columns[i].Hidden && !t.columns[i].omitted && len(t.columns[i].Title) > 0 {
			return false
		}
	}
	return true
}

// SortRowsBy sorts the table rows with the given column indices as the sorting
// key, optionally performing a stable sort. Column indices out of range are
// ignored - it is the caller's responsibility to ensure the indices are in
// range.
func (t *Table) SortRowsBy(colIdxKey []int, stable bool) {
	lessFn := func(a, b []string) int {
		for _, col := range colIdxKey {
			limit := min(len(a), len(b))
			if col >= limit {
				continue
			}
			if a[col] != b[col] {
				return strings.Compare(a[col], b[col])
			}
		}
		return 0 // Rows are equal.
	}
	if stable {
		slices.SortStableFunc(t.rows, lessFn)
	} else {
		slices.SortFunc(t.rows, lessFn)
	}
}
