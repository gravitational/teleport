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
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

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
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

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
				logger:     utils.NewSlogLoggerForTests(),
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
			logger: utils.NewSlogLoggerForTests(),
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
			versionInfo{ServerVersion: "1.2.3", MinClientVersion: "1.0.0"},
			process.proxyVersionInfo(),
		)
	})

	t.Run("fetch failure fails open", func(t *testing.T) {
		process := newProcess(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		require.Equal(t, versionInfo{}, process.proxyVersionInfo())
	})

	t.Run("no proxy address fails open", func(t *testing.T) {
		process := &TeleportProcess{
			Supervisor: &LocalSupervisor{exitContext: t.Context()},
			Config:     &servicecfg.Config{},
			logger:     utils.NewSlogLoggerForTests(),
		}
		require.Equal(t, versionInfo{}, process.proxyVersionInfo())
	})
}
