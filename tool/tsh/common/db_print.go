/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
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
	for i := 0; i < t.NumField(); i++ {
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
		for i := 0; i < v.NumField(); i++ {
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
}

func (cfg printDatabaseTableConfig) excludeColumns() (out []string) {
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
	excludeColumns := cfg.excludeColumns()

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
	if database.SupportsAutoUsers() && database.GetAdminUser().Name != "" {
		// may happen if fetching the role set failed for any reason.
		if accessChecker == nil {
			return "(unknown)"
		}

		autoUser, roles, err := accessChecker.CheckDatabaseRoles(database)
		if err != nil {
			log.Warnf("Failed to CheckDatabaseRoles for database %v: %v.", database.GetName(), err)
		} else if autoUser.IsEnabled() {
			return fmt.Sprintf("%v", roles)
		}
	}
	return ""
}
