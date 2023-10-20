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
