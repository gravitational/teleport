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
	"context"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/services"
)

type databaseTableRow struct {
	Proxy         string
	Cluster       string
	DisplayName   string `title:"Name"`
	Description   string
	Protocol      string
	Type          string
	URI           string
	AllowedUsers  string
	DatabaseRoles string
	Labels        string
	Connect       string
}

func makeTableColumnTitles(row any) (out []string) {
	// Regular expression to convert from "DatabaseRoles" to "Database Roles" etc.
	re := regexp.MustCompile(`([a-z])([A-Z])`)

	t := reflect.TypeOf(row)
	for i := range t.NumField() {
		field := t.Field(i)
		title := field.Tag.Get("title")
		if title == "" {
			title = re.ReplaceAllString(field.Name, "${1} ${2}")
		}
		out = append(out, title)
	}
	return out
}

func makeTableRows[T any](rows []T) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		var columnValues []string
		v := reflect.ValueOf(row)
		for i := range v.NumField() {
			columnValues = append(columnValues, fmt.Sprintf("%v", v.Field(i)))
		}
		out = append(out, columnValues)
	}
	return out
}

type printDatabaseTableConfig struct {
	writer              io.Writer
	rows                []databaseTableRow
	showProxyAndCluster bool
	verbose             bool
	// includeColumns specifies a whitelist of columns to include. verbose and
	// showProxyAndCluster are ignored when includeColumns is provided.
	includeColumns []string
}

func (cfg printDatabaseTableConfig) excludeColumns(allColumns []string) (out []string) {
	if len(cfg.includeColumns) > 0 {
		for _, column := range allColumns {
			if !slices.Contains(cfg.includeColumns, column) {
				out = append(out, column)
			}
		}
		return
	}
	if !cfg.showProxyAndCluster {
		out = append(out, "Proxy", "Cluster")
	}
	if !cfg.verbose {
		out = append(out, "Protocol", "Type", "URI", "Database Roles")
	}
	return out
}

func printDatabaseTable(cfg printDatabaseTableConfig) {
	allColumns := makeTableColumnTitles(databaseTableRow{})
	rowsWithAllColumns := makeTableRows(cfg.rows)
	excludeColumns := cfg.excludeColumns(allColumns)

	var printColumns []string
	printRows := make([][]string, len(cfg.rows))
	for columnIndex, column := range allColumns {
		if slices.Contains(excludeColumns, column) {
			continue
		}

		printColumns = append(printColumns, column)
		for rowIndex, row := range rowsWithAllColumns {
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

func formatDatabaseRolesForDB(database types.Database, accessChecker services.AccessChecker) string {
	if database.IsAutoUsersEnabled() {
		// may happen if fetching the role set failed for any reason.
		if accessChecker == nil {
			return "(unknown)"
		}

		autoUser, err := accessChecker.DatabaseAutoUserMode(database)
		if err != nil {
			logger.WarnContext(context.Background(), "Failed to get DatabaseAutoUserMode for database",
				"database", database.GetName(),
				"error", err,
			)
			return ""
		} else if !autoUser.IsEnabled() {
			return ""
		}

		roles, err := accessChecker.CheckDatabaseRoles(database, nil)
		if err != nil {
			logger.WarnContext(context.Background(), "Failed to CheckDatabaseRoles for database",
				"database", database.GetName(),
				"error", err,
			)
			return ""
		}
		return fmt.Sprintf("%v", roles)
	}
	return ""
}

func shouldShowListDatabasesHint(cf *CLIConf, numRows int) bool {
	selector := newDatabaseResourceSelectors(cf)

	return numRows >= minNumRowsToShowListDatabasesHint &&
		cf.command == "db ls" &&
		cf.SearchKeywords == "" &&
		selector.IsEmpty()
}

func maybeShowListDatabasesHint(cf *CLIConf, w io.Writer, numRows int) {
	if !shouldShowListDatabasesHint(cf, numRows) {
		return
	}

	fmt.Fprint(w, listDatabaseHint)
}

// minNumRowsToShowListDatabasesHint is an arbitrary number selected to show
// filtering hint for `tsh db ls` command when too many databases are listed.
const minNumRowsToShowListDatabasesHint = 20

const listDatabaseHint = "" +
	"hint: use 'tsh db ls --search foo,bar' to search keywords\n" +
	"      use 'tsh db ls key1=value1,key2=value2' to filter databases by labels\n\n"
