/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This module implements a simple ASCII table formatter for printing
tabular values into a text terminal.

Example usage:

func main() {
	// building a table
	t := MakeTable([]string{"Name", "Motto", "Age"})
	t.AddRow([]string{"Joe Forrester", "Trains are much better than cars", "40"})
	t.AddRow([]string{"Jesus", "Read the bible", "2018"})

	// using the table:
	t.WriteTo(os.Stdout)
}

*/

package asciitable

import (
	"bytes"
	"fmt"
	"strings"
)

type column struct {
	width int
	title string
}

type PrintOptions int

type Table struct {
	columns []column
	rows    [][]string
}

// MakeTable creates a new instance of the table with a given title
func MakeTable(headers []string) Table {
	t := MakeHeadlessTable(len(headers))
	for i := range t.columns {
		t.columns[i].title = headers[i]
		t.columns[i].width = len(headers[i])
	}
	return t
}

// MakeTable creates a new instance of the table without a title,
// but the number of columns must be set
func MakeHeadlessTable(columnCount int) Table {
	return Table{
		columns: make([]column, columnCount),
		rows:    make([][]string, 0),
	}
}

// Body returns the fully formatted table body as a buffer
func (t *Table) Body() *bytes.Buffer {
	var (
		padding string
		buf     bytes.Buffer
	)
	for _, row := range t.rows {
		for columnIndex, cell := range row {
			padding = strings.Repeat(" ", t.columns[columnIndex].width-len(cell)+1)
			fmt.Fprintf(&buf, "%s%s", cell, padding)
		}
		fmt.Fprintln(&buf, "")
	}
	return &buf
}

// Header returns the fully formatted header as a buffer
func (t *Table) Header() *bytes.Buffer {
	var (
		buf     bytes.Buffer
		padding string
	)
	for i := range t.columns {
		title := t.columns[i].title
		padding = strings.Repeat(" ", t.columns[i].width-len(title)+1)
		fmt.Fprintf(&buf, "%s%s", title, padding)
	}
	return &buf
}

// ColumnWidths returns the slice of ints that are the widths of each column
func (t *Table) ColumnWidths() []int {
	retval := make([]int, len(t.columns))
	for i := range t.columns {
		retval[i] = t.columns[i].width
	}
	return retval
}

func (t *Table) AddRow(row []string) {
	limit := min(len(row), len(t.columns))
	for i := 0; i < limit; i++ {
		cellWidth := len(row[i])
		t.columns[i].width = max(cellWidth, t.columns[i].width)
	}
	t.rows = append(t.rows, row[:limit])
}

// WriteTo prints the table to the given writer
func (t *Table) AsBuffer() *bytes.Buffer {
	var buf bytes.Buffer

	// the hearder:
	if !t.IsHeadless() {
		fmt.Fprintf(&buf, "%s\n", t.Header().String())
		// the separator:
		for _, w := range t.ColumnWidths() {
			fmt.Fprintf(&buf, "%s ", strings.Repeat("-", w))
		}
		buf.WriteString("\n")
	}

	// the body:
	fmt.Fprintf(&buf, "%s", t.Body().String())
	return &buf
}

// IsHeadless returns 'true' if none of the table title cells contains any text
func (t *Table) IsHeadless() bool {
	total := 0
	for i := range t.columns {
		total += len(t.columns[i].title)
	}
	return total == 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
