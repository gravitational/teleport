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
