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
	lockPath := path + ".lock"
	require.NoError(t, os.WriteFile(lockPath, nil, 0o600))

	require.NoError(t, removeMCPOAuthCredentials(path))

	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "credentials file must be removed")
	_, err = os.Stat(lockPath)
	require.True(t, os.IsNotExist(err), "lock file must be removed")

	// Other servers' credentials stay untouched.
	_, err = loadMCPOAuthCredentials(otherPath)
	require.NoError(t, err)
}

func TestRemoveMCPOAuthCredentialsNotFound(t *testing.T) {
	err := removeMCPOAuthCredentials(filepath.Join(t.TempDir(), "nope.json"))
	require.True(t, trace.IsNotFound(err))
}
