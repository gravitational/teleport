/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/service"
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
		t.Skip("Skipping AWS Databases test suite.")
	}
	// when adding a new type of AWS db e2e test, you should add to this
	// unmatched discovery test and add a test for matched discovery/connection
	// as well below.
	t.Run("unmatched discovery", awsDBDiscoveryUnmatched)
	t.Run("rds", testRDS)
	t.Run("redshift serverless", testRedshiftServerless)
}

func awsDBDiscoveryUnmatched(t *testing.T) {
	t.Parallel()
	// get test settings
	awsRegion := mustGetEnv(t, awsRegionEnv)

	// setup discovery matchers
	var matchers []types.AWSMatcher
	for matcherType, assumeRoleARN := range map[string]string{
		// add a new matcher/role here to test that discovery properly
		// does *not* that kind of database for some unmatched tag.
		types.AWSMatcherRDS:                mustGetEnv(t, rdsDiscoveryRoleEnv),
		types.AWSMatcherRedshiftServerless: mustGetEnv(t, rssDiscoveryRoleEnv),
	} {
		matchers = append(matchers, types.AWSMatcher{
			Types: []string{matcherType},
			Tags: types.Labels{
				// This label should not match.
				"env": {"tag_not_found"},
			},
			Regions: []string{awsRegion},
			AssumeRole: &types.AssumeRole{
				RoleARN: assumeRoleARN,
			},
		})
	}

	cluster := createTeleportCluster(t,
		withSingleProxyPort(t),
		withDiscoveryService(t, "db-e2e-test", matchers...),
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

const (
	waitForConnTimeout = 60 * time.Second
	connRetryTick      = 10 * time.Second
)

// postgresConnTestFn tests connection to a postgres database via proxy web
// multiplexer.
func postgresConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	var pgConn *pgconn.PgConn
	// retry for a while, the database service might need time to give
	// itself IAM rds:connect permissions.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), connRetryTick)
		defer cancel()
		pgConn, err = postgres.MakeTestClient(ctx, common.TestClientConfig{
			AuthClient:      cluster.GetSiteAPI(cluster.Secrets.SiteName),
			AuthServer:      cluster.Process.GetAuthServer(),
			Address:         cluster.Web,
			Cluster:         cluster.Secrets.SiteName,
			Username:        user,
			RouteToDatabase: route,
		})
		assert.NoError(t, err)
		assert.NotNil(t, pgConn)
	}, waitForConnTimeout, connRetryTick, "connecting to postgres")

	// dont wait forever on the exec or close.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute a query.
	results, err := pgConn.Exec(ctx, query).ReadAll()
	require.NoError(t, err)
	for i, r := range results {
		require.NoError(t, r.Err, "error in result %v", i)
	}

	// Disconnect.
	err = pgConn.Close(ctx)
	require.NoError(t, err)
}

// postgresLocalProxyConnTest tests connection to a postgres database via
// local proxy tunnel.
func postgresLocalProxyConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*waitForConnTimeout)
	defer cancel()
	lp := startLocalALPNProxy(t, ctx, user, cluster, route)

	connString := fmt.Sprintf("postgres://%s@%v/%s",
		route.Username, lp.GetAddr(), route.Database)
	var pgConn *pgconn.PgConn
	// retry for a while, the database service might need time to give
	// itself IAM rds:connect permissions.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), connRetryTick)
		defer cancel()
		pgConn, err = pgconn.Connect(ctx, connString)
		assert.NoError(t, err)
		assert.NotNil(t, pgConn)
	}, waitForConnTimeout, connRetryTick, "connecting to postgres")

	// dont wait forever on the exec or close.
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute a query.
	results, err := pgConn.Exec(ctx, query).ReadAll()
	require.NoError(t, err)
	for i, r := range results {
		require.NoError(t, r.Err, "error in result %v", i)
	}

	// Disconnect.
	err = pgConn.Close(ctx)
	require.NoError(t, err)
}

// mysqlLocalProxyConnTest tests connection to a MySQL database via
// local proxy tunnel.
func mysqlLocalProxyConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*waitForConnTimeout)
	defer cancel()

	lp := startLocalALPNProxy(t, ctx, user, cluster, route)

	var conn *mysqlclient.Conn
	// retry for a while, the database service might need time to give
	// itself IAM rds:connect permissions.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		var nd net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), connRetryTick)
		defer cancel()
		conn, err = mysqlclient.ConnectWithDialer(ctx, "tcp",
			lp.GetAddr(),
			route.Username,
			"", /*no password*/
			route.Database,
			nd.DialContext,
		)
		assert.NoError(t, err)
		assert.NotNil(t, conn)
	}, waitForConnTimeout, connRetryTick, "connecting to mysql")

	// Execute a query.
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	_, err := conn.Execute(query)
	require.NoError(t, err)

	// Disconnect.
	require.NoError(t, conn.Close())
}

// startLocalALPNProxy starts local ALPN proxy for the specified database.
func startLocalALPNProxy(t *testing.T, ctx context.Context, user string, cluster *helpers.TeleInstance, route tlsca.RouteToDatabase) *alpnproxy.LocalProxy {
	t.Helper()
	proto, err := alpncommon.ToALPNProtocol(route.Protocol)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxyNetAddr, err := cluster.Process.ProxyWebAddr()
	require.NoError(t, err)

	authSrv := cluster.Process.GetAuthServer()
	tlsCert := generateClientDBCert(t, authSrv, user, route)

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
	t.Cleanup(func() {
		_ = proxy.Close()
	})

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
	t.Helper()
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
	}, 3*time.Minute, 3*time.Second, "waiting for the discovery service to create db resources")
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
