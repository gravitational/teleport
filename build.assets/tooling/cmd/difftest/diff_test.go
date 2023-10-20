/*
Copyright 2022 Gravitational, Inc.

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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleDiff(t *testing.T) {
	diff, err := os.ReadFile(filepath.Join("testdata", "example.diff"))
	require.NoError(t, err)

	files, err := getChangedTestFilesFromDiff(string(diff), []string{}, []string{})

	require.NoError(t, err)
	require.Len(t, files, 6)

	require.Contains(t, files, "access/email/email_test.go")
	require.Contains(t, files, "access/jira/jira_test.go")
	require.Contains(t, files, "access/mattermost/plugindata_test.go")
	require.Contains(t, files, "access/slack/helpers_test.go")
	require.Contains(t, files, "event-handler/event_handler_test.go")
	require.Contains(t, files, "event-handler/mtls_certs_test.go")
}

func TestExampleDiffWithExclude(t *testing.T) {
	diff, err := os.ReadFile(filepath.Join("testdata", "example.diff"))
	require.NoError(t, err)

	files, err := getChangedTestFilesFromDiff(string(diff), []string{"access/email/*"}, []string{})

	require.NoError(t, err)
	require.Len(t, files, 5)

	require.NotContains(t, files, "access/email/email_test.go")
}

func TestExampleDiffWithInclude(t *testing.T) {
	diff, err := os.ReadFile(filepath.Join("testdata", "example.diff"))
	require.NoError(t, err)

	files, err := getChangedTestFilesFromDiff(string(diff), []string{}, []string{"access/email/*"})

	require.NoError(t, err)
	require.Len(t, files, 1)

	require.Contains(t, files, "access/email/email_test.go")
}
