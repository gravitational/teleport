// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecureCredDir tests that the files can be created in a directory
// secured by secureCredDir, and they are not readable by the current user.
func TestSecureCredDir(t *testing.T) {
	creds, err := newIPCCredentials()
	require.NoError(t, err)

	_, userSID, err := currentUsernameAndSID()
	require.NoError(t, err)

	credPath := t.TempDir()
	require.NoError(t, secureCredDir(credPath, userSID))
	require.NoError(t, creds.client.write(credPath))
	defer func() {
		err := creds.client.remove(credPath)
		assert.NoError(t, err, "deleting credentials")
	}()

	_, err = readCredentials(credPath)
	assert.ErrorIs(t, err, syscall.ERROR_ACCESS_DENIED,
		"expected no access to read creds")

	files := []string{certFileName, keyFileName, caFileName}
	for _, f := range files {
		fp := filepath.Join(credPath, f)
		_, err := os.ReadFile(fp)
		assert.ErrorIs(t, err, syscall.ERROR_ACCESS_DENIED,
			"expected no access to %s", fp)
	}
}
