/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testChartYaml = `
.version: &version "1.2.3"

name: test-chart
version: *version
appVersion: *version

dependencies:
  - name: other-test-chart
    version: *version
`
	updatedTestChartYaml = `
.version: &version "1.2.4-foobar"

name: test-chart
version: *version
appVersion: *version

dependencies:
  - name: other-test-chart
    version: *version
`
)

func TestUpdateChartVersion(t *testing.T) {
	dir := t.TempDir()
	chart := Chart{
		Name: "test-chart",
		Path: dir,
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(testChartYaml), 0644))
	require.NoError(t, updateChartVersion(t.Context(), chart, "1.2.4-foobar"))
	require.FileExists(t, filepath.Join(dir, "Chart.yaml"))
	updatedChart, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	require.NoError(t, err)
	require.Equal(t, updatedTestChartYaml, string(updatedChart))

}
