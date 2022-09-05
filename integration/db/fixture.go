package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type DatabasePack struct {
	Root  databaseClusterPack
	Leaf  databaseClusterPack
	clock clockwork.Clock
}

type databaseClusterPack struct {
	Cluster         *helpers.TeleInstance
	User            types.User
	role            types.Role
	dbProcess       *service.TeleportProcess
	dbAuthClient    *auth.Client
	PostgresService service.Database
	postgresAddr    string
	postgres        *postgres.TestServer
	MysqlService    service.Database
	mysqlAddr       string
	mysql           *mysql.TestServer
	MongoService    service.Database
	mongoAddr       string
	mongo           *mongodb.TestServer
	fds             []service.FileDescriptor
}

type testOptions struct {
	clock         clockwork.Clock
	listenerSetup helpers.InstanceListenerSetupFunc
	rootConfig    func(config *service.Config)
	leafConfig    func(config *service.Config)
	nodeName      string
}

type testOptionFunc func(*testOptions)

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

func WithListenerSetupDatabaseTest(fn helpers.InstanceListenerSetupFunc) testOptionFunc {
	return func(o *testOptions) {
		o.listenerSetup = fn
	}
}

func WithRootConfig(fn func(*service.Config)) testOptionFunc {
	return func(o *testOptions) {
		o.rootConfig = fn
	}
}

func WithLeafConfig(fn func(*service.Config)) testOptionFunc {
	return func(o *testOptions) {
		o.leafConfig = fn
	}
}

func newDatabaseClusterPack(t *testing.T) databaseClusterPack {
	pack := databaseClusterPack{}
	pack.postgresAddr = helpers.NewListenerOn(t, "localhost", service.ListenerProxyPostgres, &pack.fds)
	pack.mysqlAddr = helpers.NewListenerOn(t, "localhost", service.ListenerProxyMySQL, &pack.fds)
	pack.mongoAddr = helpers.NewListenerOn(t, "localhost", service.ListenerProxyMongo, &pack.fds)
	return pack
}

func SetupDatabaseTest(t *testing.T, options ...testOptionFunc) *DatabasePack {
	var opts testOptions
	for _, opt := range options {
		opt(&opts)
	}
	opts.setDefaultIfNotSet()

	// Some global setup.
	tracer := utils.NewTracer(utils.ThisFunction()).Start()
	t.Cleanup(func() { tracer.Stop() })
	lib.SetInsecureDevMode(true)
	log := utils.NewLoggerForTests()

	// Generate keypair.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	p := &DatabasePack{
		clock: opts.clock,
		Root:  newDatabaseClusterPack(t),
		Leaf:  newDatabaseClusterPack(t),
	}

	// Create root cluster.
	rootCfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    opts.nodeName,
		Priv:        privateKey,
		Pub:         publicKey,
		Log:         log,
		Fds:         p.Root.fds,
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
		Log:         log,
		Fds:         p.Leaf.fds,
	}
	leafCfg.Listeners = opts.listenerSetup(t, &leafCfg.Fds)
	p.Leaf.Cluster = helpers.NewInstance(t, leafCfg)

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
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
	lcConf := service.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true
	lcConf.Clock = p.clock
	lcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	if opts.leafConfig != nil {
		opts.rootConfig(lcConf)
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
	ca, err := p.Leaf.Cluster.Process.GetAuthServer().GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.UserCA,
		DomainName: p.Root.Cluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(types.RoleMap{
		{Remote: p.Root.role.GetName(), Local: []string{p.Leaf.role.GetName()}},
	})
	err = p.Leaf.Cluster.Process.GetAuthServer().UpsertCertAuthority(ca)
	require.NoError(t, err)

	// Create and start database services in the root cluster.
	p.Root.PostgresService = service.Database{
		Name:     "root-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.Root.postgresAddr,
	}
	p.Root.MysqlService = service.Database{
		Name:     "root-mysql",
		Protocol: defaults.ProtocolMySQL,
		URI:      p.Root.mysqlAddr,
	}
	p.Root.MongoService = service.Database{
		Name:     "root-mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      p.Root.mongoAddr,
	}
	rdConf := service.MakeDefaultConfig()
	rdConf.DataDir = t.TempDir()
	rdConf.SetToken("static-token-value")
	rdConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        p.Root.Cluster.Web,
		},
	}
	rdConf.Databases.Enabled = true
	rdConf.Databases.Databases = []service.Database{
		p.Root.PostgresService,
		p.Root.MysqlService,
		p.Root.MongoService,
	}
	rdConf.Clock = p.clock
	rdConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	p.Root.dbProcess, p.Root.dbAuthClient, err = p.Root.Cluster.StartDatabase(rdConf)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, p.Root.dbProcess.Close()) })

	// Create and start database services in the leaf cluster.
	p.Leaf.PostgresService = service.Database{
		Name:     "leaf-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.Leaf.postgresAddr,
	}
	p.Leaf.MysqlService = service.Database{
		Name:     "leaf-mysql",
		Protocol: defaults.ProtocolMySQL,
		URI:      p.Leaf.mysqlAddr,
	}
	p.Leaf.MongoService = service.Database{
		Name:     "leaf-mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      p.Leaf.mongoAddr,
	}
	ldConf := service.MakeDefaultConfig()
	ldConf.DataDir = t.TempDir()
	ldConf.SetToken("static-token-value")
	ldConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        p.Leaf.Cluster.Web,
		},
	}
	ldConf.Databases.Enabled = true
	ldConf.Databases.Databases = []service.Database{
		p.Leaf.PostgresService,
		p.Leaf.MysqlService,
		p.Leaf.MongoService,
	}
	ldConf.Clock = p.clock
	ldConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	p.Leaf.dbProcess, p.Leaf.dbAuthClient, err = p.Leaf.Cluster.StartDatabase(ldConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		p.Leaf.dbProcess.Close()
	})

	// Create and start test Postgres in the root cluster.
	p.Root.postgres, err = postgres.NewTestServer(common.TestServerConfig{
		AuthClient: p.Root.dbAuthClient,
		Name:       p.Root.PostgresService.Name,
		Address:    p.Root.postgresAddr,
	})
	require.NoError(t, err)
	go p.Root.postgres.Serve()
	t.Cleanup(func() {
		p.Root.postgres.Close()
	})

	// Create and start test MySQL in the root cluster.
	p.Root.mysql, err = mysql.NewTestServer(common.TestServerConfig{
		AuthClient: p.Root.dbAuthClient,
		Name:       p.Root.MysqlService.Name,
		Address:    p.Root.mysqlAddr,
	})
	require.NoError(t, err)
	go p.Root.mysql.Serve()
	t.Cleanup(func() {
		p.Root.mysql.Close()
	})

	// Create and start test Mongo in the root cluster.
	p.Root.mongo, err = mongodb.NewTestServer(common.TestServerConfig{
		AuthClient: p.Root.dbAuthClient,
		Name:       p.Root.MongoService.Name,
		Address:    p.Root.mongoAddr,
	})
	require.NoError(t, err)
	go p.Root.mongo.Serve()
	t.Cleanup(func() {
		p.Root.mongo.Close()
	})

	// Create and start test Postgres in the leaf cluster.
	p.Leaf.postgres, err = postgres.NewTestServer(common.TestServerConfig{
		AuthClient: p.Leaf.dbAuthClient,
		Name:       p.Leaf.PostgresService.Name,
		Address:    p.Leaf.postgresAddr,
	})
	require.NoError(t, err)
	go p.Leaf.postgres.Serve()
	t.Cleanup(func() {
		p.Leaf.postgres.Close()
	})

	// Create and start test MySQL in the leaf cluster.
	p.Leaf.mysql, err = mysql.NewTestServer(common.TestServerConfig{
		AuthClient: p.Leaf.dbAuthClient,
		Name:       p.Leaf.MysqlService.Name,
		Address:    p.Leaf.mysqlAddr,
	})
	require.NoError(t, err)
	go p.Leaf.mysql.Serve()
	t.Cleanup(func() {
		p.Leaf.mysql.Close()
	})

	// Create and start test Mongo in the leaf cluster.
	p.Leaf.mongo, err = mongodb.NewTestServer(common.TestServerConfig{
		AuthClient: p.Leaf.dbAuthClient,
		Name:       p.Leaf.MongoService.Name,
		Address:    p.Leaf.mongoAddr,
	})
	require.NoError(t, err)
	go p.Leaf.mongo.Serve()
	t.Cleanup(func() {
		p.Leaf.mongo.Close()
	})

	return p
}

func (p *DatabasePack) setupUsersAndRoles(t *testing.T) {
	var err error

	p.Root.User, p.Root.role, err = auth.CreateUserAndRole(p.Root.Cluster.Process.GetAuthServer(), "root-user", nil)
	require.NoError(t, err)

	p.Root.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.Root.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	err = p.Root.Cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.Root.role)
	require.NoError(t, err)

	p.Leaf.User, p.Leaf.role, err = auth.CreateUserAndRole(p.Root.Cluster.Process.GetAuthServer(), "leaf-user", nil)
	require.NoError(t, err)

	p.Leaf.role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	p.Leaf.role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	err = p.Leaf.Cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.Leaf.role)
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

	for {
		select {
		case <-time.Tick(500 * time.Millisecond):
			servers, err := accessPoint.GetDatabaseServers(ctx, apidefaults.Namespace)
			if err != nil {
				// Use root logger as we need a configured logger instance and the root cluster have one.
				p.Root.Cluster.Log.WithError(err).Debugf("Leaf cluster access point is unavailable.")
				continue
			}
			if !containsDB(servers, p.Leaf.MysqlService.Name) {
				p.Root.Cluster.Log.WithError(err).Debugf("Leaf db service %q is unavailable.", p.Leaf.MysqlService.Name)
				continue
			}
			if !containsDB(servers, p.Leaf.PostgresService.Name) {
				p.Root.Cluster.Log.WithError(err).Debugf("Leaf db service %q is unavailable.", p.Leaf.PostgresService.Name)
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
func (p *DatabasePack) startRootDatabaseAgent(t *testing.T, params databaseAgentStartParams) (*service.TeleportProcess, *auth.Client) {
	conf := service.MakeDefaultConfig()
	conf.DataDir = t.TempDir()
	conf.SetToken("static-token-value")
	conf.DiagnosticAddr = *utils.MustParseAddr(helpers.NewListener(t, service.ListenerDiagnostic, &conf.FileDescriptors))
	conf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        p.Root.Cluster.Web,
		},
	}
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
