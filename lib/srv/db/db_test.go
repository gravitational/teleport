/*
Copyright 2020 Gravitational, Inc.

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

package db

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/client"
	auth "github.com/gravitational/teleport/lib/auth/server"
	libtest "github.com/gravitational/teleport/lib/auth/test"
	testservices "github.com/gravitational/teleport/lib/auth/test/services"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestPostgresAccess verifies access scenarios to a Postgres database based
// on the configured RBAC rules.
func TestPostgresAccess(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	t.Cleanup(func() { testCtx.Close() })

	// Start multiplexer.
	go testCtx.mux.Serve()
	// Start fake Postgres server.
	go testCtx.postgresServer.Serve()
	// Start database proxy server.
	go testCtx.proxyServer.Serve(testCtx.mux.DB())
	// Start database service server.
	go func() {
		for conn := range testCtx.proxyConn {
			testCtx.server.HandleConnection(conn)
		}
	}()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbNames []string
		allowDbUsers []string
		dbName       string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "metrics",
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			_, role, err := libtest.CreateUserAndRole(testCtx.tlsServer.Auth(), test.user, []string{test.role})
			require.NoError(t, err)

			role.SetDatabaseNames(types.Allow, test.allowDbNames)
			role.SetDatabaseUsers(types.Allow, test.allowDbUsers)
			err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// Try to connect to the database as this user.
			pgConn, err := postgres.MakeTestClient(ctx, common.TestClientConfig{
				AuthClient: testCtx.authClient,
				AuthServer: testCtx.authServer,
				Address:    testCtx.mux.DB().Addr().String(),
				Cluster:    testCtx.clusterName,
				Username:   test.user,
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: "postgres-test",
					Protocol:    defaults.ProtocolPostgres,
					Username:    test.dbUser,
					Database:    test.dbName,
				},
			})
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}

			require.NoError(t, err)

			// Execute a query.
			result, err := pgConn.Exec(ctx, "select 1").ReadAll()
			require.NoError(t, err)
			require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)

			// Disconnect.
			err = pgConn.Close(ctx)
			require.NoError(t, err)
		})
	}
}

// TestMySQLAccess verifies access scenarios to a MySQL database based
// on the configured RBAC rules.
func TestMySQLAccess(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	t.Cleanup(func() { testCtx.Close() })

	// Start test MySQL server.
	go testCtx.mysqlServer.Serve()
	// Start MySQL proxy server.
	go testCtx.proxyServer.ServeMySQL(testCtx.mysqlListener)
	// Start database service server.
	go func() {
		for conn := range testCtx.proxyConn {
			testCtx.server.HandleConnection(conn)
		}
	}()

	tests := []struct {
		// desc is the test case description.
		desc string
		// user is the Teleport local user name the test will use.
		user string
		// role is the Teleport role name to create and assign to the user.
		role string
		// allowDbUsers is the role's list of allowed database users.
		allowDbUsers []string
		// dbUser is the database user to simulate connect as.
		dbUser string
		// err is the expected test case error.
		err string
	}{
		{
			desc:         "has access to all database users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to database denied",
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "root",
			err:          "access to database denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			_, role, err := libtest.CreateUserAndRole(testCtx.tlsServer.Auth(), test.user, []string{test.role})
			require.NoError(t, err)

			role.SetDatabaseUsers(types.Allow, test.allowDbUsers)
			err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// Try to connect to the database as this user.
			mysqlConn, err := mysql.MakeTestClient(common.TestClientConfig{
				AuthClient: testCtx.authClient,
				AuthServer: testCtx.authServer,
				Address:    testCtx.mysqlListener.Addr().String(),
				Cluster:    testCtx.clusterName,
				Username:   test.user,
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: "mysql-test",
					Protocol:    defaults.ProtocolPostgres,
					Username:    test.dbUser,
				},
			})
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}

			require.NoError(t, err)

			// Execute a query.
			result, err := mysqlConn.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			// Disconnect.
			err = mysqlConn.Close()
			require.NoError(t, err)
		})
	}
}

type testModules struct {
	modules.Modules
}

func (m *testModules) Features() modules.Features {
	return modules.Features{
		DB: false, // Explicily turn off database access.
	}
}

// TestDatabaseAccessDisabled makes sure database access can be disabled via
// modules.
func TestDatabaseAccessDisabled(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testModules{})

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	t.Cleanup(func() { testCtx.Close() })

	// Start multiplexer.
	go testCtx.mux.Serve()
	// Start fake Postgres server.
	go testCtx.postgresServer.Serve()
	// Start database proxy server.
	go testCtx.proxyServer.Serve(testCtx.mux.DB())
	// Start database service server.
	go func() {
		for conn := range testCtx.proxyConn {
			testCtx.server.HandleConnection(conn)
		}
	}()

	userName := "alice"
	roleName := "admin"
	dbUser := "postgres"
	dbName := "postgres"

	// Create user/role with the requested permissions.
	_, role, err := libtest.CreateUserAndRole(testCtx.tlsServer.Auth(), userName, []string{roleName})
	require.NoError(t, err)

	role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Try to connect to the database as this user.
	_, err = postgres.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient: testCtx.authClient,
		AuthServer: testCtx.authServer,
		Address:    testCtx.mux.DB().Addr().String(),
		Cluster:    testCtx.clusterName,
		Username:   userName,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "postgres-test",
			Protocol:    defaults.ProtocolPostgres,
			Username:    dbUser,
			Database:    dbName,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster doesn't support database access")
}

type testContext struct {
	clusterName      string
	tlsServer        *testservices.TLSServer
	authServer       *auth.Server
	authClient       *client.Client
	postgresServer   *postgres.TestServer
	mysqlServer      *mysql.TestServer
	proxyServer      *ProxyServer
	mux              *multiplexer.Mux
	mysqlListener    net.Listener
	proxyConn        chan (net.Conn)
	server           *Server
	postgresDBServer types.DatabaseServer
	mysqlDBServer    types.DatabaseServer
}

// Close closes all resources associated with the test context.
func (c *testContext) Close() error {
	if c.mux != nil {
		c.mux.Close()
	}
	if c.mysqlListener != nil {
		c.mysqlListener.Close()
	}
	if c.postgresServer != nil {
		c.postgresServer.Close()
	}
	if c.mysqlServer != nil {
		c.mysqlServer.Close()
	}
	if c.server != nil {
		c.server.Close()
	}
	return nil
}

func setupTestContext(ctx context.Context, t *testing.T) *testContext {
	clusterName := "root.example.com"
	postgresServerName := "postgres-test"
	mysqlServerName := "mysql-test"
	hostID := uuid.New()

	// Create multiplexer.
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	mux, err := multiplexer.New(multiplexer.Config{ID: "test", Listener: listener})
	require.NoError(t, err)

	// Create MySQL proxy listener.
	mysqlListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	// Create and start test auth server.
	authServer, err := testservices.NewAuthServer(testservices.AuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	tlsServer, err := authServer.NewTLSServer()
	require.NoError(t, err)

	// Use sync recording to not involve the uploader.
	clusterConfig, err := authServer.AuthServer.GetClusterConfig()
	require.NoError(t, err)
	clusterConfig.SetSessionRecording(types.RecordAtNodeSync)
	err = authServer.AuthServer.SetClusterConfig(clusterConfig)
	require.NoError(t, err)

	// Auth client/authorizer for database service.
	dbAuthClient, err := tlsServer.NewClient(testservices.ServerID(teleport.RoleDatabase, hostID))
	require.NoError(t, err)
	dbAuthorizer, err := auth.NewAuthorizer(clusterName, dbAuthClient, dbAuthClient, dbAuthClient)
	require.NoError(t, err)

	// Auth client/authorizer for database proxy.
	proxyAuthClient, err := tlsServer.NewClient(testservices.Builtin(teleport.RoleProxy))
	require.NoError(t, err)
	proxyAuthorizer, err := auth.NewAuthorizer(clusterName, proxyAuthClient, proxyAuthClient, proxyAuthClient)
	require.NoError(t, err)

	// TLS config for database proxy and database service.
	serverIdentity, err := testservices.NewServerIdentity(authServer.AuthServer, hostID, teleport.RoleDatabase)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Fake Postgres server that speaks part of its wire protocol.
	postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: dbAuthClient,
		Name:       postgresServerName,
	})
	require.NoError(t, err)

	// Test MySQL server.
	mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
		AuthClient: dbAuthClient,
		Name:       mysqlServerName,
	})
	require.NoError(t, err)

	// Create database servers for the test database services.
	postgresDBServer := makeDatabaseServer(postgresServerName, net.JoinHostPort("localhost", postgresServer.Port()), defaults.ProtocolPostgres, hostID)
	_, err = dbAuthClient.UpsertDatabaseServer(ctx, postgresDBServer)
	require.NoError(t, err)

	mysqlDBServer := makeDatabaseServer(mysqlServerName, net.JoinHostPort("localhost", mysqlServer.Port()), defaults.ProtocolMySQL, hostID)
	_, err = dbAuthClient.UpsertDatabaseServer(ctx, mysqlDBServer)
	require.NoError(t, err)

	// Establish fake reversetunnel b/w database proxy and database service.
	connCh := make(chan net.Conn)
	tunnel := &reversetunnel.FakeServer{
		Sites: []reversetunnel.RemoteSite{
			&reversetunnel.FakeRemoteSite{
				Name:        clusterName,
				ConnCh:      connCh,
				AccessPoint: proxyAuthClient,
			},
		},
	}

	// Create database proxy server.
	proxyServer, err := NewProxyServer(ctx, ProxyServerConfig{
		AuthClient:  proxyAuthClient,
		AccessPoint: proxyAuthClient,
		Authorizer:  proxyAuthorizer,
		Tunnel:      tunnel,
		TLSConfig:   tlsConfig,
	})
	require.NoError(t, err)

	// Create database service server.
	server, err := New(ctx, Config{
		Clock:         clockwork.NewFakeClockAt(time.Now()),
		DataDir:       t.TempDir(),
		AuthClient:    dbAuthClient,
		AccessPoint:   dbAuthClient,
		StreamEmitter: dbAuthClient,
		Authorizer:    dbAuthorizer,
		Servers:       []types.DatabaseServer{postgresDBServer, mysqlDBServer},
		TLSConfig:     tlsConfig,
		GetRotation:   func(teleport.Role) (*types.Rotation, error) { return &types.Rotation{}, nil },
	})
	require.NoError(t, err)

	return &testContext{
		clusterName:      clusterName,
		mux:              mux,
		mysqlListener:    mysqlListener,
		proxyServer:      proxyServer,
		proxyConn:        connCh,
		postgresServer:   postgresServer,
		mysqlServer:      mysqlServer,
		server:           server,
		postgresDBServer: postgresDBServer,
		mysqlDBServer:    mysqlDBServer,
		tlsServer:        tlsServer,
		authServer:       tlsServer.Auth(),
		authClient:       dbAuthClient,
	}
}

func makeDatabaseServer(name, uri, protocol, hostID string) types.DatabaseServer {
	return types.NewDatabaseServerV3(
		name,
		nil,
		types.DatabaseServerSpecV3{
			Protocol: protocol,
			URI:      uri,
			Version:  teleport.Version,
			Hostname: teleport.APIDomain,
			HostID:   hostID,
			DynamicLabels: types.LabelsToV2(map[string]types.CommandLabel{
				"echo": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Second),
					Command: []string{"echo", "test"},
				},
			}),
		})
}
