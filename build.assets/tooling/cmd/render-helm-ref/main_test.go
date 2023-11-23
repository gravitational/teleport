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
