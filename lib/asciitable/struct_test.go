/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestMakeColumnsAndRows(t *testing.T) {
	type row struct {
		Name       string
		ResourceID string
	}

	rows := []row{
		{Name: "n1", ResourceID: "id1"},
		{Name: "n2", ResourceID: "id2"},
	}

	cols, data, err := MakeColumnsAndRows(rows, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"Name", "Resource ID"}, cols)
	require.Equal(t, [][]string{
		{"n1", "id1"},
		{"n2", "id2"},
	}, data)
}

func TestMakeColumnsAndRowsWithTagsAndSkip(t *testing.T) {
	type row struct {
		Name       string `asciitable:"Custom Name"`
		Skip       string `asciitable:"-"`
		ResourceID string `asciitable:"Resource ID"`
	}

	rows := []row{
		{Name: "n1", Skip: "skip1", ResourceID: "id1"},
		{Name: "n2", Skip: "skip2", ResourceID: "id2"},
	}

	cols, data, err := MakeColumnsAndRows(rows, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"Custom Name", "Resource ID"}, cols)
	require.Equal(t, [][]string{
		{"n1", "id1"},
		{"n2", "id2"},
	}, data)
}

func TestMakeColumnsAndRowsIncludeColumns(t *testing.T) {
	type row struct {
		Name       string
		Hostname   string
		Labels     string
		ResourceID string
	}

	rows := []row{
		{Name: "n1", Hostname: "h1", Labels: "a=1", ResourceID: "id1"},
		{Name: "n2", Hostname: "h2", Labels: "b=2", ResourceID: "id2"},
	}

	cols, data, err := MakeColumnsAndRows(rows, []string{"Name", "Labels"})
	require.NoError(t, err)

	require.Equal(t, []string{"Name", "Labels"}, cols)
	require.Equal(t, [][]string{
		{"n1", "a=1"},
		{"n2", "b=2"},
	}, data)
}

func TestMakeColumnsAndRowsIncludeColumnsWithTags(t *testing.T) {
	type row struct {
		Name       string `asciitable:"Custom Name"`
		ResourceID string `asciitable:"Resource ID"`
	}

	rows := []row{
		{Name: "n1", ResourceID: "id1"},
		{Name: "n2", ResourceID: "id2"},
	}

	cols, data, err := MakeColumnsAndRows(rows, []string{"Custom Name"})
	require.NoError(t, err)

	require.Equal(t, []string{"Custom Name"}, cols)
	require.Equal(t, [][]string{
		{"n1"},
		{"n2"},
	}, data)
}

func TestMakeColumnsAndRowsCamelCaseLongName(t *testing.T) {
	type row struct {
		VeryLongFieldName string
	}

	rows := []row{
		{VeryLongFieldName: "value1"},
	}

	cols, data, err := MakeColumnsAndRows(rows, nil)
	require.NoError(t, err)

	require.Len(t, cols, 1)
	require.Equal(t, "Very Long Field Name", cols[0])
	require.Equal(t, [][]string{{"value1"}}, data)
}

func TestMakeColumnsAndRowsEmptySlice(t *testing.T) {
	type row struct {
		Name       string
		ResourceID string
	}

	var rows []row

	cols, data, err := MakeColumnsAndRows(rows, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"Name", "Resource ID"}, cols)
	require.Empty(t, data)
}

func TestMakeColumnsAndRowsNonStructType(t *testing.T) {
	rows := []int{1, 2, 3}

	cols, data, err := MakeColumnsAndRows(rows, nil)
	require.Error(t, err)
	require.Nil(t, cols)
	require.Nil(t, data)

	var traceErr trace.Error
	ok := errors.As(err, &traceErr)
	require.True(t, ok)

	require.Contains(t, err.Error(), "only slices of struct are supported")
}

func TestMakeColumnsAndRowsIncludeColumnsUnknown(t *testing.T) {
	type row struct {
		Name string
	}

	rows := []row{
		{Name: "n1"},
	}

	cols, data, err := MakeColumnsAndRows(rows, []string{"Unknown"})
	require.NoError(t, err)

	require.Empty(t, cols)
	require.Equal(t, [][]string{{}}, data)
	require.Len(t, data, 1)
}
