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

package db

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db"
	"github.com/gravitational/teleport/lib/srv/db/cassandra"
	"github.com/gravitational/teleport/lib/srv/db/common"
	dbconnect "github.com/gravitational/teleport/lib/srv/db/common/connect"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestDatabaseAccess runs the database access integration test suite.
//
// It allows to make the entire cluster set up once, instead of per test,
// which speeds things up significantly.
func TestDatabaseAccess(t *testing.T) {
	pack := SetupDatabaseTest(t,
		// set tighter rotation intervals
		WithLeafConfig(func(config *servicecfg.Config) {
			config.PollingPeriod = 5 * time.Second
			config.RotationConnectionInterval = 2 * time.Second
		}),
		WithRootConfig(func(config *servicecfg.Config) {
			config.PollingPeriod = 5 * time.Second
			config.RotationConnectionInterval = 2 * time.Second
			config.Proxy.MySQLServerVersion = "8.0.1"
		}),
	)
	pack.WaitForLeaf(t)

	t.Run("PostgresRootCluster", pack.testPostgresRootCluster)
	t.Run("PostgresLeafCluster", pack.testPostgresLeafCluster)
	t.Run("MySQLRootCluster", pack.testMySQLRootCluster)
	t.Run("MySQLLeafCluster", pack.testMySQLLeafCluster)
	t.Run("MongoRootCluster", pack.testMongoRootCluster)
	t.Run("MongoLeafCluster", pack.testMongoLeafCluster)
	t.Run("MongoConnectionCount", pack.testMongoConnectionCount)
	t.Run("HARootCluster", pack.testHARootCluster)
	t.Run("HALeafCluster", pack.testHALeafCluster)
	t.Run("LargeQuery", pack.testLargeQuery)
	t.Run("AgentState", pack.testAgentState)
	t.Run("CassandraRootCluster", pack.testCassandraRootCluster)
	t.Run("CassandraLeafCluster", pack.testCassandraLeafCluster)

	t.Run("IPPinning", pack.testIPPinning)
}

// TestDatabaseAccessSeparateListeners tests the Mongo and Postgres separate port setup.
func TestDatabaseAccessSeparateListeners(t *testing.T) {
	pack := SetupDatabaseTest(t,
		WithListenerSetupDatabaseTest(helpers.SeparateMongoAndPostgresPortSetup),
	)

	t.Run("PostgresSeparateListener", pack.testPostgresSeparateListener)
	t.Run("MongoSeparateListener", pack.testMongoSeparateListener)
}

// testIPPinning tests a scenario where a user with IP pinning
// connects to a database
func (p *DatabasePack) testIPPinning(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DB: {Enabled: true},
			},
		},
	})

	type testCase struct {
		desc          string
		targetCluster databaseClusterPack
		pinnedIP      string
		wantClientErr string
	}

	testCases := []testCase{
		{
			desc:          "root cluster, no pinned ip",
			targetCluster: p.Root,
		},
		{
			desc:          "root cluster, correct pinned ip",
			targetCluster: p.Root,
			pinnedIP:      "127.0.0.1",
		},
		{
			desc:          "root cluster, incorrect pinned ip",
			targetCluster: p.Root,
			wantClientErr: "pinned IP doesn't match observed client IP",
			pinnedIP:      "127.0.0.2",
		},
		{
			desc:          "leaf cluster, no pinned ip",
			targetCluster: p.Leaf,
		},
		{
			desc:          "leaf cluster, correct pinned ip",
			targetCluster: p.Leaf,
			pinnedIP:      "127.0.0.1",
		},
		{
			desc:          "leaf cluster, incorrect pinned ip",
			targetCluster: p.Leaf,
			wantClientErr: "pinned IP doesn't match observed client IP",
			pinnedIP:      "127.0.0.2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Connect to the database service in root cluster.
			testClient, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
				AuthServer: p.Root.Cluster.Process.GetAuthServer(),
				Address:    p.Root.Cluster.Web,
				Cluster:    tc.targetCluster.Cluster.Secrets.SiteName,
				Username:   p.Root.User.GetName(),
				PinnedIP:   tc.pinnedIP,
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: tc.targetCluster.PostgresService.Name,
					Protocol:    tc.targetCluster.PostgresService.Protocol,
					Username:    "postgres",
					Database:    "test",
				},
			})
			if tc.wantClientErr != "" {
				require.ErrorContains(t, err, tc.wantClientErr)
				return
			}
			require.NoError(t, err)

			wantQueryCount := tc.targetCluster.postgres.QueryCount() + 1

			// Execute a query.
			result, err := testClient.Exec(context.Background(), "select 1").ReadAll()
			require.NoError(t, err)
			require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
			require.Equal(t, wantQueryCount, tc.targetCluster.postgres.QueryCount())

			// Disconnect.
			err = testClient.Close(context.Background())
			require.NoError(t, err)
		})
	}
}

// testPostgresRootCluster tests a scenario where a user connects
// to a Postgres database running in a root cluster.
func (p *DatabasePack) testPostgresRootCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.PostgresService.Name,
			Protocol:    p.Root.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.postgres.QueryCount() + 1
	wantLeafQueryCount := p.Leaf.postgres.QueryCount()

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, wantRootQueryCount, p.Root.postgres.QueryCount())
	require.Equal(t, wantLeafQueryCount, p.Leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// testPostgresLeafCluster tests a scenario where a user connects
// to a Postgres database running in a leaf cluster via a root cluster.
func (p *DatabasePack) testPostgresLeafCluster(t *testing.T) {
	// Connect to the database service in leaf cluster via root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web, // Connecting via root cluster.
		Cluster:    p.Leaf.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Leaf.PostgresService.Name,
			Protocol:    p.Leaf.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.postgres.QueryCount()
	wantLeafQueryCount := p.Leaf.postgres.QueryCount() + 1

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, wantLeafQueryCount, p.Leaf.postgres.QueryCount())
	require.Equal(t, wantRootQueryCount, p.Root.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// testMySQLRootCluster tests a scenario where a user connects
// to a MySQL database running in a root cluster.
func (p *DatabasePack) testMySQLRootCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.MySQL,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.MysqlService.Name,
			Protocol:    p.Root.MysqlService.Protocol,
			Username:    "root",
			// With MySQL database name doesn't matter as it's not subject to RBAC atm.
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.mysql.QueryCount() + 1
	wantLeafQueryCount := p.Leaf.mysql.QueryCount()

	// Execute a query.
	result, err := client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	require.Equal(t, wantRootQueryCount, p.Root.mysql.QueryCount())
	require.Equal(t, wantLeafQueryCount, p.Leaf.mysql.QueryCount())

	// Check if default Proxy MYSQL Engine Version was overridden the proxy settings.
	require.Equal(t, "8.0.1", client.GetServerVersion())

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}

// testMySQLLeafCluster tests a scenario where a user connects
// to a MySQL database running in a leaf cluster via a root cluster.
func (p *DatabasePack) testMySQLLeafCluster(t *testing.T) {
	// Connect to the database service in leaf cluster via root cluster.
	client, err := mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.MySQL, // Connecting via root cluster.
		Cluster:    p.Leaf.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Leaf.MysqlService.Name,
			Protocol:    p.Leaf.MysqlService.Protocol,
			Username:    "root",
			// With MySQL database name doesn't matter as it's not subject to RBAC atm.
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.mysql.QueryCount()
	wantLeafQueryCount := p.Leaf.mysql.QueryCount() + 1

	// Execute a query.
	result, err := client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	require.Equal(t, wantLeafQueryCount, p.Leaf.mysql.QueryCount())
	require.Equal(t, wantRootQueryCount, p.Root.mysql.QueryCount())

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}

// testMongoRootCluster tests a scenario where a user connects
// to a Mongo database running in a root cluster.
func (p *DatabasePack) testMongoRootCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.MongoService.Name,
			Protocol:    p.Root.MongoService.Protocol,
			Username:    "admin",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
	require.NoError(t, err)

	// Disconnect.
	err = client.Disconnect(context.Background())
	require.NoError(t, err)
}

// testMongoConnectionCount tests if mongo service releases
// resource after a mongo client disconnect.
func (p *DatabasePack) testMongoConnectionCount(t *testing.T) {
	connectMongoClient := func(t *testing.T) (serverConnectionCount int32) {
		// Connect to the database service in root cluster.
		client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
			AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
			AuthServer: p.Root.Cluster.Process.GetAuthServer(),
			Address:    p.Root.Cluster.Web,
			Cluster:    p.Root.Cluster.Secrets.SiteName,
			Username:   p.Root.User.GetName(),
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: p.Root.MongoService.Name,
				Protocol:    p.Root.MongoService.Protocol,
				Username:    "admin",
			},
		})
		require.NoError(t, err)

		// Execute a query.
		_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
		require.NoError(t, err)

		// Get a server connection count before disconnect.
		serverConnectionCount = p.Root.mongo.GetActiveConnectionsCount()

		// Disconnect.
		err = client.Disconnect(context.Background())
		require.NoError(t, err)

		return serverConnectionCount
	}

	// Get connection count while the first client is connected.
	initialConnectionCount := connectMongoClient(t)

	// Check if active connections count is not growing over time when new
	// clients connect to the mongo server.
	clientCount := 8
	for i := 0; i < clientCount; i++ {
		// Note that connection count per client fluctuates between 6 and 9.
		// Use InDelta to avoid flaky test.
		require.InDelta(t, initialConnectionCount, connectMongoClient(t), 3)
	}

	// Wait until the server reports no more connections. This usually happens
	// really quick but wait a little longer just in case.
	waitUntilNoConnections := func() bool {
		return p.Root.mongo.GetActiveConnectionsCount() == 0
	}
	require.Eventually(t, waitUntilNoConnections, 5*time.Second, 100*time.Millisecond)
}

// testMongoLeafCluster tests a scenario where a user connects
// to a Mongo database running in a leaf cluster.
func (p *DatabasePack) testMongoLeafCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web, // Connecting via root cluster.
		Cluster:    p.Leaf.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Leaf.MongoService.Name,
			Protocol:    p.Leaf.MongoService.Protocol,
			Username:    "admin",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
	require.NoError(t, err)

	// Disconnect.
	err = client.Disconnect(context.Background())
	require.NoError(t, err)
}

// TestRootLeafIdleTimeout tests idle client connection termination by proxy and DB services in
// trusted cluster setup.
func TestDatabaseRootLeafIdleTimeout(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	pack := SetupDatabaseTest(t, WithClock(clock))
	pack.WaitForLeaf(t)

	var (
		rootAuthServer = pack.Root.Cluster.Process.GetAuthServer()
		rootRole       = pack.Root.role
		leafAuthServer = pack.Leaf.Cluster.Process.GetAuthServer()
		leafRole       = pack.Leaf.role

		idleTimeout = time.Minute
	)

	rootAuthServer.SetClock(clockwork.NewFakeClockAt(time.Now()))
	leafAuthServer.SetClock(clockwork.NewFakeClockAt(time.Now()))

	mkMySQLLeafDBClient := func(t *testing.T) mysql.TestClientConn {
		// Connect to the database service in leaf cluster via root cluster.
		client, err := mysql.MakeTestClient(common.TestClientConfig{
			AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
			AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
			Address:    pack.Root.Cluster.MySQL, // Connecting via root cluster.
			Cluster:    pack.Leaf.Cluster.Secrets.SiteName,
			Username:   pack.Root.User.GetName(),
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: pack.Leaf.MysqlService.Name,
				Protocol:    pack.Leaf.MysqlService.Protocol,
				Username:    "root",
			},
		})
		require.NoError(t, err)
		return client
	}

	t.Run("root role without idle timeout", func(t *testing.T) {
		client := mkMySQLLeafDBClient(t)
		_, err := client.Execute("select 1")
		require.NoError(t, err)

		clock.Advance(idleTimeout)
		_, err = client.Execute("select 1")
		require.NoError(t, err)
		err = client.Close()
		require.NoError(t, err)
	})

	t.Run("root role with idle timeout", func(t *testing.T) {
		setRoleIdleTimeout(t, rootAuthServer, rootRole, idleTimeout)
		require.Eventually(t, func() bool {
			role, err := rootAuthServer.GetRole(context.Background(), rootRole.GetName())
			assert.NoError(t, err)
			return time.Duration(role.GetOptions().ClientIdleTimeout) == idleTimeout
		}, time.Second*2, time.Millisecond*200, "role idle timeout propagation filed")

		client := mkMySQLLeafDBClient(t)
		_, err := client.Execute("select 1")
		require.NoError(t, err)

		now := clock.Now()
		clock.Advance(idleTimeout)
		helpers.WaitForAuditEventTypeWithBackoff(t, pack.Root.Cluster.Process.GetAuthServer(), now, events.ClientDisconnectEvent)

		_, err = client.Execute("select 1")
		require.Error(t, err)
		setRoleIdleTimeout(t, rootAuthServer, rootRole, time.Hour)
	})

	t.Run("leaf role with idle timeout", func(t *testing.T) {
		setRoleIdleTimeout(t, leafAuthServer, leafRole, idleTimeout)
		require.Eventually(t, func() bool {
			role, err := leafAuthServer.GetRole(context.Background(), leafRole.GetName())
			assert.NoError(t, err)
			return time.Duration(role.GetOptions().ClientIdleTimeout) == idleTimeout
		}, time.Second*2, time.Millisecond*200, "role idle timeout propagation filed")

		client := mkMySQLLeafDBClient(t)
		_, err := client.Execute("select 1")
		require.NoError(t, err)

		now := clock.Now()
		clock.Advance(idleTimeout)
		helpers.WaitForAuditEventTypeWithBackoff(t, pack.Leaf.Cluster.Process.GetAuthServer(), now, events.ClientDisconnectEvent)

		_, err = client.Execute("select 1")
		require.Error(t, err)
		setRoleIdleTimeout(t, leafAuthServer, leafRole, time.Hour)
	})
}

// TestDatabaseAccessUnspecifiedHostname tests DB agent reverse tunnel connection in case where host address is
// unspecified thus is not present in the valid principal list. The DB agent should replace unspecified address (0.0.0.0)
// with localhost and successfully establish reverse tunnel connection.
func TestDatabaseAccessUnspecifiedHostname(t *testing.T) {
	pack := SetupDatabaseTest(t,
		WithNodeName("0.0.0.0"),
	)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.Root.Cluster.GetSiteAPI(pack.Root.Cluster.Secrets.SiteName),
		AuthServer: pack.Root.Cluster.Process.GetAuthServer(),
		Address:    pack.Root.Cluster.Web,
		Cluster:    pack.Root.Cluster.Secrets.SiteName,
		Username:   pack.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.Root.PostgresService.Name,
			Protocol:    pack.Root.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.Root.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.Leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

func (p *DatabasePack) testPostgresSeparateListener(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Postgres,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.PostgresService.Name,
			Protocol:    p.Root.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.postgres.QueryCount() + 1
	wantLeafQueryCount := p.Root.postgres.QueryCount()

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, wantRootQueryCount, p.Root.postgres.QueryCount())
	require.Equal(t, wantLeafQueryCount, p.Leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessPostgresSeparateListener tests postgres proxy listener running on separate port
// with DisableTLS.
func TestDatabaseAccessPostgresSeparateListenerTLSDisabled(t *testing.T) {
	pack := SetupDatabaseTest(t,
		WithListenerSetupDatabaseTest(helpers.SeparatePostgresPortSetup),
		WithRootConfig(func(config *servicecfg.Config) {
			config.Proxy.DisableTLS = true
		}),
	)
	pack.testPostgresSeparateListener(t)
}

func init() {
	// Override database agents shuffle behavior to ensure they're always
	// tried in the same order during tests. Used for HA tests.
	db.SetShuffleFunc(dbconnect.ShuffleSort)
}

// testHARootCluster verifies that proxy falls back to a healthy
// database agent when multiple agents are serving the same database and one
// of them is down in a root cluster.
func (p *DatabasePack) testHARootCluster(t *testing.T) {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: p.Root.PostgresService.Name,
		},
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      p.Root.postgresAddr,
		},
	)
	require.NoError(t, err)
	// Insert a database server entry not backed by an actual running agent
	// to simulate a scenario when an agent is down but the resource hasn't
	// expired from the backend yet.
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: p.Root.PostgresService.Name,
	}, types.DatabaseServerSpecV3{
		Database: database,
		// To make sure unhealthy server is always picked in tests first, make
		// sure its host ID always compares as "smaller" as the tests sort
		// agents.
		HostID:   "0000",
		Hostname: "test",
	})
	require.NoError(t, err)

	_, err = p.Root.Cluster.Process.GetAuthServer().UpsertDatabaseServer(
		context.Background(), dbServer)
	require.NoError(t, err)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.PostgresService.Name,
			Protocol:    p.Root.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.postgres.QueryCount() + 1
	wantLeafQueryCount := p.Leaf.postgres.QueryCount()
	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, wantRootQueryCount, p.Root.postgres.QueryCount())
	require.Equal(t, wantLeafQueryCount, p.Leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// testHALeafCluster verifies that proxy falls back to a healthy
// database agent when multiple agents are serving the same database and one
// of them is down in a leaf cluster.
func (p *DatabasePack) testHALeafCluster(t *testing.T) {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: p.Leaf.PostgresService.Name,
		},
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      p.Leaf.postgresAddr,
		},
	)
	require.NoError(t, err)
	// Insert a database server entry not backed by an actual running agent
	// to simulate a scenario when an agent is down but the resource hasn't
	// expired from the backend yet.
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: p.Leaf.PostgresService.Name,
	}, types.DatabaseServerSpecV3{
		Database: database,
		// To make sure unhealthy server is always picked in tests first, make
		// sure its host ID always compares as "smaller" as the tests sort
		// agents.
		HostID:   "0000",
		Hostname: "test",
	})
	require.NoError(t, err)

	_, err = p.Leaf.Cluster.Process.GetAuthServer().UpsertDatabaseServer(
		context.Background(), dbServer)
	require.NoError(t, err)

	// Connect to the database service in leaf cluster via root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web, // Connecting via root cluster.
		Cluster:    p.Leaf.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Leaf.PostgresService.Name,
			Protocol:    p.Leaf.PostgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	wantRootQueryCount := p.Root.postgres.QueryCount()
	wantLeafQueryCount := p.Leaf.postgres.QueryCount() + 1

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, wantLeafQueryCount, p.Leaf.postgres.QueryCount())
	require.Equal(t, wantRootQueryCount, p.Root.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// testDatabaseAccessMongoSeparateListener tests mongo proxy listener running on separate port.
func (p *DatabasePack) testMongoSeparateListener(t *testing.T) {
	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Mongo,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.MongoService.Name,
			Protocol:    p.Root.MongoService.Protocol,
			Username:    "admin",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
	require.NoError(t, err)

	// Disconnect.
	err = client.Disconnect(context.Background())
	require.NoError(t, err)
}

func (p *DatabasePack) testAgentState(t *testing.T) {
	tests := map[string]struct {
		agentParams databaseAgentStartParams
	}{
		"WithStaticDatabases": {
			agentParams: databaseAgentStartParams{
				databases: []servicecfg.Database{
					{Name: "mysql", Protocol: defaults.ProtocolMySQL, URI: "localhost:3306"},
					{Name: "pg", Protocol: defaults.ProtocolPostgres, URI: "localhost:5432"},
				},
			},
		},
		"WithResourceMatchers": {
			agentParams: databaseAgentStartParams{
				resourceMatchers: []services.ResourceMatcher{
					{Labels: types.Labels{"*": []string{"*"}}},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Start also ensures that the database agent has the “ready” state.
			// If the agent can’t make it, this function will fail the test.
			agent, _ := p.startRootDatabaseAgent(t, test.agentParams)

			// In addition to the checks performed during the agent start,
			// we’ll request the diagnostic server to ensure the readyz route
			// is returning to the proper state.
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%v/readyz", agent.Config.DiagnosticAddr.Addr), nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
		})
	}
}

// testCassandraRootCluster tests a scenario where a user connects
// to a Cassandra database running in a root cluster.
func (p *DatabasePack) testCassandraRootCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	dbConn, err := cassandra.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web,
		Cluster:    p.Root.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Root.CassandraService.Name,
			Protocol:    p.Root.CassandraService.Protocol,
			Username:    "cassandra",
		},
	})
	require.NoError(t, err)

	var clusterName string
	err = dbConn.Query("select cluster_name from system.local").Scan(&clusterName)
	require.NoError(t, err)
	require.Equal(t, "Test Cluster", clusterName)
	dbConn.Close()
}

// testCassandraLeafCluster tests a scenario where a user connects
// to a Cassandra database running in a root cluster.
func (p *DatabasePack) testCassandraLeafCluster(t *testing.T) {
	// Connect to the database service in root cluster.
	dbConn, err := cassandra.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: p.Root.Cluster.GetSiteAPI(p.Root.Cluster.Secrets.SiteName),
		AuthServer: p.Root.Cluster.Process.GetAuthServer(),
		Address:    p.Root.Cluster.Web,
		Cluster:    p.Leaf.Cluster.Secrets.SiteName,
		Username:   p.Root.User.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: p.Leaf.CassandraService.Name,
			Protocol:    p.Leaf.CassandraService.Protocol,
			Username:    "cassandra",
		},
	})
	require.NoError(t, err)

	var clusterName string
	err = dbConn.Query("select cluster_name from system.local").Scan(&clusterName)
	require.NoError(t, err)
	require.Equal(t, "Test Cluster", clusterName)
	dbConn.Close()
}

func setRoleIdleTimeout(t *testing.T, authServer *auth.Server, role types.Role, idleTimout time.Duration) {
	opts := role.GetOptions()
	opts.ClientIdleTimeout = types.Duration(idleTimout)
	role.SetOptions(opts)
	_, err := authServer.UpsertRole(context.Background(), role)
	require.NoError(t, err)
}
