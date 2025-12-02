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
	"reflect"
	"regexp"
	"slices"

	"github.com/gravitational/trace"
)

const asciitableTag = "asciitable"

// MakeAsciitableColumnsAndRows converts a slice of structs into column headers and
// row data suitable for use with asciitable.MakeTable.
// T must be a struct type. If T is not a struct, the function returns an error.
func MakeAsciitableColumnsAndRows[T any](rows []T, includeColumns []string) ([]string, [][]string, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		return nil, nil, trace.Errorf("only slices of struct are supported: got slice of %s", t.Kind())
	}

	// Regular expression to convert from "DatabaseRoles" to "Database Roles" etc.
	re := regexp.MustCompile(`([a-z])([A-Z])`)

	type fieldInfo struct {
		index int
		name  string
	}

	var fields []fieldInfo
	var columns []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		header := f.Tag.Get(asciitableTag)
		if header == "-" {
			continue
		}
		if header == "" {
			header = re.ReplaceAllString(f.Name, "${1} ${2}")
		}

		if len(includeColumns) > 0 && !slices.Contains(includeColumns, header) {
			continue
		}

		fields = append(fields, fieldInfo{
			index: i,
			name:  header,
		})
		columns = append(columns, header)
	}

	outRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		v := reflect.ValueOf(row)
		rowValues := make([]string, 0, len(fields))
		for _, fi := range fields {
			rowValues = append(rowValues, fmt.Sprintf("%v", v.Field(fi.index)))
		}
		outRows = append(outRows, rowValues)
	}

	return columns, outRows, nil
}
