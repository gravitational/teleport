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
