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
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/inventory/metadata"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/db/cassandra"
	"github.com/gravitational/teleport/lib/srv/db/clickhouse"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/cloud/users"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/dynamodb"
	"github.com/gravitational/teleport/lib/srv/db/elasticsearch"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/objects"
	"github.com/gravitational/teleport/lib/srv/db/opensearch"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/redis"
	"github.com/gravitational/teleport/lib/srv/db/snowflake"
	"github.com/gravitational/teleport/lib/srv/db/spanner"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
	discoverycommon "github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/utils"
)

func init() {
	common.RegisterEngine(cassandra.NewEngine, defaults.ProtocolCassandra)
	common.RegisterEngine(elasticsearch.NewEngine, defaults.ProtocolElasticsearch)
	common.RegisterEngine(opensearch.NewEngine, defaults.ProtocolOpenSearch)
	common.RegisterEngine(mongodb.NewEngine, defaults.ProtocolMongoDB)
	common.RegisterEngine(mysql.NewEngine, defaults.ProtocolMySQL)
	common.RegisterEngine(postgres.NewEngine, defaults.ProtocolPostgres, defaults.ProtocolCockroachDB)
	common.RegisterEngine(redis.NewEngine, defaults.ProtocolRedis)
	common.RegisterEngine(snowflake.NewEngine, defaults.ProtocolSnowflake)
	common.RegisterEngine(sqlserver.NewEngine, defaults.ProtocolSQLServer)
	common.RegisterEngine(dynamodb.NewEngine, defaults.ProtocolDynamoDB)
	common.RegisterEngine(clickhouse.NewEngine, defaults.ProtocolClickHouse)
	common.RegisterEngine(clickhouse.NewEngine, defaults.ProtocolClickHouseHTTP)
	common.RegisterEngine(spanner.NewEngine, defaults.ProtocolSpanner)

	objects.RegisterObjectFetcher(postgres.NewObjectFetcher, defaults.ProtocolPostgres)
}

// Config is the configuration for a database proxy server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock
	// DataDir is the path to the data directory for the server.
	DataDir string
	// AuthClient is a client directly connected to the Auth server.
	AuthClient *authclient.Client
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.DatabaseAccessPoint
	// Emitter is used to emit audit events.
	Emitter apievents.Emitter
	// NewAudit allows to override audit logger in tests.
	NewAudit NewAuditFn
	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config
	// Limiter limits the number of connections per client IP.
	Limiter *limiter.Limiter
	// Authorizer is used to authorize requests coming from proxy.
	Authorizer authz.Authorizer
	// GetRotation returns the certificate rotation state.
	GetRotation func(role types.SystemRole) (*types.Rotation, error)
	// GetServerInfoFn returns function that returns database info for heartbeats.
	GetServerInfoFn func(database types.Database) func(context.Context) (*types.DatabaseServerV3, error)
	// Hostname is the hostname where this database server is running.
	Hostname string
	// HostID is the id of the host where this database server is running.
	HostID string
	// ResourceMatchers is a list of database resource matchers.
	ResourceMatchers []services.ResourceMatcher
	// AWSMatchers is a list of AWS databases matchers.
	AWSMatchers []types.AWSMatcher
	// AzureMatchers is a list of Azure databases matchers.
	AzureMatchers []types.AzureMatcher
	// Databases is a list of proxied databases from static configuration.
	Databases types.Databases
	// CloudLabels is a service that imports labels from a cloud provider. The labels are shared
	// between all databases.
	CloudLabels labels.Importer
	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)
	// OnReconcile is called after each database resource reconciliation.
	OnReconcile func(types.Databases)
	// Auth is responsible for generating database auth tokens.
	Auth common.Auth
	// CADownloader automatically downloads root certs for cloud hosted databases.
	CADownloader CADownloader
	// CloudClients creates cloud API clients.
	CloudClients clients.Clients
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// AWSDatabaseFetcherFactory provides AWS database fetchers
	AWSDatabaseFetcherFactory *db.AWSFetcherFactory
	// CloudMeta fetches cloud metadata for cloud hosted databases.
	CloudMeta *cloud.Metadata
	// CloudIAM configures IAM for cloud hosted databases.
	CloudIAM *cloud.IAM
	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter *reversetunnel.ConnectedProxyGetter
	// CloudUsers manage users for cloud hosted databases.
	CloudUsers *users.Users
	// DatabaseObjects manages database object importers.
	DatabaseObjects objects.Objects
	// ConnectionMonitor monitors and closes connections if session controls
	// prevent the connections.
	ConnectionMonitor ConnMonitor
	// ShutdownPollPeriod defines the shutdown poll period.
	ShutdownPollPeriod time.Duration
	// InventoryHandle is used to send db server heartbeats via the inventory control stream.
	InventoryHandle inventory.DownstreamHandle

	// discoveryResourceChecker performs some pre-checks when creating databases
	// discovered by the discovery service.
	discoveryResourceChecker cloud.DiscoveryResourceChecker
	// getEngineFn returns a [common.Engine]. It can be overridden in tests to
	// customize the returned engine.
	getEngineFn func(types.Database, common.EngineConfig) (common.Engine, error)
}

// NewAuditFn defines a function that creates an audit logger.
type NewAuditFn func(common.AuditConfig) (common.Audit, error)

// CheckAndSetDefaults makes sure the configuration has the minimum required
// to function.
func (c *Config) CheckAndSetDefaults(ctx context.Context) (err error) {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.DataDir == "" {
		return trace.BadParameter("missing DataDir")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.Emitter == nil {
		c.Emitter = c.AuthClient
	}
	if c.NewAudit == nil {
		c.NewAudit = common.NewAudit
	}
	if c.CloudClients == nil {
		cloudClients, err := clients.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.CloudClients = cloudClients
	}
	if c.AWSConfigProvider == nil {
		provider, err := awsconfig.NewCache()
		if err != nil {
			return trace.Wrap(err, "unable to create AWS config provider cache")
		}
		c.AWSConfigProvider = provider
	}
	if c.AWSDatabaseFetcherFactory == nil {
		factory, err := db.NewAWSFetcherFactory(db.AWSFetcherFactoryConfig{
			CloudClients:      c.CloudClients,
			AWSConfigProvider: c.AWSConfigProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		c.AWSDatabaseFetcherFactory = factory
	}
	if c.Auth == nil {
		c.Auth, err = common.NewAuth(common.AuthConfig{
			AuthClient:        c.AuthClient,
			AccessPoint:       c.AccessPoint,
			Clock:             c.Clock,
			Clients:           c.CloudClients,
			AWSConfigProvider: c.AWSConfigProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.Hostname == "" {
		return trace.BadParameter("missing Hostname")
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLSConfig")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.GetRotation == nil {
		return trace.BadParameter("missing GetRotation")
	}
	if c.CADownloader == nil {
		c.CADownloader = NewRealDownloader()
	}
	if c.ConnectionMonitor == nil {
		return trace.BadParameter("missing ConnectionMonitor")
	}
	if c.CloudMeta == nil {
		c.CloudMeta, err = cloud.NewMetadata(cloud.MetadataConfig{
			Clients:           c.CloudClients,
			AWSConfigProvider: c.AWSConfigProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.CloudIAM == nil {
		c.CloudIAM, err = cloud.NewIAM(ctx, cloud.IAMConfig{
			AccessPoint:       c.AccessPoint,
			AWSConfigProvider: c.AWSConfigProvider,
			Clients:           c.CloudClients,
			HostID:            c.HostID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.Limiter == nil {
		// Use default limiter if nothing is provided. Connection limiting will be disabled.
		c.Limiter, err = limiter.NewLimiter(limiter.Config{})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.ConnectedProxyGetter == nil {
		c.ConnectedProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}

	if c.CloudUsers == nil {
		clusterName, err := c.AuthClient.GetClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		c.CloudUsers, err = users.NewUsers(users.Config{
			AWSConfigProvider: c.AWSConfigProvider,
			Clients:           c.CloudClients,
			UpdateMeta:        c.CloudMeta.Update,
			ClusterName:       clusterName.GetClusterName(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.DatabaseObjects == nil {
		c.DatabaseObjects, err = objects.NewObjects(ctx, objects.Config{
			DatabaseObjectClient: c.AuthClient.DatabaseObjectsClient(),
			ImportRules:          c.AuthClient,
			Auth:                 c.Auth,
			CloudClients:         c.CloudClients,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.discoveryResourceChecker == nil {
		c.discoveryResourceChecker, err = cloud.NewDiscoveryResourceChecker(cloud.DiscoveryResourceCheckerConfig{
			ResourceMatchers:  c.ResourceMatchers,
			Clients:           c.CloudClients,
			AWSConfigProvider: c.AWSConfigProvider,
			Context:           ctx,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.getEngineFn == nil {
		c.getEngineFn = common.GetEngine
	}

	if c.ShutdownPollPeriod == 0 {
		c.ShutdownPollPeriod = defaults.ShutdownPollPeriod
	}

	return nil
}

// Server is a database server. It accepts database client requests coming over
// reverse tunnel from Teleport proxy and proxies them to databases.
type Server struct {
	// cfg is the database server configuration.
	cfg Config
	// closeContext is used to indicate the server is closing.
	closeContext context.Context
	// closeFunc is the cancel function of the close context.
	closeFunc context.CancelFunc
	// middleware extracts identity from client certificates.
	middleware *auth.Middleware
	// dynamicLabels contains dynamic labels for databases.
	dynamicLabels map[string]*labels.Dynamic
	// heartbeats holds heartbeats for database servers.
	heartbeats map[string]srv.HeartbeatI
	// watcher monitors changes to database resources.
	watcher *services.GenericWatcher[types.Database, readonly.Database]
	// proxiedDatabases contains databases this server currently is proxying.
	// Proxied databases are reconciled against monitoredDatabases below.
	proxiedDatabases map[string]types.Database
	// monitoredDatabases contains all cluster databases the proxied databases
	// are reconciled against.
	monitoredDatabases monitoredDatabases
	// reconcileCh triggers reconciliation of proxied databases.
	reconcileCh chan struct{}
	// mu protects access to server infos and databases.
	mu sync.RWMutex
	// logger is used for logging.
	log *slog.Logger
	// activeConnections counts the number of database active connections.
	activeConnections atomic.Int32
	// connContext context used by connection resources. Canceling will cause
	// active connections to drop.
	connContext context.Context
	// closeConnFunc is the cancel function of the connContext context.
	closeConnFunc context.CancelFunc
}

// monitoredDatabases is a collection of databases from different sources
// like configuration file, dynamic resources and imported from cloud.
//
// It's updated by respective watchers and is used for reconciling with the
// currently proxied databases.
type monitoredDatabases struct {
	// static are databases from the agent's YAML configuration.
	static types.Databases
	// resources are databases created via CLI, API, or discovery service.
	resources types.Databases
	// cloud are databases detected by cloud watchers.
	cloud types.Databases
	// mu protects access to the fields.
	mu sync.RWMutex
}

func (m *monitoredDatabases) setResources(databases types.Databases) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = databases
}

func (m *monitoredDatabases) setCloud(databases types.Databases) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cloud = databases
}

// isCloud_Locked returns whether a database was discovered by the cloud
// watchers, aka legacy database discovery done by the db service.
// The lock must be held when calling this function.
func (m *monitoredDatabases) isCloud_Locked(database types.Database) bool {
	for i := range m.cloud {
		if m.cloud[i] == database {
			return true
		}
	}
	return false
}

// isDiscoveryResource_Locked returns whether a database was discovered by the
// discovery service.
// The lock must be held when calling this function.
func (m *monitoredDatabases) isDiscoveryResource_Locked(database types.Database) bool {
	return database.Origin() == types.OriginCloud && m.isResource_Locked(database)
}

// isResource_Locked returns whether a database is a dynamic database, aka a db
// object.
// The lock must be held when calling this function.
func (m *monitoredDatabases) isResource_Locked(database types.Database) bool {
	for i := range m.resources {
		if m.resources[i] == database {
			return true
		}
	}
	return false
}

// getLocked returns a slice containing all of the monitored databases.
// The lock must be held when calling this function.
func (m *monitoredDatabases) getLocked() map[string]types.Database {
	return utils.FromSlice(append(append(m.static, m.resources...), m.cloud...), types.Database.GetName)
}

// New returns a new database server.
func New(ctx context.Context, config Config) (*Server, error) {
	if err := common.CheckEngines(defaults.DatabaseProtocols...); err != nil {
		return nil, trace.Wrap(err)
	}

	err := config.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clustername, err := config.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, closeCancelFunc := context.WithCancel(ctx)
	connCtx, connCancelFunc := context.WithCancel(ctx)
	server := &Server{
		cfg:              config,
		log:              slog.With(teleport.ComponentKey, teleport.ComponentDatabase),
		closeContext:     closeCtx,
		closeFunc:        closeCancelFunc,
		dynamicLabels:    make(map[string]*labels.Dynamic),
		heartbeats:       make(map[string]srv.HeartbeatI),
		proxiedDatabases: config.Databases.ToMap(),
		monitoredDatabases: monitoredDatabases{
			static: config.Databases,
		},
		reconcileCh: make(chan struct{}),
		middleware: &auth.Middleware{
			ClusterName:   clustername.GetClusterName(),
			AcceptedUsage: []string{teleport.UsageDatabaseOnly},
		},
		connContext:   connCtx,
		closeConnFunc: connCancelFunc,
	}

	// Update TLS config to require client certificate.
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		ctx,
		server.cfg.TLSConfig,
		server.cfg.AccessPoint,
		server.log,
		types.DatabaseCA,
	)

	return server, nil
}

// startDatabase performs initialization actions for the provided database
// such as starting dynamic labels and initializing CA certificate.
func (s *Server) startDatabase(ctx context.Context, database types.Database) error {
	// For cloud-hosted databases (RDS, Redshift, GCP), try to automatically
	// download a CA certificate.
	// TODO(r0mant): This should ideally become a part of cloud metadata service.
	if err := s.initCACert(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	// Update cloud metadata if it's a cloud hosted database on a best-effort
	// basis.
	if err := s.cfg.CloudMeta.Update(ctx, database); err != nil {
		s.log.WarnContext(ctx, "Failed to fetch cloud metadata.", "db", database.GetName(), "error", err)
	}
	// Attempts to fetch cloud metadata and configure IAM for cloud-hosted
	// databases on a best-effort basis.
	//
	// TODO(r0mant): It may also make sense to auto-configure IAM upon getting
	// access denied error at connection time in case this fails or the policy
	// gets removed off-band.
	if err := s.cfg.CloudIAM.Setup(ctx, database); err != nil {
		s.log.WarnContext(ctx, "Failed to auto-configure IAM.", "db", database.GetName(), "error", err)
	}
	// Start a goroutine that will be updating database's command labels (if any)
	// on the defined schedule.
	if err := s.startDynamicLabels(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	if err := fetchMySQLVersion(ctx, database); err != nil {
		// Log, but do not fail. We will fetch the version later.
		s.log.WarnContext(ctx, "Failed to fetch the MySQL version.", "db", database.GetName(), "error", err)
	}
	// Heartbeat will periodically report the presence of this proxied database
	// to the auth server.
	if err := s.startHeartbeat(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	// Setup managed users for database.
	if err := s.cfg.CloudUsers.Setup(ctx, database); err != nil {
		s.log.WarnContext(ctx, "Failed to setup users.", "database", database.GetName(), "error", err)
	}
	// Start database object importer.
	if err := s.cfg.DatabaseObjects.StartImporter(ctx, database); err != nil {
		// special handling for "not implemented" errors; these are very likely to occur and aren't as interesting.
		if trace.IsNotImplemented(err) {
			s.log.DebugContext(ctx, "Database object importer not implemented.", "database", database.GetName())
		} else if match, reason := objects.IsErrFetcherDisabled(err); match {
			s.log.DebugContext(ctx, "Database object importer cannot be started due to disabled fetcher", "reason", reason, "database", database.GetName())
		} else {
			s.log.WarnContext(ctx, "Failed to start database object importer.", "database", database.GetName(), "error", err)
		}
	}

	s.log.DebugContext(ctx, "Started database.", "db", database)
	return nil
}

// stopDatabase uninitializes the database with the specified name.
func (s *Server) stopDatabase(ctx context.Context, name string) error {
	// Stop database object importer.
	if err := s.cfg.DatabaseObjects.StopImporter(name); err != nil {
		s.log.WarnContext(ctx, "Failed to stop database object importer.", "db", name, "error", err)
	}
	s.stopDynamicLabels(name)
	if err := s.stopHeartbeat(name); err != nil {
		return trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "Stopped database.", "db", name)
	return nil
}

// deleteDatabaseServer deletes database server for the specified database.
func (s *Server) deleteDatabaseServer(ctx context.Context, name string) error {
	err := s.cfg.AuthClient.DeleteDatabaseServer(ctx, apidefaults.Namespace, s.cfg.HostID, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// startDynamicLabels starts dynamic labels for the database if it has them.
func (s *Server) startDynamicLabels(ctx context.Context, database types.Database) error {
	if len(database.GetDynamicLabels()) == 0 {
		return nil // Nothing to do.
	}
	dynamic, err := labels.NewDynamic(ctx, &labels.DynamicConfig{
		Labels: database.GetDynamicLabels(),
		Log:    s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	dynamic.Sync()
	dynamic.Start()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dynamicLabels[database.GetName()] = dynamic
	return nil
}

// getDynamicLabels returns dynamic labels for the specified database.
func (s *Server) getDynamicLabels(name string) *labels.Dynamic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dynamic, ok := s.dynamicLabels[name]
	if !ok {
		return nil
	}
	return dynamic
}

// stopDynamicLabels stops dynamic labels for the specified database.
func (s *Server) stopDynamicLabels(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dynamic, ok := s.dynamicLabels[name]
	if !ok {
		return
	}
	delete(s.dynamicLabels, name)
	dynamic.Close()
}

// registerDatabase initializes the provided database and adds it to the list
// of databases this server proxies.
func (s *Server) registerDatabase(ctx context.Context, database types.Database) error {
	if err := s.startDatabase(ctx, database); err != nil {
		// Cleanup in case database was initialized only partially.
		if errStop := s.stopDatabase(ctx, database.GetName()); errStop != nil {
			return trace.NewAggregate(err, errStop)
		}
		return trace.Wrap(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxiedDatabases[database.GetName()] = database
	return nil
}

// updateDatabase updates database that is already registered.
func (s *Server) updateDatabase(ctx context.Context, database types.Database) error {
	// Stop heartbeat and dynamic labels before starting new ones.
	if err := s.stopDatabase(ctx, database.GetName()); err != nil {
		return trace.Wrap(err)
	}
	if err := s.registerDatabase(ctx, database); err != nil {
		// If we failed to re-register, don't keep proxying the old database.
		if errUnregister := s.unregisterDatabase(ctx, database); errUnregister != nil {
			return trace.NewAggregate(err, errUnregister)
		}
		return trace.Wrap(err)
	}
	return nil
}

// unregisterDatabase uninitializes the specified database and removes it from
// the list of databases this server proxies.
func (s *Server) unregisterDatabase(ctx context.Context, database types.Database) error {
	// Deconfigure IAM for the cloud database.
	if err := s.cfg.CloudIAM.Teardown(ctx, database); err != nil {
		s.log.WarnContext(ctx, "Failed to teardown IAM.", "db", database.GetName(), "error", err)
	}
	// Stop heartbeat, labels, etc.
	if err := s.stopProxyingAndDeleteDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// stopProxyingAndDeleteDatabase stops and deletes the database, then
// unregisters it from the list of proxied databases.
func (s *Server) stopProxyingAndDeleteDatabase(ctx context.Context, database types.Database) error {
	// Stop heartbeat and dynamic labels updates.
	if err := s.stopDatabase(ctx, database.GetName()); err != nil {
		return trace.Wrap(err)
	}
	// Heartbeat is stopped but if we don't remove this database server,
	// it can linger for up to ~10m until its TTL expires.
	if err := s.deleteDatabaseServer(ctx, database.GetName()); err != nil {
		return trace.Wrap(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.proxiedDatabases, database.GetName())
	return nil
}

// getProxiedDatabases returns a list of all databases this server is proxying.
func (s *Server) getProxiedDatabases() (databases types.Databases) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, database := range s.proxiedDatabases {
		databases = append(databases, database)
	}
	return databases
}

// getProxiedDatabase returns a proxied database by name with updated dynamic
// and cloud labels.
func (s *Server) getProxiedDatabase(name string) (types.Database, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// don't call s.getProxiedDatabases() as this will call RLock and
	// potentially deadlock.
	db, found := s.proxiedDatabases[name]
	if !found {
		return nil, trace.NotFound("%q not found among registered databases: %v",
			name, s.proxiedDatabases)
	}
	return s.copyDatabaseWithUpdatedLabelsLocked(db), nil
}

// copyDatabaseWithUpdatedLabelsLocked will inject updated dynamic and cloud labels into
// a database object.
// The caller must invoke an RLock on `s.mu` before calling this function.
func (s *Server) copyDatabaseWithUpdatedLabelsLocked(database types.Database) *types.DatabaseV3 {
	// create a copy of the database to modify.
	copy := database.Copy()

	// Update dynamic labels if the database has them.
	labels, ok := s.dynamicLabels[copy.GetName()]
	if ok && labels != nil {
		copy.SetDynamicLabels(labels.Get())
	}

	// Add in the cloud labels if the db has them.
	if s.cfg.CloudLabels != nil {
		s.cfg.CloudLabels.Apply(copy)
	}
	return copy
}

// startHeartbeat starts the registration heartbeat to the auth server.
func (s *Server) startHeartbeat(ctx context.Context, database types.Database) error {
	heartbeat, err := srv.NewDatabaseServerHeartbeat(srv.HeartbeatV2Config[*types.DatabaseServerV3]{
		InventoryHandle: s.cfg.InventoryHandle,
		Announcer:       s.cfg.AccessPoint,
		GetResource:     s.getServerInfoFunc(database),
		OnHeartbeat:     s.cfg.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go heartbeat.Run()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats[database.GetName()] = heartbeat
	return nil
}

// stopHeartbeat stops the heartbeat for the specified database.
func (s *Server) stopHeartbeat(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	heartbeat, ok := s.heartbeats[name]
	if !ok {
		return nil
	}
	delete(s.heartbeats, name)
	return heartbeat.Close()
}

// getServerInfoFunc returns function that the heartbeater uses to report the
// provided database to the auth server.
//
// It can be overridden by GetServerInfoFn from config by tests.
func (s *Server) getServerInfoFunc(database types.Database) func(context.Context) (*types.DatabaseServerV3, error) {
	if s.cfg.GetServerInfoFn != nil {
		return s.cfg.GetServerInfoFn(database)
	}
	return func(ctx context.Context) (*types.DatabaseServerV3, error) {
		return s.getServerInfo(ctx, database)
	}
}

// getServerInfo returns up-to-date database resource e.g. with updated dynamic
// labels.
func (s *Server) getServerInfo(ctx context.Context, database types.Database) (*types.DatabaseServerV3, error) {
	// Make sure to return a new object, because it gets cached by
	// heartbeat and will always compare as equal otherwise.
	s.mu.RLock()
	copy := s.copyDatabaseWithUpdatedLabelsLocked(database)
	s.mu.RUnlock()
	if s.cfg.CloudIAM != nil {
		s.cfg.CloudIAM.UpdateIAMStatus(ctx, copy)
	}
	expires := s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)

	server, err := types.NewDatabaseServerV3(types.Metadata{
		Name:    copy.GetName(),
		Expires: &expires,
	}, types.DatabaseServerSpecV3{
		Version:  teleport.Version,
		Hostname: s.cfg.Hostname,
		HostID:   s.cfg.HostID,
		Rotation: s.getRotationState(),
		Database: copy,
		ProxyIDs: s.cfg.ConnectedProxyGetter.GetProxyIDs(),
	})
	return server, trace.Wrap(err)
}

// getRotationState is a helper to return this server's CA rotation state.
func (s *Server) getRotationState() types.Rotation {
	rotation, err := s.cfg.GetRotation(types.RoleDatabase)
	if err != nil && !trace.IsNotFound(err) {
		s.log.WarnContext(s.closeContext, "Failed to get rotation state.", "error", err)
	}
	if rotation != nil {
		return *rotation
	}
	return types.Rotation{}
}

// Start starts proxying all server's registered databases.
func (s *Server) Start(ctx context.Context) (err error) {
	// Start IAM service that will be configuring IAM auth for databases.
	if err := s.cfg.CloudIAM.Start(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Start cloud users that will be monitoring cloud users.
	go s.cfg.CloudUsers.Start(ctx, s.getProxiedDatabases)

	// Start heartbeating the Database Service itself.
	if err := s.startServiceHeartbeat(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Register all databases from static configuration.
	for _, database := range s.cfg.Databases {
		if err := s.registerDatabase(ctx, database); err != nil {
			return trace.Wrap(err)
		}
	}

	// Start reconciler that will be reconciling proxied databases with
	// database resources and cloud instances.
	if err := s.startReconciler(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Start watcher that will be dynamically (un-)registering
	// proxied databases based on the database resources.
	if s.watcher, err = s.startResourceWatcher(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Start watcher that will be monitoring cloud provider databases
	// according to the server's selectors.
	if err := s.startCloudWatcher(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Start the cloud-based databases CA renewer.
	go s.startCARenewer(ctx)

	return nil
}

// startServiceHeartbeat sends the current DatabaseService server info.
func (s *Server) startServiceHeartbeat(ctx context.Context) error {
	labels := make(map[string]string)
	if metadata.AWSOIDCDeployServiceInstallMethod() {
		labels[types.AWSOIDCAgentLabel] = types.True
	}

	getDatabaseServiceServerInfo := func() (types.Resource, error) {
		expires := s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)
		resource, err := types.NewDatabaseServiceV1(types.Metadata{
			Name:      s.cfg.HostID,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
			Labels:    labels,
		}, types.DatabaseServiceSpecV1{
			ResourceMatchers: services.ResourceMatchersToTypes(s.cfg.ResourceMatchers),
			Hostname:         s.cfg.Hostname,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return resource, nil
	}

	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeContext,
		Component:       teleport.ComponentDatabase,
		Mode:            srv.HeartbeatModeDatabaseService,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   getDatabaseServiceServerInfo,
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     s.cfg.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		if err := heartbeat.Run(); err != nil {
			s.log.ErrorContext(ctx, "Heartbeat ended with error.", "error", err)
		}
	}()
	return nil
}

// Close stops proxying all server's databases, drops active connections, and
// frees up other resources.
func (s *Server) Close() error {
	s.closeConnFunc()
	return trace.Wrap(s.close(s.closeContext))
}

// Shutdown performs a graceful shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.close(ctx)
	defer s.closeConnFunc()

	activeConnections := s.activeConnections.Load()
	if activeConnections == 0 {
		return trace.Wrap(err)
	}

	s.log.InfoContext(ctx, "Shutdown: waiting for active connections to finish.", "count", activeConnections)
	lastReport := time.Now()
	ticker := time.NewTicker(s.cfg.ShutdownPollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			activeConnections = s.activeConnections.Load()
			if activeConnections == 0 {
				return trace.Wrap(err)
			}

			if time.Since(lastReport) > 10*s.cfg.ShutdownPollPeriod {
				s.log.InfoContext(ctx, "Shutdown: waiting for active connections to finish.", "count", activeConnections)
				lastReport = time.Now()
			}
		case <-ctx.Done():
			s.log.InfoContext(ctx, "Context canceled wait, returning.")
			return trace.Wrap(err)
		}
	}
}

func (s *Server) close(ctx context.Context) error {
	shouldDeleteDBs := services.ShouldDeleteServerHeartbeatsOnShutdown(ctx)
	sender, ok := s.cfg.InventoryHandle.GetSender()
	if ok {
		// Manual deletion per database is only required if the auth server
		// doesn't support actively cleaning up database resources when the
		// inventory control stream is terminated during shutdown.
		if capabilities := sender.Hello().Capabilities; capabilities != nil {
			shouldDeleteDBs = shouldDeleteDBs && !capabilities.DatabaseCleanup
		}
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(100)

	// Hold the READ lock while iterating the databases here to prevent
	// deadlocking in flight heartbeats. The heartbeat announce acquires
	// the lock to build the db resource to send. If the WRITE lock is
	// held during the shutdown procedure below, any in flight heartbeats
	// will block acquiring the mutex until shutdown completes, at which
	// point the heartbeat will be emitted and the removal of the db
	// server below would be undone.
	s.mu.RLock()
	for name := range s.proxiedDatabases {
		name := name
		heartbeat := s.heartbeats[name]

		if dynamic, ok := s.dynamicLabels[name]; ok {
			dynamic.Close()
		}

		// Stop database object importer.
		if err := s.cfg.DatabaseObjects.StopImporter(name); err != nil {
			s.log.WarnContext(ctx, "Failed to stop database object importer.", "db", name, "error", err)
		}

		if heartbeat != nil {
			log := s.log.With("db", name)
			log.DebugContext(ctx, "Stopping db")
			if err := heartbeat.Close(); err != nil {
				log.WarnContext(ctx, "Failed to stop db.", "error", err)
			} else {
				log.DebugContext(ctx, "Stopped db")
			}

			if shouldDeleteDBs {
				g.Go(func() error {
					log.DebugContext(gctx, "Deleting db")
					if err := s.deleteDatabaseServer(gctx, name); err != nil {
						log.WarnContext(gctx, "Failed to delete db.", "error", err)
					} else {
						log.DebugContext(gctx, "Deleted db")
					}
					return nil
				})
			}
		}
	}
	s.mu.RUnlock()

	if err := g.Wait(); err != nil {
		s.log.WarnContext(ctx, "Deleting all databases failed", "error", err)
	}

	s.mu.Lock()
	clear(s.proxiedDatabases)
	clear(s.dynamicLabels)
	clear(s.heartbeats)
	s.mu.Unlock()

	// Signal to all goroutines to stop.
	s.closeFunc()

	// Stop the database resource watcher.
	if s.watcher != nil {
		s.watcher.Close()
	}

	// Close all cloud clients.
	return trace.Wrap(s.cfg.CloudClients.Close())
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	var errs []error
	for _, ctx := range []context.Context{s.closeContext, s.connContext} {
		<-ctx.Done()

		if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

// HandleConnection accepts the connection coming over reverse tunnel,
// upgrades it to TLS, extracts identity information from it, performs
// authorization and dispatches to the appropriate database engine.
func (s *Server) HandleConnection(conn net.Conn) {
	// Track active connections.
	s.activeConnections.Add(1)
	defer s.activeConnections.Add(-1)

	s.log.DebugContext(s.closeContext, "Accepted connection.", "addr", conn.RemoteAddr())
	// Upgrade the connection to TLS since the other side of the reverse
	// tunnel connection (proxy) will initiate a handshake.
	tlsConn := tls.Server(conn, s.cfg.TLSConfig)
	// Make sure to close the upgraded connection, not "conn", otherwise
	// the other side may not detect that connection has closed.
	defer tlsConn.Close()
	// Perform the handshake explicitly, normally it should be performed
	// on the first read/write but when the connection is passed over
	// reverse tunnel it doesn't happen for some reason.
	err := tlsConn.HandshakeContext(s.closeContext)
	if err != nil {
		s.log.ErrorContext(s.closeContext, "Failed to perform TLS handshake.", "error", err, "addr", conn.RemoteAddr())
		return
	}
	// Now that the handshake has completed and the client has sent us a
	// certificate, extract identity information from it.
	ctx, err := s.middleware.WrapContextWithUser(s.connContext, tlsConn)
	if err != nil {
		s.log.ErrorContext(s.closeContext, "Failed to extract identity from connection.", "error", err, "addr", conn.RemoteAddr())
		return
	}
	// Dispatch the connection for processing by an appropriate database
	// service.
	err = s.handleConnection(ctx, tlsConn)
	if err != nil && !utils.IsOKNetworkError(err) && !trace.IsAccessDenied(err) {
		s.log.ErrorContext(s.closeContext, "Failed to handle connection.", "error", err, "addr", conn.RemoteAddr())
		return
	}
}

func (s *Server) handleConnection(ctx context.Context, clientConn net.Conn) error {
	sessionCtx, err := s.authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create a session tracker so that other services, such as
	// the session upload completer, can track the session's lifetime.
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := s.trackSession(cancelCtx, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	rec, err := s.newSessionRecorder(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		// Close session stream in a goroutine since depending on session size
		// it can take a while, and we don't want to block the client.
		go func() {
			// Use the server closing context to make sure that upload
			// continues beyond the session lifetime.
			err := rec.Close(s.closeContext)
			if err != nil {
				sessionCtx.Log.WarnContext(ctx, "Failed to close stream writer.", "error", err)
			}
		}()
	}()

	// Wrap a client connection into monitor that auto-terminates
	// idle connection and connection with expired cert.
	ctx, clientConn, err = s.cfg.ConnectionMonitor.MonitorConn(cancelCtx, sessionCtx.AuthContext, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	engine, err := s.dispatch(sessionCtx, rec, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if r := recover(); r != nil {
			s.log.WarnContext(ctx, "Recovered while handling DB connection.", "from", clientConn.RemoteAddr(), "to", r)
			err = trace.BadParameter("failed to handle client connection")
		}
		if err != nil {
			engine.SendError(err)
		}
	}()

	// TODO(jakule): LoginIP should be required starting from 10.0.
	clientIP := sessionCtx.Identity.LoginIP
	if clientIP != "" {
		s.log.DebugContext(ctx, "Found real client IP.", "ip", clientIP)

		var release func()
		release, err = s.cfg.Limiter.RegisterRequestAndConnection(clientIP)
		if err != nil {
			return trace.Wrap(err)
		}
		defer release()
	} else {
		s.log.DebugContext(ctx, "LoginIP is not set (Proxy Service has to be updated). Rate limiting is disabled.")
	}

	// Update database roles. It needs to be done here after engine is
	// dispatched so the engine can propagate the error message to the client.
	if sessionCtx.AutoCreateUserMode.IsEnabled() {
		sessionCtx.DatabaseRoles, err = sessionCtx.Checker.CheckDatabaseRoles(sessionCtx.Database, sessionCtx.Identity.RouteToDatabase.Roles)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = engine.HandleConnection(ctx, sessionCtx)
	if err != nil {
		connectionDiagnosticID := sessionCtx.Identity.ConnectionDiagnosticID
		if connectionDiagnosticID != "" && trace.IsAccessDenied(err) {
			_, diagErr := s.cfg.AuthClient.AppendDiagnosticTrace(cancelCtx,
				connectionDiagnosticID,
				&types.ConnectionDiagnosticTrace{
					Type:    types.ConnectionDiagnosticTrace_RBAC_DATABASE_LOGIN,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "Access denied when accessing Database. Please check the Error message for more information.",
					Error:   err.Error(),
				},
			)

			if diagErr != nil {
				return trace.Wrap(diagErr)
			}
		}

		return trace.Wrap(err)
	}
	return nil
}

// dispatch creates and initializes an appropriate database engine for the session.
func (s *Server) dispatch(sessionCtx *common.Session, rec events.SessionPreparerRecorder, clientConn net.Conn) (common.Engine, error) {
	audit, err := s.cfg.NewAudit(common.AuditConfig{
		Emitter:  s.cfg.Emitter,
		Recorder: rec,
		Database: sessionCtx.Database,
		Clock:    s.cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	engine, err := s.createEngine(sessionCtx, audit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := engine.InitializeConnection(clientConn, sessionCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	return engine, nil
}

// createEngine creates a new database engine based on the database protocol.
// An error is returned when a protocol is not supported.
func (s *Server) createEngine(sessionCtx *common.Session, audit common.Audit) (common.Engine, error) {
	return s.cfg.getEngineFn(sessionCtx.Database, common.EngineConfig{
		Auth:              common.NewAuthForSession(s.cfg.Auth, sessionCtx),
		Audit:             audit,
		AuthClient:        s.cfg.AuthClient,
		AWSConfigProvider: s.cfg.AWSConfigProvider,
		CloudClients:      s.cfg.CloudClients,
		Context:           s.connContext,
		Clock:             s.cfg.Clock,
		Log:               sessionCtx.Log,
		Users:             s.cfg.CloudUsers,
		DataDir:           s.cfg.DataDir,
		GetUserProvisioner: func(aub common.AutoUsers) *common.UserProvisioner {
			return &common.UserProvisioner{
				AuthClient: s.cfg.AuthClient,
				Backend:    aub,
				Log:        sessionCtx.Log,
				Clock:      s.cfg.Clock,
			}
		},
		UpdateProxiedDatabase: func(name string, doUpdate func(types.Database) error) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			db, found := s.proxiedDatabases[name]
			if !found {
				return trace.NotFound("%q not found among registered databases", name)
			}
			return trace.Wrap(doUpdate(db))
		},
	})
}

func (s *Server) authorize(ctx context.Context) (*common.Session, error) {
	// Only allow local and remote identities to proxy to a database.
	userType, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch userType.(type) {
	case authz.LocalUser, authz.RemoteUser:
	default:
		return nil, trace.BadParameter("invalid identity: %T", userType)
	}
	// Extract authorizing context and identity of the user from the request.
	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()
	s.log.DebugContext(ctx, "Client identity authorized.", "identity", identity)

	// Fetch the requested database server.
	database, err := s.getProxiedDatabase(identity.RouteToDatabase.ServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	autoCreate, err := authContext.Checker.DatabaseAutoUserMode(database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.log.DebugContext(ctx, "Will connect to database.", "db", database.GetName(), "uri", database.GetURI())

	id := uuid.New().String()
	sessionCtx := &common.Session{
		ID:                 id,
		ClusterName:        identity.RouteToCluster,
		HostID:             s.cfg.HostID,
		Database:           database,
		Identity:           identity,
		AutoCreateUserMode: autoCreate,
		DatabaseUser:       identity.RouteToDatabase.Username,
		DatabaseName:       identity.RouteToDatabase.Database,
		AuthContext:        authContext,
		Checker:            authContext.Checker,
		StartupParameters:  make(map[string]string),
		Log:                s.log.With("id", id, "db", database.GetName()),
		LockTargets:        authContext.LockTargets(),
		StartTime:          s.cfg.Clock.Now(),
	}

	s.log.DebugContext(ctx, "Created session context.", "session", sessionCtx)
	return sessionCtx, nil
}

// fetchMySQLVersion tries to connect to MySQL instance, read initial handshake package and extract
// the server version.
func fetchMySQLVersion(ctx context.Context, database types.Database) error {
	if database.GetProtocol() != defaults.ProtocolMySQL || database.GetMySQLServerVersion() != "" {
		return nil
	}

	// Try to extract the engine version for AWS metadata labels.
	if database.IsRDS() || database.IsAzure() {
		version := discoverycommon.GetMySQLEngineVersion(database.GetMetadata().Labels)
		if version != "" {
			database.SetMySQLServerVersion(version)
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

	version, err := mysql.FetchMySQLVersion(ctx, database)
	if err != nil {
		return trace.Wrap(err)
	}

	database.SetMySQLServerVersion(version)

	return nil
}

// trackSession creates a new session tracker for the database session.
// While ctx is open, the session tracker's expiration will be extended
// on an interval. Once the ctx is closed, the session tracker's state
// will be updated to terminated.
func (s *Server) trackSession(ctx context.Context, sessionCtx *common.Session) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:    sessionCtx.ID,
		Kind:         string(types.DatabaseSessionKind),
		State:        types.SessionState_SessionStateRunning,
		Hostname:     sessionCtx.HostID,
		DatabaseName: sessionCtx.Database.GetName(),
		ClusterName:  sessionCtx.ClusterName,
		Login:        sessionCtx.Identity.GetUserMetadata().Login,
		Participants: []types.Participant{{
			User: sessionCtx.Identity.Username,
		}},
		HostUser: sessionCtx.Identity.Username,
		Created:  s.cfg.Clock.Now(),
		HostID:   sessionCtx.HostID,
	}

	s.log.DebugContext(ctx, "Creating session tracker.", "session", sessionCtx.ID)
	tracker, err := srv.NewSessionTracker(ctx, trackerSpec, s.cfg.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		if err := tracker.UpdateExpirationLoop(ctx, s.cfg.Clock); err != nil {
			s.log.WarnContext(ctx, "Failed to update session tracker expiration.", "session", sessionCtx.ID, "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := tracker.Close(s.connContext); err != nil {
			s.log.DebugContext(ctx, "Failed to close session tracker.", "session", sessionCtx.ID, "error", err)
		}
	}()

	return nil
}
