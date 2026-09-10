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

package common

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestRemoveMCPOAuthCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentry.json")
	otherPath := filepath.Join(dir, "linear.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("token-1", time.Now().Add(time.Hour))))
	require.NoError(t, saveMCPOAuthCredentials(otherPath, newTestCreds("token-2", time.Now().Add(time.Hour))))

	require.NoError(t, removeMCPOAuthCredentials(t.Context(), path, filepath.Join(dir, "mcp_oauth.lock")))

	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "credentials file must be removed")

	_, err = loadMCPOAuthCredentials(otherPath)
	require.NoError(t, err)
}

func TestRemoveMCPOAuthCredentialsNotFound(t *testing.T) {
	dir := t.TempDir()
	err := removeMCPOAuthCredentials(t.Context(), filepath.Join(dir, "nope.json"), filepath.Join(dir, "mcp_oauth.lock"))
	require.True(t, trace.IsNotFound(err))
}

func TestRemoveAllMCPOAuthCredentials(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sentry.oauth.json", "@linear.oauth.json"} {
		require.NoError(t, saveMCPOAuthCredentials(filepath.Join(dir, name), newTestCreds("token", time.Now().Add(time.Hour))))
	}
	other := filepath.Join(dir, "sentry.oauth.json.lock")
	require.NoError(t, os.WriteFile(other, nil, 0o600))

	removed, err := removeAllMCPOAuthCredentials(t.Context(), dir, filepath.Join(dir, "mcp_oauth.lock"))
	require.NoError(t, err)
	require.Equal(t, 2, removed)

	require.NoFileExists(t, filepath.Join(dir, "sentry.oauth.json"))
	require.NoFileExists(t, filepath.Join(dir, "@linear.oauth.json"))
	require.FileExists(t, other)

	removed, err = removeAllMCPOAuthCredentials(t.Context(), filepath.Join(dir, "missing"), filepath.Join(dir, "mcp_oauth.lock"))
	require.NoError(t, err)
	require.Zero(t, removed)
}
