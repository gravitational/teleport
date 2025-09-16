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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/utils"
)

func TestDestinationDirectory_Lock(t *testing.T) {
	dd := &DestinationDirectory{
		Path:     t.TempDir(),
		Symlinks: botfs.SymlinksInsecure,
		ACLs:     botfs.ACLOff,
	}

	// Successful lock
	unlock, err := dd.TryLock()
	require.NoError(t, err)

	// Another lock should fail
	_, err = dd.TryLock()
	require.ErrorIs(t, err, utils.ErrUnsuccessfulLockTry)

	// Release the lock
	require.NoError(t, unlock())

	// Trying to lock again should succeed
	unlock, err = dd.TryLock()
	require.NoError(t, err)

	// Release the lock
	require.NoError(t, unlock())
}

func TestDestinationDirectory_YAML(t *testing.T) {
	tests := []testYAMLCase[DestinationDirectory]{
		{
			name: "full",
			in: DestinationDirectory{
				Path:     "/my/path",
				ACLs:     botfs.ACLRequired,
				Symlinks: botfs.SymlinksSecure,
			},
		},
		{
			name: "minimal",
			in: DestinationDirectory{
				Path: "/my/path",
			},
		},
	}
	testYAML(t, tests)
}
