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
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"golang.org/x/term"
)

// truncateMode controls how a column's cell text is truncated when it
// exceeds MaxCellLength.
type truncateMode int

const (
	// truncateEnd truncates from the end ("start...").
	truncateEnd truncateMode = iota
	// truncateMiddle truncates from the middle ("start...end").
	truncateMiddle
)

// Column represents a column in the table.
type Column struct {
	Title         string
	MaxCellLength int
	FootnoteLabel string
	truncateMode  truncateMode
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

// terminalWidth returns the width to use for table truncation.
// It checks, in order:
//  1. The COLUMNS environment variable (explicit override).
//  2. The terminal width via term.GetSize (interactive terminal).
//  3. Zero when stdout is not a terminal (pipe/file redirect),
//     signaling that no truncation should be applied.
func terminalWidth() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			return w
		}
		return teleport.DefaultTerminalWidth
	}
	return 0
}

// MakeTableWithTruncatedColumn creates a table that fits the terminal width
// by truncating the named column from the end ("start...").
func MakeTableWithTruncatedColumn(columnOrder []string, rows [][]string, truncatedColumn string) Table {
	return makeTableWithTruncatedColumn(columnOrder, rows, truncatedColumn, truncateEnd)
}

// MakeTableWithEllipsisColumn creates a table that fits the terminal width
// by truncating the named column from the middle ("start...end").
func MakeTableWithEllipsisColumn(columnOrder []string, rows [][]string, ellipsisColumn string) Table {
	return makeTableWithTruncatedColumn(columnOrder, rows, ellipsisColumn, truncateMiddle)
}

func makeTableWithTruncatedColumn(columnOrder []string, rows [][]string, targetColumn string, mode truncateMode) Table {
	width := terminalWidth()
	if width == 0 {
		return MakeTable(columnOrder, rows...)
	}

	const targetColMinSize = 16                      // minimum chars reserved for the target column
	const ellipsisSeparatorWidth = len("... ")        // truncation marker ("...") plus column separator (" ")

	// Each non-target column gets an equal share of the remaining width.
	maxColWidth := (width - targetColMinSize) / (len(columnOrder) - 1)
	usedWidth := 0

	// First pass: measure each non-target column's natural width (capped
	// at maxColWidth) and accumulate the total space they consume.
	colWidths := make(map[int]int, len(columnOrder))
	for colIndex, colName := range columnOrder {
		if colName == targetColumn {
			continue
		}
		w := len(colName)
		for _, row := range rows {
			if len(row[colIndex]) > w {
				w = len(row[colIndex])
			}
		}
		w = min(w, maxColWidth)
		colWidths[colIndex] = w
		if w >= maxColWidth {
			// Column was capped — it will display with a "..." suffix.
			usedWidth += w + ellipsisSeparatorWidth
		} else {
			usedWidth += w + 1 // +1 for column separator
		}
	}

	// Second pass: build the table, giving the target column whatever
	// terminal width remains after the other columns.
	t := MakeTable([]string{})
	for colIndex, colName := range columnOrder {
		column := Column{Title: colName}
		if colName == targetColumn {
			column.MaxCellLength = max(width-usedWidth-ellipsisSeparatorWidth, 0)
			column.truncateMode = mode
		} else {
			column.MaxCellLength = colWidths[colIndex]
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
	for i := range limit {
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
	col := t.columns[colIndex]
	if col.MaxCellLength == 0 || len(cell) <= col.MaxCellLength {
		return cell, false
	}

	var truncatedCell string
	if col.truncateMode == truncateMiddle {
		avail := col.MaxCellLength
		head := (avail + 1) / 2
		tail := avail / 2
		truncatedCell = cell[:head] + "..." + cell[len(cell)-tail:]
	} else {
		truncatedCell = fmt.Sprintf("%v...", cell[:col.MaxCellLength])
	}

	if col.FootnoteLabel == "" {
		return truncatedCell, false
	}
	return fmt.Sprintf("%v %v", truncatedCell, col.FootnoteLabel), true
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
	writer := tabwriter.NewWriter(w, 5, 0, 1, ' ', 0)
	template := strings.Repeat("%v\t", len(t.columns))

	// Header and separator.
	if !t.IsHeadless() {
		var colh []any
		var cols []any

		for _, col := range t.columns {
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
			cell, addFootnote := t.truncateCell(i, row[i])
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
