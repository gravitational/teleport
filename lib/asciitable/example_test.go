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

package asciitable

import (
	"fmt"
)

func ExampleMakeTable() {
	// Create a table with three column headers.
	t := MakeTable([]string{"Token", "Type", "Expiry Time (UTC)"})

	// Add in multiple rows.
	t.AddRow([]string{"b53bd9d3e04add33ac53edae1a2b3d4f", "auth", "30 Aug 18 23:31 UTC"})
	t.AddRow([]string{"5ecde0ca17824454b21937109df2c2b5", "node", "30 Aug 18 23:31 UTC"})
	t.AddRow([]string{"9333929146c08928a36466aea12df963", "trusted_cluster", "30 Aug 18 23:33 UTC"})

	// Write the table to stdout.
	fmt.Println(t.AsBuffer().String())
}

func ExampleMakeColumnsAndRows() {
	type dbResourceRow struct {
		DatabaseName string `asciitable:"DB Name"` // This column will appear in the table under a custom name.
		Skip         string `asciitable:"-"`       // This column will be skipped entirely.
		ResourceID   string // It will derive the name "Resource ID"
	}

	rows := []dbResourceRow{
		{DatabaseName: "orders", Skip: "ignored", ResourceID: "db-1"},
		{DatabaseName: "users", Skip: "ignored", ResourceID: "db-2"},
	}

	// Build table columns + rows.
	cols, data, err := MakeColumnsAndRows(rows, nil)
	if err != nil {
		panic(err)
	}

	// Create asciitable table.
	table := MakeTable(cols, data...)

	// Write the table to stdout.
	fmt.Println(table.AsBuffer().String())
}
