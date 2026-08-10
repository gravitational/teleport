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
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authtest"
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

// TestTeleportProcessJoinVersionCheck covers version enforcement during a
// fresh instance's join.
func TestTeleportProcessJoinVersionCheck(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	// The skew is induced from the server side: the stub advertises requirements
	// the running build can't satisfy, so the client-side check trips against the
	// real teleport.Version without overriding the local version.
	tooOldMinVersion := semver.Version{Major: teleport.SemVer().Major + 1}.String()
	tooNewServerVersion := semver.Version{Major: teleport.SemVer().Major - 1}.String()

	newProcess := func(t *testing.T, findHandler http.HandlerFunc, skipVersionCheck bool) *TeleportProcess {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The skipped-check variant proceeds past the version check and hits
			// join endpoints this stub doesn't implement before failing.
			if r.URL.Path != "/webapi/find" {
				http.NotFound(w, r)
				return
			}
			findHandler(w, r)
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
		cfg.SkipVersionCheck = skipVersionCheck

		process, err := NewTeleport(cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = process.Close() })
		return process
	}

	serveVersions := func(minClientVersion, serverVersion string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
				MinClientVersion: minClientVersion,
				ServerVersion:    serverVersion,
			}))
		}
	}

	const (
		skipVersionCheckTrue  = true
		skipVersionCheckFalse = false
	)

	t.Run("client too new stops reconnect retries", func(t *testing.T) {
		process := newProcess(t, serveVersions(teleport.MinClientSemVer().String(), tooNewServerVersion), skipVersionCheckFalse)
		c, err := process.reconnectToAuthService(types.RoleInstance)
		requireClientTooNew(t, err)
		require.Nil(t, c)
	})

	// The cases below call connectToAuthService, not reconnectToAuthService.
	// Once past the version check the join fails against the stub with an
	// ordinary error that reconnectToAuthService would retry forever.

	t.Run("client too new with skip version check bypasses the failure", func(t *testing.T) {
		// A non-version error proves SkipVersionCheck was plumbed through.
		process := newProcess(t, serveVersions(teleport.MinClientSemVer().String(), tooNewServerVersion), skipVersionCheckTrue)
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		var tooNew *clientTooNewError
		require.NotErrorAs(t, err, &tooNew)
		require.Nil(t, c)
	})

	t.Run("client too old is not blocked by the client", func(t *testing.T) {
		// Too-old is the server's call, so the client-side join check must not
		// block it. The join proceeds past the version check and fails against the
		// stub with an ordinary (non-version) error. connectToAuthService is used
		// (single attempt) rather than reconnectToAuthService, which would retry a
		// non-version error forever.
		process := newProcess(t, serveVersions(tooOldMinVersion, teleport.Version), skipVersionCheckFalse)
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		require.False(t, isVersionIncompatible(err), "too-old must not be a client-side version block, got %v", err)
		var tooOld *clientTooOldError
		require.NotErrorAs(t, err, &tooOld)
		require.Nil(t, c)
	})

	t.Run("compatible version passes the check", func(t *testing.T) {
		// Only too-new is enforced, so a non-version error proves the check
		// didn't reject and the flow reached the join.
		process := newProcess(t, serveVersions(teleport.MinClientSemVer().String(), teleport.Version), skipVersionCheckFalse)
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		require.False(t, isVersionIncompatible(err), "compatible version should pass the check, got %v", err)
		require.Nil(t, c)
	})

	t.Run("find failure skips version check", func(t *testing.T) {
		// A failed /webapi/find must fail open, not block the connection.
		process := newProcess(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}, skipVersionCheckFalse)
		c, err := process.connectToAuthService(types.RoleInstance)
		require.Error(t, err)
		require.False(t, isVersionIncompatible(err), "version check should have been skipped on find failure, got %v", err)
		require.Nil(t, c)
	})
}

// fakeAuthPingServer is a minimal AuthService gRPC server that only answers
// Ping, letting tests advertise an arbitrary ServerVersion.
type fakeAuthPingServer struct {
	proto.UnimplementedAuthServiceServer
	serverVersion string
}

func (f *fakeAuthPingServer) Ping(context.Context, *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{ServerVersion: f.serverVersion, ServerFeatures: &proto.Features{}}, nil
}

// TestGetConnectorVersionCheck covers the startup too-new check for an
// already-joined instance reconnecting. The fake gRPC server fakes its
// version while presenting an authtest-issued cert so it passes the
// connector's TLS check.
func TestGetConnectorVersionCheck(t *testing.T) {
	t.Parallel()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	// nodeID is the connecting client. authID backs the fake server's cert: it
	// plays the auth server, and an auth-role cert carries the
	// "*.teleport.cluster.local" SAN the connector's TLS check requires (node
	// certs don't). Both are issued by the same cluster CA the connector trusts.
	nodeID, err := authtest.NewServerIdentity(testAuthServer.AuthServer, "test-host-id", types.RoleNode)
	require.NoError(t, err)
	authID, err := authtest.NewServerIdentity(testAuthServer.AuthServer, "test-auth-id", types.RoleAuth)
	require.NoError(t, err)
	serverTLSConfig, err := authID.TLSConfig(nil)
	require.NoError(t, err)

	// A server one major behind makes this instance too new.
	tooNewServerVersion := semver.Version{Major: teleport.SemVer().Major - 1}.String()

	for _, tt := range []struct {
		name          string
		serverVersion string
		skipCheck     bool
		assertError   require.ErrorAssertionFunc
	}{
		{
			name:          "client too new stops connection",
			serverVersion: tooNewServerVersion,
			assertError:   requireClientTooNew,
		},
		{
			// Skip must not just suppress the error but yield a working connector.
			name:          "client too new with skip version check connects",
			serverVersion: tooNewServerVersion,
			skipCheck:     true,
			assertError:   require.NoError,
		},
		{
			name:          "compatible version connects",
			serverVersion: teleport.Version,
			assertError:   require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = lis.Close() })

			grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLSConfig)))
			proto.RegisterAuthServiceServer(grpcServer, &fakeAuthPingServer{serverVersion: tt.serverVersion})
			go grpcServer.Serve(lis)
			t.Cleanup(grpcServer.Stop)

			// V3 config with an auth address and no proxy routes newClient down
			// the direct-to-auth path, straight to the fake gRPC server.
			cfg := servicecfg.MakeDefaultConfig()
			cfg.Version = defaults.TeleportConfigVersionV3
			cfg.SkipVersionCheck = tt.skipCheck
			cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: lis.Addr().String()})

			process := &TeleportProcess{
				Clock:      clockwork.NewRealClock(),
				Supervisor: &LocalSupervisor{exitContext: t.Context()},
				Config:     cfg,
				logger:     logtest.NewLogger(),
			}
			// getConnector checks for a reusable instance client for non-instance
			// roles. A closed ready channel makes that lookup return immediately
			// instead of blocking, so it falls through to a fresh connection.
			process.instanceConnectorReady = make(chan struct{})
			close(process.instanceConnectorReady)

			conn, err := process.getConnector(nodeID, nodeID)
			if conn != nil {
				t.Cleanup(func() { _ = conn.Close() })
			}
			tt.assertError(t, err)
		})
	}
}

// TestProxyVersionInfo covers the reconnect-path fetch of the proxy's advertised
// version information, including the fail-open behavior that keeps a transient or
// unreachable web API from blocking an otherwise-valid instance.
func TestProxyVersionInfo(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	newProcess := func(t *testing.T, handler http.HandlerFunc) *TeleportProcess {
		srv := httptest.NewTLSServer(handler)
		t.Cleanup(srv.Close)
		return &TeleportProcess{
			Supervisor: &LocalSupervisor{exitContext: t.Context()},
			Config: &servicecfg.Config{
				Version:     defaults.TeleportConfigVersionV3,
				ProxyServer: utils.NetAddr{AddrNetwork: "tcp", Addr: strings.TrimPrefix(srv.URL, "https://")},
			},
			logger: logtest.NewLogger(),
		}
	}

	t.Run("advertised versions are returned", func(t *testing.T) {
		process := newProcess(t, func(w http.ResponseWriter, _ *http.Request) {
			require.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
				ServerVersion:    "1.2.3",
				MinClientVersion: "1.0.0",
			}))
		})
		require.Equal(t,
			joinclient.VersionInfo{ServerVersion: "1.2.3", MinClientVersion: "1.0.0"},
			process.proxyVersionInfo(),
		)
	})

	t.Run("fetch failure fails open", func(t *testing.T) {
		process := newProcess(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		require.Equal(t, joinclient.VersionInfo{}, process.proxyVersionInfo())
	})

	t.Run("no proxy address fails open", func(t *testing.T) {
		process := &TeleportProcess{
			Supervisor: &LocalSupervisor{exitContext: t.Context()},
			Config:     &servicecfg.Config{},
			logger:     logtest.NewLogger(),
		}
		require.Equal(t, joinclient.VersionInfo{}, process.proxyVersionInfo())
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
