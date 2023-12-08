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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testChartPath    = "./testdata"
	testSnapshotPath = "./testdata/expected-output.mdx"
)

func Test_parseAndRender(t *testing.T) {
	// Test setup: we load the fixtures
	expected, err := os.ReadFile(testSnapshotPath)
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	// Test execution: we render templates, expect no error and check result
	actual, err := parseAndRender(testChartPath)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
