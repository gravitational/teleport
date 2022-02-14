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
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/redis"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/siddontang/go-mysql/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
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

func TestDatabaseAccessRedisRootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	serverConfig := common.TestServerConfig{
		AuthClient: pack.root.dbAuthClient,
		Name:       pack.root.redisService.Name,
		Address:    pack.root.redisAddr,
	}

	startRedis(t, serverConfig)

	config := &common.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    net.JoinHostPort(Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.redisService.Name,
			Protocol:    pack.root.redisService.Protocol,
			Username:    "admin",
		},
	}

	ctx := context.Background()

	db, err := redis.MakeTestClient(ctx, *config)
	require.NoError(t, err)

	err = db.Ping(ctx).Err()
	require.NoError(t, err)

	t.Run("get/set", func(t *testing.T) {
		err = db.Set(ctx, "testKey1", "abc", 0).Err()
		require.NoError(t, err)

		resp := db.Get(ctx, "testKey1")
		require.NoError(t, resp.Err())
		resVal, err := resp.Result()
		require.NoError(t, err)
		require.Equal(t, "abc", resVal)
	})

	t.Run("keys", func(t *testing.T) {
		resp := db.Keys(ctx, "*")
		require.NoError(t, resp.Err())
		resVal, err := resp.Result()
		require.NoError(t, err)
		require.Len(t, resVal, 1)
		require.Equal(t, "testKey1", resVal[0])
	})
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
	pack := setupDatabaseTest(t,
		withPortSetupDatabaseTest(separatePostgresPortSetup),
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

	redisService service.Database
	redisAddr    string
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
	log := utils.NewLoggerForTests()

	// Generate keypair.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	p := &databasePack{
		clock: opts.clock,
		root: databaseClusterPack{
			postgresAddr: net.JoinHostPort("localhost", ports.Pop()),
			mysqlAddr:    net.JoinHostPort("localhost", ports.Pop()),
			mongoAddr:    net.JoinHostPort("localhost", ports.Pop()),
			redisAddr:    net.JoinHostPort("localhost", ports.Pop()),
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
		HostID:      uuid.New().String(),
		NodeName:    opts.nodeName,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
		Ports:       opts.instancePortsFunc(),
	})

	// Create leaf cluster.
	p.leaf.cluster = NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
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
	ca, err := p.leaf.cluster.Process.GetAuthServer().GetCertAuthority(types.CertAuthID{
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
	p.root.redisService = service.Database{
		Name:     "root-redis",
		Protocol: defaults.ProtocolRedis,
		URI:      p.root.redisAddr,
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
		p.root.redisService,
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

func containsDB(servers []types.DatabaseServer, name string) bool {
	for _, server := range servers {
		if server.GetDatabase().GetName() == name {
			return true
		}
	}
	return false
}

// makeTestServerTLSConfig returns private key, client cert and CA as PEM files.
func makeTestServerTLSConfig(config common.TestServerConfig) ([]byte, []byte, []byte, error) {
	cn := config.CN
	if cn == "" {
		cn = "localhost"
	}
	privateKey, _, err := testauthority.New().GenerateKeyPair("")
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: cn,
	}, privateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	resp, err := config.AuthClient.GenerateDatabaseCert(context.Background(),
		&proto.DatabaseCertRequest{
			CSR:        csr,
			ServerName: cn,
			TTL:        proto.Duration(time.Hour),
		})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(resp.Cert, privateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	cas := make([]byte, 0)
	for _, ca := range resp.CACerts {
		cas = append(cas, ca...)
	}

	certPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Certificate[0],
		},
	)

	return privateKey, certPem, cas, nil
}

func startRedis(t *testing.T, config common.TestServerConfig) {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err, "could not connect to docker")

	privPem, cert, cas, err := makeTestServerTLSConfig(config)
	require.NoError(t, err)

	certDir := t.TempDir()

	// save server keys, so we can mount them in a container.
	err = os.WriteFile(fmt.Sprintf("%s/server.key", certDir), privPem, 0600)
	require.NoError(t, err)

	err = os.WriteFile(fmt.Sprintf("%s/server.crt", certDir), cert, 0600)
	require.NoError(t, err)

	err = os.WriteFile(fmt.Sprintf("%s/server.cas", certDir), cas, 0600)
	require.NoError(t, err)

	_, port, err := net.SplitHostPort(config.Address)
	require.NoError(t, err)

	// Run docker with Redis.
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "6.2",
		// change user uid and gid. Redis runs as 999 user, and it will
		// most likely refuse to load keys with different uid.
		User: fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		Mounts: []string{
			fmt.Sprintf("%s/server.crt:/certs/server.crt:ro", certDir),
			fmt.Sprintf("%s/server.key:/certs/server.key:ro", certDir),
			fmt.Sprintf("%s/server.cas:/certs/server.cas:ro", certDir),
		},
		Cmd: []string{
			"--port", "0",
			"--tls-port", "6379",
			"--tls-cert-file", "/certs/server.crt",
			"--tls-key-file", "/certs/server.key",
			"--tls-ca-cert-file", "/certs/server.cas",
		},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"6379/tcp": {{HostPort: port}},
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	require.NoError(t, err, "could not start resource")

	t.Cleanup(func() {
		// When you're done, kill and remove the container
		if err = pool.Purge(resource); err != nil {
			t.Fatalf("Could not purge resource: %s", err)
		}
	})
}
