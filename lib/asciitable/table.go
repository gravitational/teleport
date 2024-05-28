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
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
)

// Column represents a column in the table.
type Column struct {
	Title         string
	MaxCellLength int
	FootnoteLabel string
	width         int
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
			totalLen += column.MaxCellLength + 4 // "...<space>"
		} else {
			totalLen += column.MaxCellLength + 1 // +1 for column separator
		}
		columns = append(columns, column)
	}

	for _, column := range columns {
		if column.Title == truncatedColumn {
			column.MaxCellLength = max(width-totalLen-len("... "), 0)
		}
		t.AddColumn(column)
	}

	for _, row := range rows {
		t.AddRow(row)
	}
	return t
}

// AddColumn adds a column to the table's structure.
func (t *Table) AddColumn(c Column) {
	c.width = len(c.Title)
	t.columns = append(t.columns, c)
}

// AddRow adds a row of cells to the table.
func (t *Table) AddRow(row []string) {
	limit := min(len(row), len(t.columns))
	for i := 0; i < limit; i++ {
		cell, _ := t.truncateCell(i, row[i])
		t.columns[i].width = max(len(cell), t.columns[i].width)
	}
	t.rows = append(t.rows, row[:limit])
}

// AddFootnote adds a footnote for referencing from truncated cells.
func (t *Table) AddFootnote(label string, note string) {
	t.footnotes[label] = note
}

// truncateCell truncates cell contents to shorter than the column's
// MaxCellLength, and adds the footnote symbol if specified.
func (t *Table) truncateCell(colIndex int, cell string) (string, bool) {
	maxCellLength := t.columns[colIndex].MaxCellLength
	if maxCellLength == 0 || len(cell) <= maxCellLength {
		return cell, false
	}
	truncatedCell := fmt.Sprintf("%v...", cell[:maxCellLength])
	footnoteLabel := t.columns[colIndex].FootnoteLabel
	if footnoteLabel == "" {
		return truncatedCell, false
	}
	return fmt.Sprintf("%v %v", truncatedCell, footnoteLabel), true
}

// AsBuffer returns a *bytes.Buffer with the printed output of the table.
func (t *Table) AsBuffer() *bytes.Buffer {
	var buffer bytes.Buffer

	writer := tabwriter.NewWriter(&buffer, 5, 0, 1, ' ', 0)
	template := strings.Repeat("%v\t", len(t.columns))

	// Header and separator.
	if !t.IsHeadless() {
		var colh []interface{}
		var cols []interface{}

		for _, col := range t.columns {
			colh = append(colh, col.Title)
			cols = append(cols, strings.Repeat("-", col.width))
		}
		fmt.Fprintf(writer, template+"\n", colh...)
		fmt.Fprintf(writer, template+"\n", cols...)
	}

	// Body.
	footnoteLabels := make(map[string]struct{})
	for _, row := range t.rows {
		var rowi []interface{}
		for i := range row {
			cell, addFootnote := t.truncateCell(i, row[i])
			if addFootnote {
				footnoteLabels[t.columns[i].FootnoteLabel] = struct{}{}
			}
			rowi = append(rowi, cell)
		}
		fmt.Fprintf(writer, template+"\n", rowi...)
	}

	// Footnotes.
	for label := range footnoteLabels {
		fmt.Fprintln(writer)
		fmt.Fprintln(writer, label, t.footnotes[label])
	}

	writer.Flush()
	return &buffer
}

// IsHeadless returns true if none of the table title cells contains any text.
func (t *Table) IsHeadless() bool {
	for i := range t.columns {
		if len(t.columns[i].Title) > 0 {
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
