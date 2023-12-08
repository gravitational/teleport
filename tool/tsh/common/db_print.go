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

package common

import (
	"fmt"
	"io"

	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib/asciitable"
)

type printDatabaseTableConfig struct {
	writer              io.Writer
	rows                [][]string
	showProxyAndCluster bool
	verbose             bool
	excludeColumns      []string
}

func printDatabaseTable(cfg printDatabaseTableConfig) {
	allColumns := databaseTableColumns()
	if !cfg.showProxyAndCluster {
		cfg.excludeColumns = append(cfg.excludeColumns, "Proxy", "Cluster")
	}
	if !cfg.verbose {
		cfg.excludeColumns = append(cfg.excludeColumns, "Protocol", "Type", "URI")
	}

	var printColumns []string
	printRows := make([][]string, len(cfg.rows))

	for columnIndex, column := range allColumns {
		if slices.Contains(cfg.excludeColumns, column) {
			continue
		}

		printColumns = append(printColumns, column)
		for rowIndex, row := range cfg.rows {
			printRows[rowIndex] = append(printRows[rowIndex], row[columnIndex])
		}
	}

	var t asciitable.Table
	if cfg.verbose {
		t = asciitable.MakeTable(printColumns, printRows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(printColumns, printRows, "Labels")
	}
	fmt.Fprintln(cfg.writer, t.AsBuffer().String())
}

func databaseTableColumns() []string {
	return []string{"Proxy", "Cluster", "Name", "Description", "Protocol", "Type", "URI", "Allowed Users", "Labels", "Connect"}
}
