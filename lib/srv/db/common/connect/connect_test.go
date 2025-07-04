// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package connect

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGetDatabaseServers(t *testing.T) {
	for name, tc := range map[string]struct {
		identity           tlsca.Identity
		getter             *databaseServersMock
		expectErrorFunc    require.ErrorAssertionFunc
		expectedServersLen int
	}{
		"match": {
			identity:           identityWithDatabase("matched-db", "root", "alice", nil),
			getter:             newDatabaseServersWithServers("no-match", "matched-db", "another-db"),
			expectErrorFunc:    require.NoError,
			expectedServersLen: 1,
		},
		"no match": {
			identity: identityWithDatabase("no-match", "root", "alice", nil),
			getter:   newDatabaseServersWithServers("first", "second", "third"),
			expectErrorFunc: func(tt require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err), "expected trace.NotFound error but got %T", err)
			},
		},
		"get server error": {
			identity:        identityWithDatabase("no-match", "root", "alice", nil),
			getter:          newDatabaseServersWithErr(trace.Errorf("failure")),
			expectErrorFunc: require.Error,
		},
	} {
		t.Run(name, func(t *testing.T) {
			servers, err := GetDatabaseServers(context.Background(), GetDatabaseServersParams{
				Logger:                utils.NewSlogLoggerForTests(),
				ClusterName:           "root",
				DatabaseServersGetter: tc.getter,
				Identity:              tc.identity,
			})
			tc.expectErrorFunc(t, err)
			require.Len(t, servers, tc.expectedServersLen)
		})
	}
}

func TestGetServerTLSConfig(t *testing.T) {
	clusterName := "root"
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: clusterName,
		AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
			SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	user, role, err := auth.CreateUserAndRole(authServer.AuthServer, "alice", []string{"db-access"}, nil)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		server              types.DatabaseServer
		identity            tlsca.Identity
		expectErrorFunc     require.ErrorAssertionFunc
		expectTLSConfigFunc require.ValueAssertionFunc
	}{
		"generates the config": {
			server:          databaseServerWithName("db", "server1"),
			identity:        identityWithDatabase("db", clusterName, user.GetName(), []string{role.GetName()}),
			expectErrorFunc: require.NoError,
			expectTLSConfigFunc: func(tt require.TestingT, tlsConfigI any, _ ...any) {
				require.IsType(t, &tls.Config{}, tlsConfigI)
				tlsConfig, _ := tlsConfigI.(*tls.Config)
				require.Len(t, tlsConfig.Certificates, 1)

				ca, err := tlsca.FromTLSCertificate(tlsConfig.Certificates[0])
				require.NoError(t, err, "failed to extract CA from TLS certificate")

				identity, err := tlsca.FromSubject(ca.Cert.Subject, ca.Cert.NotAfter)
				require.NoError(t, err, "failed to convert certificate subject into tlsca.Identity")
				require.Equal(t, clusterName, identity.TeleportCluster)
				require.ElementsMatch(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
			},
		},
		"failed to generate config due to missing information on identity": {
			server:              databaseServerWithName("db", "server1"),
			identity:            tlsca.Identity{},
			expectErrorFunc:     require.Error,
			expectTLSConfigFunc: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tlsConfig, err := GetServerTLSConfig(context.Background(), ServerTLSConfigParams{
				CertSigner:     authServer.AuthServer,
				AuthPreference: authServer.AuthServer,
				Server:         tc.server,
				Identity:       tc.identity,
			})
			tc.expectErrorFunc(t, err)
			tc.expectTLSConfigFunc(t, tlsConfig)
		})
	}
}

func TestConnect(t *testing.T) {
	clusterName := "root"
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: clusterName,
		AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
			SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	user, role, err := auth.CreateUserAndRole(authServer.AuthServer, "alice", []string{"db-access"}, nil)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		identity      tlsca.Identity
		dialer        *dialerMock
		expectErrFunc require.ErrorAssertionFunc
		expectedStats ConnectStats
	}{
		"connects": {
			identity:      identityWithDatabase("db", clusterName, user.GetName(), []string{role.GetName()}),
			dialer:        newDialerMock(t, authServer.AuthServer, "db", []string{"server-1", "server-2"}, nil),
			expectErrFunc: require.NoError,
			expectedStats: Stats{attemptedServers: 1, dialAttempts: 1},
		},
		"connects but with dial failures": {
			identity: identityWithDatabase("db", clusterName, user.GetName(), []string{role.GetName()}),
			// Given the shuffle function, the "server-1" will be attempted first (cause the initial failure).
			dialer:        newDialerMock(t, authServer.AuthServer, "db", []string{"server-2"}, []string{"server-1"}),
			expectErrFunc: require.NoError,
			expectedStats: Stats{attemptedServers: 2, dialAttempts: 2, dialFailures: 1},
		},
		"fails to connect": {
			identity:      identityWithDatabase("db", clusterName, user.GetName(), []string{role.GetName()}),
			dialer:        newDialerMock(t, authServer.AuthServer, "db", nil, []string{"server-1"}),
			expectErrFunc: require.Error,
			expectedStats: Stats{attemptedServers: 1, dialAttempts: 1, dialFailures: 1},
		},
		"no servers": {
			identity:      identityWithDatabase("db", clusterName, user.GetName(), []string{role.GetName()}),
			dialer:        newDialerMock(t, authServer.AuthServer, "db", nil, nil),
			expectErrFunc: require.Error,
			expectedStats: Stats{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			conn, stats, err := Connect(context.Background(), ConnectParams{
				Logger:         utils.NewSlogLoggerForTests(),
				Identity:       tc.identity,
				Servers:        tc.dialer.getServers(),
				ShuffleFunc:    ShuffleSort,
				ClusterName:    clusterName,
				Dialer:         tc.dialer,
				CertSigner:     authServer.AuthServer,
				AuthPreference: authServer.AuthServer,
				ClientSrcAddr:  net.TCPAddrFromAddrPort(netip.MustParseAddrPort("0.0.0.0:3000")),
				ClientDstAddr:  net.TCPAddrFromAddrPort(netip.MustParseAddrPort("0.0.0.0:3000")),
			})
			tc.expectErrFunc(t, err)
			require.Equal(t, tc.expectedStats, stats)
			if conn != nil {
				conn.Close()
			}
		})
	}
}

func identityWithDatabase(name, clusterName, user string, roles []string) tlsca.Identity {
	return tlsca.Identity{
		RouteToCluster:  clusterName,
		TeleportCluster: clusterName,
		Username:        user,
		Groups:          roles,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: name,
			Protocol:    defaults.ProtocolPostgres,
			Username:    "postgres",
			Database:    "postgres",
		},
	}
}

type databaseServersMock struct {
	servers []types.DatabaseServer
	err     error
}

func databaseServerWithName(name, hostId string) types.DatabaseServer {
	return &types.DatabaseServerV3{
		Spec: types.DatabaseServerSpecV3{
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: name,
				},
			},
			HostID:   hostId,
			Hostname: name,
		},
	}
}

func newDatabaseServersWithServers(dbNames ...string) *databaseServersMock {
	var servers []types.DatabaseServer
	for _, name := range dbNames {
		servers = append(servers, databaseServerWithName(name, uuid.New().String()))
	}

	return &databaseServersMock{servers: servers}
}

func newDatabaseServersWithErr(err error) *databaseServersMock {
	return &databaseServersMock{err: err}
}

func (d *databaseServersMock) GetDatabaseServers(_ context.Context, _ string, _ ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return d.servers, d.err
}

func newDialerMock(t *testing.T, authServer *auth.Server, dbName string, availableServers []string, unavailableServers []string) *dialerMock {
	m := &dialerMock{serverConfig: make(map[string]*tls.Config)}
	for _, host := range availableServers {
		serverIdentity, err := auth.NewServerIdentity(authServer, host, types.RoleDatabase)
		require.NoError(t, err)
		tlsConfig, err := serverIdentity.TLSConfig(nil)
		require.NoError(t, err)

		m.serverConfig[host] = tlsConfig
		m.servers = append(m.servers, databaseServerWithName(dbName, host))
	}

	for _, host := range unavailableServers {
		m.servers = append(m.servers, databaseServerWithName(dbName, host))
	}

	return m
}

type dialerMock struct {
	servers      []types.DatabaseServer
	serverConfig map[string]*tls.Config
}

func (m *dialerMock) Dial(params reversetunnelclient.DialParams) (conn net.Conn, err error) {
	hostID, _, _ := strings.Cut(params.ServerID, ".")
	tlsConfig, ok := m.serverConfig[hostID]
	if !ok {
		return nil, trace.ConnectionProblem(nil, reversetunnelclient.NoDatabaseTunnel)
	}

	// Start a fake database server that only performs the TLS handshake.
	clt, srv := net.Pipe()
	go func() {
		defer srv.Close()
		conn := tls.Server(srv, tlsConfig)
		_ = conn.Handshake()
	}()

	return clt, nil
}

func (m *dialerMock) getServers() []types.DatabaseServer {
	return m.servers
}
