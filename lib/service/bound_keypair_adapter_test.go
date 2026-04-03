// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package service

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestBoundKeypairStorageAdapter(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	processStorage, err := storage.NewProcessStorage(
		ctx,
		filepath.Join(tempDir, teleport.ComponentProcess),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		backend.Close()
		processStorage.Close()
	})

	process := &TeleportProcess{
		Supervisor: &LocalSupervisor{
			exitContext: t.Context(),
		},
		backend: backend,
		storage: processStorage,
		logger:  logtest.NewLogger(),
	}

	adapter := process.boundKeypairStorageAdapter()

	state := boundkeypair.NewEmptyFSClientState(adapter)

	// Fake a new keypair request, this should update the client state and make
	// it non-empty.
	signer, err := state.RequestNewKeypair(ctx,
		cryptosuites.StaticAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1),
	)
	require.NoError(t, err)

	// Set the new key as active and store it. The state should have a nonempty
	// key.
	require.NoError(t, state.SetActiveKey(signer))
	require.NoError(t, state.Store(ctx))
	require.NotEmpty(t, state.PrivateKeyBytes)

	loadedState, err := boundkeypair.LoadClientState(ctx, adapter)
	require.NoError(t, err)

	require.Equal(t, state.PrivateKeyBytes, loadedState.PrivateKeyBytes)
}
