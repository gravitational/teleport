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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestTeleportProcessClientVersionCheck(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	// The skew is induced from the server side: the stub advertises requirements
	// the running build can't satisfy, so the client-side check trips against the
	// real teleport.Version without overriding the local version.
	tooOldMinVersion := semver.Version{Major: teleport.SemVer().Major + 1}.String()
	tooNewServerVersion := semver.Version{Major: teleport.SemVer().Major - 1}.String()

	newConfig := func(t *testing.T, minClientVersion, serverVersion string) *servicecfg.Config {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The skipped-check variant proceeds past the version check and hits
			// join endpoints this stub doesn't implement before failing.
			if r.URL.Path != "/webapi/find" {
				http.NotFound(w, r)
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
				MinClientVersion: minClientVersion,
				ServerVersion:    serverVersion,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := servicecfg.MakeDefaultConfig()
		cfg.Version = defaults.TeleportConfigVersionV3
		cfg.ProxyServer = utils.NetAddr{AddrNetwork: "tcp", Addr: strings.TrimPrefix(srv.URL, "https://")}
		cfg.DataDir = makeTempDir(t)
		cfg.SetToken("join-token")
		cfg.Auth.Enabled = false
		cfg.Proxy.Enabled = false
		cfg.SSH.Enabled = true
		return cfg
	}

	t.Run("client too old stops reconnect retries", func(t *testing.T) {
		cfg := newConfig(t, tooOldMinVersion, teleport.Version)
		process, err := NewTeleport(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = process.Close() })

		c, err := process.reconnectToAuthService(types.RoleInstance)
		var tooOld *joinclient.ClientTooOldError
		require.ErrorAs(t, err, &tooOld)
		require.Nil(t, c)
	})

	t.Run("client too old with skip version check bypasses the failure", func(t *testing.T) {
		cfg := newConfig(t, tooOldMinVersion, teleport.Version)
		cfg.SkipVersionCheck = true

		process, err := NewTeleport(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = process.Close() })

		// Deliberately call connectToAuthService, not reconnectToAuthService.
		// With the check skipped the join fails against the stub with an
		// ordinary connection error, which reconnectToAuthService treats as
		// retryable and would loop on forever. Failing with anything other
		// than ClientTooOldError proves SkipVersionCheck was plumbed through
		// and bypassed the check.
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		var tooOld *joinclient.ClientTooOldError
		require.NotErrorAs(t, err, &tooOld)
		require.Nil(t, c)
	})

	t.Run("client too new stops reconnect retries", func(t *testing.T) {
		cfg := newConfig(t, teleport.MinClientSemVer().String(), tooNewServerVersion)

		process, err := NewTeleport(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = process.Close() })

		c, err := process.reconnectToAuthService(types.RoleInstance)
		var tooNew *joinclient.ClientTooNewError
		require.ErrorAs(t, err, &tooNew)
		require.Nil(t, c)
	})

	t.Run("client too new with skip version check bypasses the failure", func(t *testing.T) {
		cfg := newConfig(t, teleport.MinClientSemVer().String(), tooNewServerVersion)
		cfg.SkipVersionCheck = true

		process, err := NewTeleport(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = process.Close() })

		// Deliberately call connectToAuthService, not reconnectToAuthService.
		// With the check skipped the join fails against the stub with an
		// ordinary connection error, which reconnectToAuthService treats as
		// retryable and would loop on forever. Failing with anything other
		// than ClientTooNewError proves SkipVersionCheck was plumbed through
		// and bypassed the check.
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		var tooNew *joinclient.ClientTooNewError
		require.NotErrorAs(t, err, &tooNew)
		require.Nil(t, c)
	})
}

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
