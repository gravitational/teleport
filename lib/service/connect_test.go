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

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMakeJoinParams_BoundKeypair(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	staticKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.StaticAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1),
		cryptosuites.BoundKeypairJoining)
	require.NoError(t, err)

	sshPubKey, err := ssh.NewPublicKey(staticKey.Public())
	require.NoError(t, err)

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	privateKeyBytes, err := keys.MarshalPrivateKey(staticKey)
	require.NoError(t, err)

	dir := t.TempDir()
	regSecretPath := filepath.Join(dir, "reg-secret")
	staticKeyPath := filepath.Join(dir, "static-key")

	require.NoError(t, os.WriteFile(regSecretPath, []byte("reg-secret"), 0600))
	require.NoError(t, os.WriteFile(staticKeyPath, privateKeyBytes, 0600))

	for _, tt := range []struct {
		name             string
		mutateConfig     func(*servicecfg.Config)
		assertError      require.ErrorAssertionFunc
		assertJoinParams func(t *testing.T, params *joinclient.JoinParams)
	}{
		{
			name:        "bound keypair not configured",
			assertError: require.NoError,
			assertJoinParams: func(t *testing.T, params *joinclient.JoinParams) {
				require.Nil(t, params.BoundKeypairState)
			},
		},
		{
			name: "bound keypair registration secret value configured",
			mutateConfig: func(c *servicecfg.Config) {
				c.JoinMethod = types.JoinMethodBoundKeypair
				c.JoinParams.BoundKeypair.RegistrationSecretValue = "test"
			},
			assertError: require.NoError,
			assertJoinParams: func(t *testing.T, params *joinclient.JoinParams) {
				require.Equal(t, "test", params.BoundKeypairRegistrationSecret)
				require.NotNil(t, params.BoundKeypairState)

				// Should be initialized but empty.
				require.Empty(t, params.BoundKeypairState.GetClientParams("").PreviousJoinState)
			},
		},
		{
			name: "bound keypair registration secret path configured",
			mutateConfig: func(c *servicecfg.Config) {
				c.JoinMethod = types.JoinMethodBoundKeypair
				c.JoinParams.BoundKeypair.RegistrationSecretPath = regSecretPath
			},
			assertError: require.NoError,
			assertJoinParams: func(t *testing.T, params *joinclient.JoinParams) {
				require.Equal(t, "reg-secret", params.BoundKeypairRegistrationSecret)
				require.NotNil(t, params.BoundKeypairState)

				// Should be initialized but empty.
				require.Empty(t, params.BoundKeypairState.GetClientParams("").PreviousJoinState)
			},
		},
		{
			name: "bound keypair static key configured",
			mutateConfig: func(c *servicecfg.Config) {
				c.JoinMethod = types.JoinMethodBoundKeypair
				c.JoinParams.BoundKeypair.StaticPrivateKeyPath = staticKeyPath
			},
			assertError: require.NoError,
			assertJoinParams: func(t *testing.T, params *joinclient.JoinParams) {
				require.Empty(t, params.BoundKeypairRegistrationSecret)

				// Should be initialized and nonempty.
				state := params.BoundKeypairState
				require.NotNil(t, state)

				// It should be possible to fetch the signer by its public key
				signer, err := state.GetSigner(publicKeyBytes)
				require.NoError(t, err)
				require.NotNil(t, signer)

				// Previous join state should still be empty.
				require.Empty(t, params.BoundKeypairState.GetClientParams("").PreviousJoinState)

				// RequestNewKeypair should fail (impl doesn't support rotation)
				_, err = state.RequestNewKeypair(ctx, cryptosuites.StaticAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1))
				require.ErrorContains(t, err, "do not support automatic rotation")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
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
					exitContext:         t.Context(),
					gracefulExitContext: t.Context(),
				},
				Config:  servicecfg.MakeDefaultConfig(),
				backend: backend,
				storage: processStorage,
				logger:  logtest.NewLogger(),
			}
			process.Config.SetToken("example")

			if tt.mutateConfig != nil {
				tt.mutateConfig(process.Config)
			}

			params, err := process.makeJoinParams(state.IdentityID{}, []string{}, []string{})
			tt.assertError(t, err)
			tt.assertJoinParams(t, params)
		})
	}
}
