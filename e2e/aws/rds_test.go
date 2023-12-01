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

package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestDatabases(t *testing.T) {
	t.Parallel()
	testEnabled := os.Getenv(teleport.AWSRunDBTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping Databases test suite.")
	}
	t.Run("unmatched discovery", awsDBDiscoveryUnmatched)
	t.Run("matched discovery", awsDBDiscoveryMatched)
}

func awsDBDiscoveryUnmatched(t *testing.T) {
	t.Parallel()
	// get test settings
	awsRegion := mustGetEnv(t, awsRegionEnv)
	dbDiscoverySvcRoleARN := mustGetEnv(t, dbDiscoverySvcRoleARNEnv)
	dbSvcRoleARN := mustGetEnv(t, dbSvcRoleARNEnv)
	cluster := createTeleportCluster(t,
		withSingleProxyPort(t),
		withDiscoveryService(t, "db-e2e-test", types.AWSMatcher{
			Types: []string{types.AWSMatcherRDS},
			Tags: types.Labels{
				// This label should not match.
				"env": {"tag_not_found"},
			},
			Regions: []string{awsRegion},
			AssumeRole: &types.AssumeRole{
				RoleARN: dbDiscoverySvcRoleARN,
			},
		}),
		withDatabaseService(t, services.ResourceMatcher{
			Labels: types.Labels{types.Wildcard: {types.Wildcard}},
			AWS: services.ResourceMatcherAWS{
				AssumeRoleARN: dbSvcRoleARN,
			},
		}),
		withFullDatabaseAccessUserRole(t),
	)

	// Get the auth server.
	authC := cluster.Process.GetAuthServer()
	// Wait for the discovery service to not create a database resource
	// because the database does not match the selectors.
	require.Never(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		databases, err := authC.GetDatabases(ctx)
		return err == nil && len(databases) != 0
	}, 2*time.Minute, 10*time.Second, "discovery service incorrectly created a database")
}

func awsDBDiscoveryMatched(t *testing.T) {
	t.Parallel()
	// get test settings
	awsRegion := mustGetEnv(t, awsRegionEnv)
	dbDiscoverySvcRoleARN := mustGetEnv(t, dbDiscoverySvcRoleARNEnv)
	dbSvcRoleARN := mustGetEnv(t, dbSvcRoleARNEnv)
	dbUser := mustGetEnv(t, dbUserEnv)
	rdsPostgresInstanceName := mustGetEnv(t, rdsPostgresInstanceNameEnv)
	rdsMySQLInstanceName := mustGetEnv(t, rdsMySQLInstanceNameEnv)

	cluster := createTeleportCluster(t,
		withSingleProxyPort(t),
		withDiscoveryService(t, "db-e2e-test", types.AWSMatcher{
			Types:   []string{types.AWSMatcherRDS},
			Tags:    mustGetDiscoveryMatcherLabels(t),
			Regions: []string{awsRegion},
			AssumeRole: &types.AssumeRole{
				RoleARN: dbDiscoverySvcRoleARN,
			},
		}),
		withDatabaseService(t, services.ResourceMatcher{
			Labels: types.Labels{types.Wildcard: {types.Wildcard}},
			AWS: services.ResourceMatcherAWS{
				AssumeRoleARN: dbSvcRoleARN,
			},
		}),
		withFullDatabaseAccessUserRole(t),
	)

	wantDBNames := []string{
		rdsPostgresInstanceName,
		rdsMySQLInstanceName,
	}
	// wait for the databases to be discovered
	waitForDatabases(t, cluster.Process, wantDBNames...)
	// wait for the database heartbeats from database service
	waitForDatabaseServers(t, cluster.Process, wantDBNames...)

	rdsPostgresInstance := tlsca.RouteToDatabase{
		ServiceName: rdsPostgresInstanceName,
		Protocol:    defaults.ProtocolPostgres,
		Username:    dbUser,
		Database:    "postgres",
	}
	rdsMySQLInstance := tlsca.RouteToDatabase{
		ServiceName: rdsMySQLInstanceName,
		Protocol:    defaults.ProtocolMySQL,
		Username:    dbUser,
		Database:    "", // not needed
	}
	t.Run("connection", func(t *testing.T) {
		tests := []struct {
			name             string
			route            tlsca.RouteToDatabase
			testDBConnection dbConnectionTestFunc
		}{
			{
				name:             "RDS postgres instance",
				route:            rdsPostgresInstance,
				testDBConnection: postgresConnTestFn(cluster),
			},
			{
				name:             "RDS postgres instance via local proxy",
				route:            rdsPostgresInstance,
				testDBConnection: postgresLocalProxyConnTestFn(cluster),
			},
			{
				name:             "RDS MySQL instance via local proxy",
				route:            rdsMySQLInstance,
				testDBConnection: mySQLLocalProxyConnTestFn(cluster),
			},
		}
		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				test.testDBConnection(t, ctx, test.route)
			})
		}
	})
}

type dbConnectionTestFunc func(*testing.T, context.Context, tlsca.RouteToDatabase)

// postgresConnTestFn tests connection to a postgres database via proxy web
// multiplexer.
func postgresConnTestFn(cluster *helpers.TeleInstance) dbConnectionTestFunc {
	return func(t *testing.T, ctx context.Context, route tlsca.RouteToDatabase) {
		var pgConn *pgconn.PgConn
		// retry for a while, the database service might need time to give
		// itself IAM rds:connect permissions.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			var err error
			pgConn, err = postgres.MakeTestClient(ctx, common.TestClientConfig{
				AuthClient:      cluster.GetSiteAPI(cluster.Secrets.SiteName),
				AuthServer:      cluster.Process.GetAuthServer(),
				Address:         cluster.Web,
				Cluster:         cluster.Secrets.SiteName,
				Username:        username,
				RouteToDatabase: route,
			})
			assert.NoError(t, err)
			assert.NotNil(t, pgConn)
		}, time.Second*10, time.Second, "connecting to postgres")

		// Execute a query.
		results, err := pgConn.Exec(ctx, "select 1").ReadAll()
		require.NoError(t, err)
		for i, r := range results {
			require.NoError(t, r.Err, "error in result %v", i)
		}

		// Disconnect.
		err = pgConn.Close(ctx)
		require.NoError(t, err)
	}
}

// postgresLocalProxyConnTestFn tests connection to a postgres database via
// local proxy tunnel.
func postgresLocalProxyConnTestFn(cluster *helpers.TeleInstance) dbConnectionTestFunc {
	return func(t *testing.T, ctx context.Context, route tlsca.RouteToDatabase) {
		lp := startLocalALPNProxy(t, ctx, cluster, route)
		defer lp.Close()

		connString := fmt.Sprintf("postgres://%s@%v/%s",
			route.Username, lp.GetAddr(), route.Database)
		var pgConn *pgconn.PgConn
		// retry for a while, the database service might need time to give
		// itself IAM rds:connect permissions.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			var err error
			pgConn, err = pgconn.Connect(ctx, connString)
			assert.NoError(t, err)
			assert.NotNil(t, pgConn)
		}, time.Second*10, time.Second, "connecting to postgres")

		// Execute a query.
		results, err := pgConn.Exec(ctx, "select 1").ReadAll()
		require.NoError(t, err)
		for i, r := range results {
			require.NoError(t, r.Err, "error in result %v", i)
		}

		// Disconnect.
		err = pgConn.Close(ctx)
		require.NoError(t, err)
	}
}

// mySQLLocalProxyConnTestFn tests connection to a MySQL database via
// local proxy tunnel.
func mySQLLocalProxyConnTestFn(cluster *helpers.TeleInstance) dbConnectionTestFunc {
	return func(t *testing.T, ctx context.Context, route tlsca.RouteToDatabase) {
		lp := startLocalALPNProxy(t, ctx, cluster, route)
		defer lp.Close()

		var conn *mysqlclient.Conn
		// retry for a while, the database service might need time to give
		// itself IAM rds:connect permissions.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			var err error
			conn, err = mysqlclient.Connect(lp.GetAddr(), route.Username, "" /*no password*/, route.Database)
			assert.NoError(t, err)
			assert.NotNil(t, conn)
		}, time.Second*10, time.Second, "connecting to mysql")

		// Execute a query.
		_, err := conn.Execute("select 1")
		require.NoError(t, err)

		// Disconnect.
		require.NoError(t, conn.Close())
	}
}

// startLocalALPNProxy starts local ALPN proxy for the specified database.
func startLocalALPNProxy(t *testing.T, ctx context.Context, cluster *helpers.TeleInstance, route tlsca.RouteToDatabase) *alpnproxy.LocalProxy {
	t.Helper()
	proto, err := alpncommon.ToALPNProtocol(route.Protocol)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxyNetAddr, err := cluster.Process.ProxyWebAddr()
	require.NoError(t, err)

	authSrv := cluster.Process.GetAuthServer()
	tlsCert := generateClientDBCert(t, authSrv, username, route)

	proxy, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    proxyNetAddr.String(),
		Protocols:          []alpncommon.Protocol{proto},
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      ctx,
		Certs:              []tls.Certificate{tlsCert},
	})
	require.NoError(t, err)

	go proxy.Start(ctx)

	return proxy
}

// generateClientDBCert creates a test db cert for the given user and database.
func generateClientDBCert(t *testing.T, authSrv *auth.Server, user string, route tlsca.RouteToDatabase) tls.Certificate {
	t.Helper()
	key, err := client.GenerateRSAKey()
	require.NoError(t, err)

	clusterName, err := authSrv.GetClusterName()
	require.NoError(t, err)

	clientCert, err := authSrv.GenerateDatabaseTestCert(
		auth.DatabaseTestCertRequest{
			PublicKey:       key.MarshalSSHPublicKey(),
			Cluster:         clusterName.GetClusterName(),
			Username:        user,
			RouteToDatabase: route,
		})
	require.NoError(t, err)

	tlsCert, err := key.TLSCertificate(clientCert)
	require.NoError(t, err)
	return tlsCert
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, wantNames ...string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		databases, err := auth.GetAuthServer().GetDatabases(ctx)
		assert.NoError(t, err)

		// map the registered "db" resource names.
		seen := map[string]struct{}{}
		for _, db := range databases {
			seen[db.GetName()] = struct{}{}
		}
		for _, name := range wantNames {
			assert.Contains(t, seen, name)
		}
	}, 3*time.Minute, 3*time.Second, "waiting for the discovery service to create databases")
}

func waitForDatabaseServers(t *testing.T, auth *service.TeleportProcess, wantNames ...string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		servers, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
		assert.NoError(t, err)

		// map the registered "db_server" resource names.
		seen := map[string]struct{}{}
		for _, s := range servers {
			seen[s.GetName()] = struct{}{}
		}
		for _, name := range wantNames {
			assert.Contains(t, seen, name)
		}
	}, 1*time.Minute, time.Second, "waiting for the database service to heartbeat the databases")
}
