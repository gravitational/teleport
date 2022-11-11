/*
Copyright 2020-2021 Gravitational, Inc.

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

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testlog"
)

// TestDatabaseAccessPostgresRootCluster tests a scenario where a user connects
// to a Postgres database running in a root cluster.
func TestDatabaseAccessPostgresRootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.postgresService.Name,
			Protocol:    pack.root.postgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.root.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessPostgresLeafCluster tests a scenario where a user connects
// to a Postgres database running in a leaf cluster via a root cluster.
func TestDatabaseAccessPostgresLeafCluster(t *testing.T) {
	pack := setupDatabaseTest(t)
	pack.waitForLeaf(t)

	// Connect to the database service in leaf cluster via root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()), // Connecting via root cluster.
		Cluster:    pack.leaf.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.leaf.postgresService.Name,
			Protocol:    pack.leaf.postgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.leaf.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.root.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessMySQLRootCluster tests a scenario where a user connects
// to a MySQL database running in a root cluster.
func TestDatabaseAccessMySQLRootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in root cluster.
	client, err := mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortMySQL()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.mysqlService.Name,
			Protocol:    pack.root.mysqlService.Protocol,
			Username:    "root",
			// With MySQL database name doesn't matter as it's not subject to RBAC atm.
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	require.Equal(t, uint32(1), pack.root.mysql.QueryCount())
	require.Equal(t, uint32(0), pack.leaf.mysql.QueryCount())

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}

// TestDatabaseAccessMySQLLeafCluster tests a scenario where a user connects
// to a MySQL database running in a leaf cluster via a root cluster.
func TestDatabaseAccessMySQLLeafCluster(t *testing.T) {
	pack := setupDatabaseTest(t)
	pack.waitForLeaf(t)

	// Connect to the database service in leaf cluster via root cluster.
	client, err := mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortMySQL()), // Connecting via root cluster.
		Cluster:    pack.leaf.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.leaf.mysqlService.Name,
			Protocol:    pack.leaf.mysqlService.Protocol,
			Username:    "root",
			// With MySQL database name doesn't matter as it's not subject to RBAC atm.
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	require.Equal(t, uint32(1), pack.leaf.mysql.QueryCount())
	require.Equal(t, uint32(0), pack.root.mysql.QueryCount())

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}

// TestDatabaseAccessMongoRootCluster tests a scenario where a user connects
// to a Mongo database running in a root cluster.
func TestDatabaseAccessMongoRootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.mongoService.Name,
			Protocol:    pack.root.mongoService.Protocol,
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

// TestDatabaseAccessMongoConnectionCount tests if mongo service releases
// resource after a mongo client disconnect.
func TestDatabaseAccessMongoConnectionCount(t *testing.T) {
	pack := setupDatabaseTest(t)

	connectMongoClient := func(t *testing.T) (serverConnectionCount int32) {
		// Connect to the database service in root cluster.
		client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
			AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
			AuthServer: pack.root.cluster.Process.GetAuthServer(),
			Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
			Cluster:    pack.root.cluster.Secrets.SiteName,
			Username:   pack.root.user.GetName(),
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: pack.root.mongoService.Name,
				Protocol:    pack.root.mongoService.Protocol,
				Username:    "admin",
			},
		})
		require.NoError(t, err)

		// Execute a query.
		_, err = client.Database("test").Collection("test").Find(context.Background(), bson.M{})
		require.NoError(t, err)

		// Get a server connection count before disconnect.
		serverConnectionCount = pack.root.mongo.GetActiveConnectionsCount()

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
		return 0 == pack.root.mongo.GetActiveConnectionsCount()
	}
	require.Eventually(t, waitUntilNoConnections, 5*time.Second, 100*time.Millisecond)
}

// TestDatabaseAccessMongoLeafCluster tests a scenario where a user connects
// to a Mongo database running in a leaf cluster.
func TestDatabaseAccessMongoLeafCluster(t *testing.T) {
	pack := setupDatabaseTest(t)
	pack.waitForLeaf(t)

	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()), // Connecting via root cluster.
		Cluster:    pack.leaf.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.leaf.mongoService.Name,
			Protocol:    pack.leaf.mongoService.Protocol,
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
	pack := setupDatabaseTest(t, withClock(clock))
	pack.waitForLeaf(t)

	var (
		rootAuthServer = pack.root.cluster.Process.GetAuthServer()
		rootRole       = pack.root.role
		leafAuthServer = pack.leaf.cluster.Process.GetAuthServer()
		leafRole       = pack.leaf.role

		idleTimeout = time.Minute
	)

	mkMySQLLeafDBClient := func(t *testing.T) *client.Conn {
		// Connect to the database service in leaf cluster via root cluster.
		client, err := mysql.MakeTestClient(common.TestClientConfig{
			AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
			AuthServer: pack.root.cluster.Process.GetAuthServer(),
			Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortMySQL()), // Connecting via root cluster.
			Cluster:    pack.leaf.cluster.Secrets.SiteName,
			Username:   pack.root.user.GetName(),
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: pack.leaf.mysqlService.Name,
				Protocol:    pack.leaf.mysqlService.Protocol,
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
		client := mkMySQLLeafDBClient(t)
		_, err := client.Execute("select 1")
		require.NoError(t, err)

		now := clock.Now()
		clock.Advance(idleTimeout)
		waitForAuditEventTypeWithBackoff(t, pack.root.cluster.Process.GetAuthServer(), now, events.ClientDisconnectEvent)

		_, err = client.Execute("select 1")
		require.Error(t, err)
		setRoleIdleTimeout(t, rootAuthServer, rootRole, time.Hour)
	})

	t.Run("leaf role with idle timeout", func(t *testing.T) {
		setRoleIdleTimeout(t, leafAuthServer, leafRole, idleTimeout)
		client := mkMySQLLeafDBClient(t)
		_, err := client.Execute("select 1")
		require.NoError(t, err)

		now := clock.Now()
		clock.Advance(idleTimeout)
		waitForAuditEventTypeWithBackoff(t, pack.leaf.cluster.Process.GetAuthServer(), now, events.ClientDisconnectEvent)

		_, err = client.Execute("select 1")
		require.Error(t, err)
		setRoleIdleTimeout(t, leafAuthServer, leafRole, time.Hour)
	})
}

// TestDatabaseAccessUnspecifiedHostname tests DB agent reverse tunnel connection in case where host address is
// unspecified thus is not present in the valid principal list. The DB agent should replace unspecified address (0.0.0.0)
// with localhost and successfully establish reverse tunnel connection.
func TestDatabaseAccessUnspecifiedHostname(t *testing.T) {
	pack := setupDatabaseTest(t,
		withNodeName("0.0.0.0"),
	)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.postgresService.Name,
			Protocol:    pack.root.postgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.root.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessPostgresSeparateListener tests postgres proxy listener running on separate port.
func TestDatabaseAccessPostgresSeparateListener(t *testing.T) {
	tests := []struct {
		desc       string
		disableTLS bool
	}{
		{desc: "With TLS enabled", disableTLS: false},
		{desc: "With TLS disabled", disableTLS: true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			pack := setupDatabaseTest(t,
				withPortSetupDatabaseTest(separatePostgresPortSetup),
				withRootConfig(func(config *service.Config) {
					config.Proxy.DisableTLS = tt.disableTLS
				}),
			)

			// Connect to the database service in root cluster.
			client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
				AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
				AuthServer: pack.root.cluster.Process.GetAuthServer(),
				Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortPostgres()),
				Cluster:    pack.root.cluster.Secrets.SiteName,
				Username:   pack.root.user.GetName(),
				RouteToDatabase: tlsca.RouteToDatabase{
					ServiceName: pack.root.postgresService.Name,
					Protocol:    pack.root.postgresService.Protocol,
					Username:    "postgres",
					Database:    "test",
				},
			})
			require.NoError(t, err)

			// Execute a query.
			result, err := client.Exec(context.Background(), "select 1").ReadAll()
			require.NoError(t, err)
			require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
			require.Equal(t, uint32(1), pack.root.postgres.QueryCount())
			require.Equal(t, uint32(0), pack.leaf.postgres.QueryCount())

			// Disconnect.
			err = client.Close(context.Background())
			require.NoError(t, err)
		})
	}
}

func init() {
	// Override database agents shuffle behavior to ensure they're always
	// tried in the same order during tests. Used for HA tests.
	db.SetShuffleFunc(db.ShuffleSort)
}

// TestDatabaseAccessHARootCluster verifies that proxy falls back to a healthy
// database agent when multiple agents are serving the same database and one
// of them is down in a root cluster.
func TestDatabaseAccessHARootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Insert a database server entry not backed by an actual running agent
	// to simulate a scenario when an agent is down but the resource hasn't
	// expired from the backend yet.
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: pack.root.postgresService.Name,
	}, types.DatabaseServerSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      pack.root.postgresAddr,
		// To make sure unhealthy server is always picked in tests first, make
		// sure its host ID always compares as "smaller" as the tests sort
		// agents.
		HostID:   "0000",
		Hostname: "test",
	})
	require.NoError(t, err)

	_, err = pack.root.cluster.Process.GetAuthServer().UpsertDatabaseServer(
		context.Background(), dbServer)
	require.NoError(t, err)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.postgresService.Name,
			Protocol:    pack.root.postgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.root.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessHALeafCluster verifies that proxy falls back to a healthy
// database agent when multiple agents are serving the same database and one
// of them is down in a leaf cluster.
func TestDatabaseAccessHALeafCluster(t *testing.T) {
	pack := setupDatabaseTest(t)
	pack.waitForLeaf(t)

	// Insert a database server entry not backed by an actual running agent
	// to simulate a scenario when an agent is down but the resource hasn't
	// expired from the backend yet.
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: pack.leaf.postgresService.Name,
	}, types.DatabaseServerSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      pack.leaf.postgresAddr,
		// To make sure unhealthy server is always picked in tests first, make
		// sure its host ID always compares as "smaller" as the tests sort
		// agents.
		HostID:   "0000",
		Hostname: "test",
	})
	require.NoError(t, err)

	_, err = pack.leaf.cluster.Process.GetAuthServer().UpsertDatabaseServer(
		context.Background(), dbServer)
	require.NoError(t, err)

	// Connect to the database service in leaf cluster via root cluster.
	client, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()), // Connecting via root cluster.
		Cluster:    pack.leaf.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.leaf.postgresService.Name,
			Protocol:    pack.leaf.postgresService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.leaf.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.root.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessMongoSeparateListener tests mongo proxy listener running on separate port.
func TestDatabaseAccessMongoSeparateListener(t *testing.T) {
	pack := setupDatabaseTest(t,
		withPortSetupDatabaseTest(separateMongoPortSetup),
	)

	// Connect to the database service in root cluster.
	client, err := mongodb.MakeTestClient(context.Background(), common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortMongo()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.mongoService.Name,
			Protocol:    pack.root.mongoService.Protocol,
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

func TestDatabaseAgentState(t *testing.T) {
	tests := map[string]struct {
		agentParams databaseAgentStartParams
	}{
		"WithStaticDatabases": {
			agentParams: databaseAgentStartParams{
				databases: []service.Database{
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
			pack := setupDatabaseTest(t)

			// Start also ensures that the database agent has the “ready” state.
			// If the agent can’t make it, this function will fail the test.
			agent, _ := pack.startRootDatabaseAgent(t, test.agentParams)

			// In addition to the checks performed during the agent start,
			// we’ll request the diagnostic server to ensure the readyz route
			// is returning to the proper state.
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%v/readyz", agent.Config.DiagnosticAddr.Addr), nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func waitForAuditEventTypeWithBackoff(t *testing.T, cli *auth.Server, startTime time.Time, eventType string) []apievents.AuditEvent {
	max := time.Second
	timeout := time.After(max)
	bf, err := utils.NewLinear(utils.LinearConfig{
		Step: max / 10,
		Max:  max,
	})
	if err != nil {
		t.Fatalf("failed to create linear backoff: %v", err)
	}
	for {
		events, _, err := cli.SearchEvents(startTime, time.Now().Add(time.Hour), apidefaults.Namespace, []string{eventType}, 100, types.EventOrderAscending, "")
		if err != nil {
			t.Fatalf("failed to call SearchEvents: %v", err)
		}
		if len(events) != 0 {
			return events
		}
		select {
		case <-bf.After():
			bf.Inc()
		case <-timeout:
			t.Fatalf("event type %q not found after %v", eventType, max)
		}
	}
}

func setRoleIdleTimeout(t *testing.T, authServer *auth.Server, role types.Role, idleTimout time.Duration) {
	opts := role.GetOptions()
	opts.ClientIdleTimeout = types.Duration(idleTimout)
	role.SetOptions(opts)
	err := authServer.UpsertRole(context.Background(), role)
	require.NoError(t, err)
}

type databasePack struct {
	root  databaseClusterPack
	leaf  databaseClusterPack
	clock clockwork.Clock
}

type databaseClusterPack struct {
	cluster         *TeleInstance
	user            types.User
	role            types.Role
	dbProcess       *service.TeleportProcess
	dbAuthClient    *auth.Client
	postgresService service.Database
	postgresAddr    string
	postgres        *postgres.TestServer
	mysqlService    service.Database
	mysqlAddr       string
	mysql           *mysql.TestServer
	mongoService    service.Database
	mongoAddr       string
	mongo           *mongodb.TestServer
}

type testOptions struct {
	clock             clockwork.Clock
	instancePortsFunc func() *InstancePorts
	rootConfig        func(config *service.Config)
	leafConfig        func(config *service.Config)
	nodeName          string
}

type testOptionFunc func(*testOptions)

func (o *testOptions) setDefaultIfNotSet() {
	if o.clock == nil {
		o.clock = clockwork.NewRealClock()
	}
	if o.instancePortsFunc == nil {
		o.instancePortsFunc = standardPortSetup
	}
	if o.nodeName == "" {
		o.nodeName = Host
	}
}

func withClock(clock clockwork.Clock) testOptionFunc {
	return func(o *testOptions) {
		o.clock = clock
	}
}

func withNodeName(nodeName string) testOptionFunc {
	return func(o *testOptions) {
		o.nodeName = nodeName
	}
}

func withPortSetupDatabaseTest(portFn func() *InstancePorts) testOptionFunc {
	return func(o *testOptions) {
		o.instancePortsFunc = portFn
	}
}

func withRootConfig(fn func(*service.Config)) testOptionFunc {
	return func(o *testOptions) {
		o.rootConfig = fn
	}
}

func withLeafConfig(fn func(*service.Config)) testOptionFunc {
	return func(o *testOptions) {
		o.leafConfig = fn
	}
}

func setupDatabaseTest(t *testing.T, options ...testOptionFunc) *databasePack {
	var opts testOptions
	for _, opt := range options {
		opt(&opts)
	}
	opts.setDefaultIfNotSet()

	// Some global setup.
	tracer := utils.NewTracer(utils.ThisFunction()).Start()
	t.Cleanup(func() { tracer.Stop() })
	lib.SetInsecureDevMode(true)
	SetTestTimeouts(100 * time.Millisecond)
	log := testlog.FailureOnly(t)

	// Generate keypair.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	p := &databasePack{
		clock: opts.clock,
		root: databaseClusterPack{
			postgresAddr: net.JoinHostPort("localhost", ports.Pop()),
			mysqlAddr:    net.JoinHostPort("localhost", ports.Pop()),
			mongoAddr:    net.JoinHostPort("localhost", ports.Pop()),
		},
		leaf: databaseClusterPack{
			postgresAddr: net.JoinHostPort("localhost", ports.Pop()),
			mysqlAddr:    net.JoinHostPort("localhost", ports.Pop()),
			mongoAddr:    net.JoinHostPort("localhost", ports.Pop()),
		},
	}

	// Create root cluster.
	p.root.cluster = NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New(),
		NodeName:    opts.nodeName,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
		Ports:       opts.instancePortsFunc(),
	})

	// Create leaf cluster.
	p.leaf.cluster = NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New(),
		NodeName:    opts.nodeName,
		Ports:       opts.instancePortsFunc(),
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Clock = p.clock
	if opts.rootConfig != nil {
		opts.rootConfig(rcConf)
	}

	// Make leaf cluster config.
	lcConf := service.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true
	lcConf.Clock = p.clock
	if opts.leafConfig != nil {
		opts.rootConfig(lcConf)
	}

	// Establish trust b/w root and leaf.
	err = p.root.cluster.CreateEx(t, p.leaf.cluster.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)
	err = p.leaf.cluster.CreateEx(t, p.root.cluster.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)

	// Start both clusters.
	err = p.leaf.cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.leaf.cluster.StopAll()
	})
	err = p.root.cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.root.cluster.StopAll()
	})

	// Setup users and roles on both clusters.
	p.setupUsersAndRoles(t)

	// Update root's certificate authority on leaf to configure role mapping.
	ca, err := p.leaf.cluster.Process.GetAuthServer().GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.UserCA,
		DomainName: p.root.cluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(types.RoleMap{
		{Remote: p.root.role.GetName(), Local: []string{p.leaf.role.GetName()}},
	})
	err = p.leaf.cluster.Process.GetAuthServer().UpsertCertAuthority(ca)
	require.NoError(t, err)

	// Create and start database services in the root cluster.
	p.root.postgresService = service.Database{
		Name:     "root-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.root.postgresAddr,
	}
	p.root.mysqlService = service.Database{
		Name:     "root-mysql",
		Protocol: defaults.ProtocolMySQL,
		URI:      p.root.mysqlAddr,
	}
	p.root.mongoService = service.Database{
		Name:     "root-mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      p.root.mongoAddr,
	}
	rdConf := service.MakeDefaultConfig()
	rdConf.DataDir = t.TempDir()
	rdConf.Token = "static-token-value"
	rdConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.root.cluster.GetPortWeb()),
		},
	}
	rdConf.Databases.Enabled = true
	rdConf.Databases.Databases = []service.Database{
		p.root.postgresService,
		p.root.mysqlService,
		p.root.mongoService,
	}
	rdConf.Clock = p.clock
	p.root.dbProcess, p.root.dbAuthClient, err = p.root.cluster.StartDatabase(rdConf)
	require.NoError(t, err)

	t.Cleanup(func() {
		p.root.dbProcess.Close()
	})

	// Create and start database services in the leaf cluster.
	p.leaf.postgresService = service.Database{
		Name:     "leaf-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.leaf.postgresAddr,
	}
	p.leaf.mysqlService = service.Database{
		Name:     "leaf-mysql",
		Protocol: defaults.ProtocolMySQL,
		URI:      p.leaf.mysqlAddr,
	}
	p.leaf.mongoService = service.Database{
		Name:     "leaf-mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      p.leaf.mongoAddr,
	}
	ldConf := service.MakeDefaultConfig()
	ldConf.DataDir = t.TempDir()
	ldConf.Token = "static-token-value"
	ldConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.leaf.cluster.GetPortWeb()),
		},
	}
	ldConf.Databases.Enabled = true
	ldConf.Databases.Databases = []service.Database{
		p.leaf.postgresService,
		p.leaf.mysqlService,
		p.leaf.mongoService,
	}
	ldConf.Clock = p.clock
	p.leaf.dbProcess, p.leaf.dbAuthClient, err = p.leaf.cluster.StartDatabase(ldConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		p.leaf.dbProcess.Close()
	})

	// Create and start test Postgres in the root cluster.
	p.root.postgres, err = postgres.NewTestServer(common.TestServerConfig{
		AuthClient: p.root.dbAuthClient,
		Name:       p.root.postgresService.Name,
		Address:    p.root.postgresAddr,
	})
	require.NoError(t, err)
	go p.root.postgres.Serve()
	t.Cleanup(func() {
		p.root.postgres.Close()
	})

	// Create and start test MySQL in the root cluster.
	p.root.mysql, err = mysql.NewTestServer(common.TestServerConfig{
		AuthClient: p.root.dbAuthClient,
		Name:       p.root.mysqlService.Name,
		Address:    p.root.mysqlAddr,
	})
	require.NoError(t, err)
	go p.root.mysql.Serve()
	t.Cleanup(func() {
		p.root.mysql.Close()
	})

	// Create and start test Mongo in the root cluster.
	p.root.mongo, err = mongodb.NewTestServer(common.TestServerConfig{
		AuthClient: p.root.dbAuthClient,
		Name:       p.root.mongoService.Name,
		Address:    p.root.mongoAddr,
	})
	require.NoError(t, err)
	go p.root.mongo.Serve()
	t.Cleanup(func() {
		p.root.mongo.Close()
	})

	// Create and start test Postgres in the leaf cluster.
	p.leaf.postgres, err = postgres.NewTestServer(common.TestServerConfig{
		AuthClient: p.leaf.dbAuthClient,
		Name:       p.leaf.postgresService.Name,
		Address:    p.leaf.postgresAddr,
	})
	require.NoError(t, err)
	go p.leaf.postgres.Serve()
	t.Cleanup(func() {
		p.leaf.postgres.Close()
	})

	// Create and start test MySQL in the leaf cluster.
	p.leaf.mysql, err = mysql.NewTestServer(common.TestServerConfig{
		AuthClient: p.leaf.dbAuthClient,
		Name:       p.leaf.mysqlService.Name,
		Address:    p.leaf.mysqlAddr,
	})
	require.NoError(t, err)
	go p.leaf.mysql.Serve()
	t.Cleanup(func() {
		p.leaf.mysql.Close()
	})

	// Create and start test Mongo in the leaf cluster.
	p.leaf.mongo, err = mongodb.NewTestServer(common.TestServerConfig{
		AuthClient: p.leaf.dbAuthClient,
		Name:       p.leaf.mongoService.Name,
		Address:    p.leaf.mongoAddr,
	})
	require.NoError(t, err)
	go p.leaf.mongo.Serve()
	t.Cleanup(func() {
		p.leaf.mongo.Close()
	})

	return p
}

func (p *databasePack) setupUsersAndRoles(t *testing.T) {
	var err error

	p.root.user, p.root.role, err = auth.CreateUserAndRole(p.root.cluster.Process.GetAuthServer(), "root-user", nil)
	require.NoError(t, err)

	p.root.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.root.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	err = p.root.cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.root.role)
	require.NoError(t, err)

	p.leaf.user, p.leaf.role, err = auth.CreateUserAndRole(p.root.cluster.Process.GetAuthServer(), "leaf-user", nil)
	require.NoError(t, err)

	p.leaf.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.leaf.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	err = p.leaf.cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.leaf.role)
	require.NoError(t, err)
}

func (p *databasePack) waitForLeaf(t *testing.T) {
	site, err := p.root.cluster.Tunnel.GetSite(p.leaf.cluster.Secrets.SiteName)
	require.NoError(t, err)

	accessPoint, err := site.CachingAccessPoint()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-time.Tick(500 * time.Millisecond):
			servers, err := accessPoint.GetDatabaseServers(ctx, apidefaults.Namespace)
			if err != nil {
				logrus.WithError(err).Debugf("Leaf cluster access point is unavailable.")
				continue
			}
			if !containsDB(servers, p.leaf.mysqlService.Name) {
				logrus.WithError(err).Debugf("Leaf db service %q is unavailable.", p.leaf.mysqlService.Name)
				continue
			}
			if !containsDB(servers, p.leaf.postgresService.Name) {
				logrus.WithError(err).Debugf("Leaf db service %q is unavailable.", p.leaf.postgresService.Name)
				continue
			}
			return
		case <-ctx.Done():
			t.Fatal("Leaf cluster access point is unavailable.")
		}
	}
}

// databaseAgentStartParams parameters used to configure a database agent.
type databaseAgentStartParams struct {
	databases        []service.Database
	resourceMatchers []services.ResourceMatcher
}

// startRootDatabaseAgent starts a database agent with the provided
// configuration on the root cluster.
func (p *databasePack) startRootDatabaseAgent(t *testing.T, params databaseAgentStartParams) (*service.TeleportProcess, *auth.Client) {
	conf := service.MakeDefaultConfig()
	conf.DataDir = t.TempDir()
	conf.Token = "static-token-value"
	conf.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("localhost", ports.Pop())}
	conf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.root.cluster.GetPortWeb()),
		},
	}
	conf.Clock = p.clock
	conf.Databases.Enabled = true
	conf.Databases.Databases = params.databases
	conf.Databases.ResourceMatchers = params.resourceMatchers

	server, authClient, err := p.root.cluster.StartDatabase(conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Close()
	})

	return server, authClient
}

func containsDB(servers []types.DatabaseServer, name string) bool {
	for _, server := range servers {
		if server.GetDatabase().GetName() == name {
			return true
		}
	}
	return false
}

// TestDatabaseAccessLargeQuery tests a scenario where a user connects
// to a MySQL database running in a root cluster.
func TestDatabaseAccessLargeQuery(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in root cluster.
	client, err := mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortMySQL()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.mysqlService.Name,
			Protocol:    pack.root.mysqlService.Protocol,
			Username:    "root",
		},
	})
	require.NoError(t, err)

	now := time.Now()
	query := fmt.Sprintf("select %s", strings.Repeat("A", 100*1024))
	result, err := client.Execute(query)
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	result.Close()

	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	result.Close()

	ee := waitForAuditEventTypeWithBackoff(t, pack.root.cluster.Process.GetAuthServer(), now, events.DatabaseSessionQueryEvent)
	require.Len(t, ee, 1)

	query = "select 1"
	result, err = client.Execute(query)
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	result.Close()

	require.Eventually(t, func() bool {
		ee := waitForAuditEventTypeWithBackoff(t, pack.root.cluster.Process.GetAuthServer(), now, events.DatabaseSessionQueryEvent)
		return len(ee) == 2
	}, time.Second*3, time.Millisecond*500)

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}
