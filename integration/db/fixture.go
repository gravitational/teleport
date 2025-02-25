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
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cassandra"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type DatabasePack struct {
	Root  databaseClusterPack
	Leaf  databaseClusterPack
	clock clockwork.Clock
}

type databaseClusterPack struct {
	Cluster          *helpers.TeleInstance
	User             types.User
	role             types.Role
	dbProcess        *service.TeleportProcess
	dbAuthClient     *authclient.Client
	PostgresService  servicecfg.Database
	postgresAddr     string
	postgres         *postgres.TestServer
	MysqlService     servicecfg.Database
	mysqlAddr        string
	mysql            *mysql.TestServer
	MongoService     servicecfg.Database
	mongoAddr        string
	mongo            *mongodb.TestServer
	name             string
	CassandraService servicecfg.Database
	cassandra        *cassandra.TestServer
	cassandraAddr    string
}

func mustListen(t *testing.T) (net.Listener, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return listener, net.JoinHostPort("localhost", port)
}

func (pack *databaseClusterPack) StartDatabaseServices(t *testing.T, clock clockwork.Clock) {
	var err error

	var postgresListener, mysqlListener, mongoListener, cassandaListener net.Listener

	postgresListener, pack.postgresAddr = mustListen(t)
	pack.PostgresService = servicecfg.Database{
		Name:     fmt.Sprintf("%s-postgres", pack.name),
		Protocol: defaults.ProtocolPostgres,
		URI:      pack.postgresAddr,
	}

	mysqlListener, pack.mysqlAddr = mustListen(t)
	pack.MysqlService = servicecfg.Database{
		Name:     fmt.Sprintf("%s-mysql", pack.name),
		Protocol: defaults.ProtocolMySQL,
		URI:      pack.mysqlAddr,
	}

	mongoListener, pack.mongoAddr = mustListen(t)
	pack.MongoService = servicecfg.Database{
		Name:     fmt.Sprintf("%s-mongo", pack.name),
		Protocol: defaults.ProtocolMongoDB,
		URI:      pack.mongoAddr,
	}

	cassandaListener, pack.cassandraAddr = mustListen(t)
	pack.CassandraService = servicecfg.Database{
		Name:     fmt.Sprintf("%s-cassandra", pack.name),
		Protocol: defaults.ProtocolCassandra,
		URI:      pack.cassandraAddr,
	}

	conf := servicecfg.MakeDefaultConfig()
	conf.DataDir = filepath.Join(t.TempDir(), pack.name)
	conf.SetToken("static-token-value")

	conf.SetAuthServerAddress(utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        pack.Cluster.Web,
	})

	conf.Databases.Enabled = true
	conf.Databases.Databases = []servicecfg.Database{
		pack.PostgresService,
		pack.MysqlService,
		pack.MongoService,
		pack.CassandraService,
	}
	conf.Clock = clock
	conf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	pack.dbProcess, pack.dbAuthClient, err = pack.Cluster.StartDatabase(conf)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, pack.dbProcess.Close()) })

	// Create and start test Postgres in the leaf cluster.
	pack.postgres, err = postgres.NewTestServer(common.TestServerConfig{
		AuthClient: pack.dbAuthClient,
		Name:       pack.PostgresService.Name,
		Listener:   postgresListener,
	})
	require.NoError(t, err)
	go pack.postgres.Serve()
	t.Cleanup(func() { pack.postgres.Close() })

	// Create and start test MySQL in the leaf cluster.
	pack.mysql, err = mysql.NewTestServer(common.TestServerConfig{
		AuthClient: pack.dbAuthClient,
		Name:       pack.MysqlService.Name,
		Listener:   mysqlListener,
	})
	require.NoError(t, err)
	go pack.mysql.Serve()
	t.Cleanup(func() { pack.mysql.Close() })

	// Create and start test Mongo in the leaf cluster.
	pack.mongo, err = mongodb.NewTestServer(common.TestServerConfig{
		AuthClient: pack.dbAuthClient,
		Name:       pack.MongoService.Name,
		Listener:   mongoListener,
	})
	require.NoError(t, err)
	go pack.mongo.Serve()
	t.Cleanup(func() { pack.mongo.Close() })

	pack.cassandra, err = cassandra.NewTestServer(common.TestServerConfig{
		AuthClient: pack.dbAuthClient,
		Name:       pack.CassandraService.Name,
		Listener:   cassandaListener,
	})
	require.NoError(t, err)
	go pack.cassandra.Serve()
	t.Cleanup(func() { pack.cassandra.Close() })

	helpers.WaitForDatabaseServers(t, pack.Cluster.Process.GetAuthServer(), conf.Databases.Databases)
}

type testOptions struct {
	clock         clockwork.Clock
	listenerSetup helpers.InstanceListenerSetupFunc
	rootConfig    func(config *servicecfg.Config)
	leafConfig    func(config *servicecfg.Config)
	nodeName      string
}

type TestOptionFunc func(*testOptions)

func (o *testOptions) setDefaultIfNotSet() {
	if o.clock == nil {
		o.clock = clockwork.NewRealClock()
	}
	if o.listenerSetup == nil {
		o.listenerSetup = helpers.StandardListenerSetup
	}
	if o.nodeName == "" {
		o.nodeName = helpers.Host
	}
}

func WithClock(clock clockwork.Clock) TestOptionFunc {
	return func(o *testOptions) {
		o.clock = clock
	}
}

func WithNodeName(nodeName string) TestOptionFunc {
	return func(o *testOptions) {
		o.nodeName = nodeName
	}
}

func WithListenerSetupDatabaseTest(fn helpers.InstanceListenerSetupFunc) TestOptionFunc {
	return func(o *testOptions) {
		o.listenerSetup = fn
	}
}

func WithRootConfig(fn func(*servicecfg.Config)) TestOptionFunc {
	return func(o *testOptions) {
		o.rootConfig = fn
	}
}

func WithLeafConfig(fn func(*servicecfg.Config)) TestOptionFunc {
	return func(o *testOptions) {
		o.leafConfig = fn
	}
}

func SetupDatabaseTest(t *testing.T, options ...TestOptionFunc) *DatabasePack {
	ctx := context.Background()
	var opts testOptions
	for _, opt := range options {
		opt(&opts)
	}
	opts.setDefaultIfNotSet()

	// Some global setup.
	tracer := utils.NewTracer(utils.ThisFunction()).Start()
	t.Cleanup(func() { tracer.Stop() })
	lib.SetInsecureDevMode(true)
	log := utils.NewSlogLoggerForTests()

	// Generate keypair.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	p := &DatabasePack{
		clock: opts.clock,
		Root:  databaseClusterPack{name: "root"},
		Leaf:  databaseClusterPack{name: "leaf"},
	}

	// Create root cluster.
	rootCfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    opts.nodeName,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      log,
	}
	rootCfg.Listeners = opts.listenerSetup(t, &rootCfg.Fds)
	p.Root.Cluster = helpers.NewInstance(t, rootCfg)

	// Create leaf cluster.
	leafCfg := helpers.InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
		NodeName:    opts.nodeName,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      log,
	}
	leafCfg.Listeners = opts.listenerSetup(t, &leafCfg.Fds)
	p.Leaf.Cluster = helpers.NewInstance(t, leafCfg)

	// Make root cluster config.
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Clock = p.clock
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	if opts.rootConfig != nil {
		opts.rootConfig(rcConf)
	}

	// Make leaf cluster config.
	lcConf := servicecfg.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true
	lcConf.Clock = p.clock
	lcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	if opts.leafConfig != nil {
		opts.leafConfig(lcConf)
	}

	// Establish trust b/w root and leaf.
	err = p.Root.Cluster.CreateEx(t, p.Leaf.Cluster.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)
	err = p.Leaf.Cluster.CreateEx(t, p.Root.Cluster.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)

	// Start both clusters.
	err = p.Leaf.Cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.Leaf.Cluster.StopAll()
	})
	err = p.Root.Cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.Root.Cluster.StopAll()
	})

	// Setup users and roles on both clusters.
	p.setupUsersAndRoles(t)

	// Update root's certificate authority on leaf to configure role mapping.
	ca, err := p.Leaf.Cluster.Process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: p.Root.Cluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(types.RoleMap{
		{Remote: p.Root.role.GetName(), Local: []string{p.Leaf.role.GetName()}},
	})
	err = p.Leaf.Cluster.Process.GetAuthServer().UpsertCertAuthority(ctx, ca)
	require.NoError(t, err)

	// Start database service and test servers in the clusters
	p.StartDatabases(t)

	return p
}

func (p *DatabasePack) setupUsersAndRoles(t *testing.T) {
	var err error

	p.Root.User, p.Root.role, err = auth.CreateUserAndRole(p.Root.Cluster.Process.GetAuthServer(), "root-user", nil, nil)
	require.NoError(t, err)

	p.Root.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.Root.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	p.Root.role, err = p.Root.Cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.Root.role)
	require.NoError(t, err)

	p.Leaf.User, p.Leaf.role, err = auth.CreateUserAndRole(p.Root.Cluster.Process.GetAuthServer(), "leaf-user", nil, nil)
	require.NoError(t, err)

	p.Leaf.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.Leaf.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	p.Leaf.role, err = p.Leaf.Cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.Leaf.role)
	require.NoError(t, err)
}

func (p *DatabasePack) WaitForLeaf(t *testing.T) {
	helpers.WaitForProxyCount(p.Leaf.Cluster, p.Root.Cluster.Secrets.SiteName, 1)
	site, err := p.Root.Cluster.Tunnel.GetSite(p.Leaf.Cluster.Secrets.SiteName)
	require.NoError(t, err)

	accessPoint, err := site.CachingAccessPoint()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			servers, err := accessPoint.GetDatabaseServers(ctx, apidefaults.Namespace)
			if err != nil {
				// Use root logger as we need a configured logger instance and the root cluster have one.
				p.Root.Cluster.Log.DebugContext(ctx, "Leaf cluster access point is unavailable", "error", err)
				continue
			}
			if !containsDB(servers, p.Leaf.MysqlService.Name) {
				p.Root.Cluster.Log.DebugContext(ctx, "Leaf db service is unavailable", "error", err, "db_service", p.Leaf.MysqlService.Name)
				continue
			}
			if !containsDB(servers, p.Leaf.PostgresService.Name) {
				p.Root.Cluster.Log.DebugContext(ctx, "Leaf db service is unavailable", "error", err, "db_service", p.Leaf.PostgresService.Name)
				continue
			}
			return
		case <-ctx.Done():
			t.Fatal("Leaf cluster access point is unavailable.")
		}
	}
}

func (p *DatabasePack) StartDatabases(t *testing.T) {
	p.Root.StartDatabaseServices(t, p.clock)
	p.Leaf.StartDatabaseServices(t, p.clock)
}

// databaseAgentStartParams parameters used to configure a database agent.
type databaseAgentStartParams struct {
	databases        []servicecfg.Database
	resourceMatchers []services.ResourceMatcher
}

// startRootDatabaseAgent starts a database agent with the provided
// configuration on the root cluster.
func (p *DatabasePack) startRootDatabaseAgent(t *testing.T, params databaseAgentStartParams) (*service.TeleportProcess, *authclient.Client) {
	conf := servicecfg.MakeDefaultConfig()
	conf.DataDir = t.TempDir()
	conf.SetToken("static-token-value")
	conf.DiagnosticAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerDiagnostic, &conf.FileDescriptors))
	conf.SetAuthServerAddress(utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        p.Root.Cluster.Web,
	})
	conf.Clock = p.clock
	conf.Databases.Enabled = true
	conf.Databases.Databases = params.databases
	conf.Databases.ResourceMatchers = params.resourceMatchers
	conf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	server, authClient, err := p.Root.Cluster.StartDatabase(conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Close()
	})

	helpers.WaitForDatabaseServers(t, p.Root.Cluster.Process.GetAuthServer(), conf.Databases.Databases)
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

// testLargeQuery tests a scenario where a user connects
// to a MySQL database running in a root cluster.
func (p *DatabasePack) testLargeQuery(t *testing.T) {
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

	ee := helpers.WaitForAuditEventTypeWithBackoff(t, p.Root.Cluster.Process.GetAuthServer(), now, events.DatabaseSessionQueryEvent)
	require.Len(t, ee, 1)

	query = "select 1"
	result, err = client.Execute(query)
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, result)
	result.Close()

	require.Eventually(t, func() bool {
		ee := helpers.WaitForAuditEventTypeWithBackoff(t, p.Root.Cluster.Process.GetAuthServer(), now, events.DatabaseSessionQueryEvent)
		return len(ee) == 2
	}, time.Second*3, time.Millisecond*500)

	// Disconnect.
	err = client.Close()
	require.NoError(t, err)
}
