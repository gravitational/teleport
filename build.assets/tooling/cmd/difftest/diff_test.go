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
