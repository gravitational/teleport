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

package db

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"

	// Import to register MongoDB engine.
	_ "github.com/gravitational/teleport/lib/srv/db/mongodb"
	// Import to register MySQL engine.
	_ "github.com/gravitational/teleport/lib/srv/db/mysql"
	// Import to register Postgres engine.
	_ "github.com/gravitational/teleport/lib/srv/db/postgres"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Config is the configuration for an database proxy server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock
	// DataDir is the path to the data directory for the server.
	DataDir string
	// AuthClient is a client directly connected to the Auth server.
	AuthClient *auth.Client
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint auth.DatabaseAccessPoint
	// StreamEmitter is a non-blocking audit events emitter.
	StreamEmitter events.StreamEmitter
	// NewAudit allows to override audit logger in tests.
	NewAudit NewAuditFn
	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config
	// Limiter limits the number of connections per client IP.
	Limiter *limiter.Limiter
	// Authorizer is used to authorize requests coming from proxy.
	Authorizer auth.Authorizer
	// GetRotation returns the certificate rotation state.
	GetRotation func(role types.SystemRole) (*types.Rotation, error)
	// GetServerInfoFn returns function that returns database info for heartbeats.
	GetServerInfoFn func(database types.Database) func() (types.Resource, error)
	// Hostname is the hostname where this database server is running.
	Hostname string
	// HostID is the id of the host where this database server is running.
	HostID string
	// ResourceMatchers is a list of database resource matchers.
	ResourceMatchers []services.ResourceMatcher
	// AWSMatchers is a list of AWS databases matchers.
	AWSMatchers []services.AWSMatcher
	// Databases is a list of proxied databases from static configuration.
	Databases types.Databases
	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)
	// OnReconcile is called after each database resource reconciliation.
	OnReconcile func(types.Databases)
	// Auth is responsible for generating database auth tokens.
	Auth common.Auth
	// CADownloader automatically downloads root certs for cloud hosted databases.
	CADownloader CADownloader
	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
	// CloudClients creates cloud API clients.
	CloudClients common.CloudClients
	// CloudMeta fetches cloud metadata for cloud hosted databases.
	CloudMeta *cloud.Metadata
	// CloudIAM configures IAM for cloud hosted databases.
	CloudIAM *cloud.IAM
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
	if c.StreamEmitter == nil {
		return trace.BadParameter("missing StreamEmitter")
	}
	if c.NewAudit == nil {
		c.NewAudit = common.NewAudit
	}
	if c.Auth == nil {
		c.Auth, err = common.NewAuth(common.AuthConfig{
			AuthClient: c.AuthClient,
			Clock:      c.Clock,
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
		c.CADownloader = NewRealDownloader(c.DataDir)
	}
	if c.LockWatcher == nil {
		return trace.BadParameter("missing LockWatcher")
	}
	if c.CloudClients == nil {
		c.CloudClients = common.NewCloudClients()
	}
	if c.CloudMeta == nil {
		c.CloudMeta, err = cloud.NewMetadata(cloud.MetadataConfig{
			Clients: c.CloudClients,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.CloudIAM == nil {
		c.CloudIAM, err = cloud.NewIAM(ctx, cloud.IAMConfig{
			Clients: c.CloudClients,
			HostID:  c.HostID,
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
	heartbeats map[string]*srv.Heartbeat
	// watcher monitors changes to database resources.
	watcher *services.DatabaseWatcher
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
	// log is used for logging.
	log *logrus.Entry
}

// monitoredDatabases is a collection of databases from different sources
// like configuration file, dynamic resources and imported from cloud.
//
// It's updated by respective watchers and is used for reconciling with the
// currently proxied databases.
type monitoredDatabases struct {
	// static are databases from the agent's YAML configuration.
	static types.Databases
	// resources are databases created via CLI or API.
	resources types.Databases
	// cloud are databases detected by cloud watchers.
	cloud types.Databases
	// mu protects access to the fields.
	mu sync.Mutex
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

func (m *monitoredDatabases) get() types.ResourcesWithLabels {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append(append(m.static, m.resources...), m.cloud...).AsResources()
}

// New returns a new database server.
func New(ctx context.Context, config Config) (*Server, error) {
	err := config.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		cfg:              config,
		log:              logrus.WithField(trace.Component, teleport.ComponentDatabase),
		closeContext:     ctx,
		closeFunc:        cancel,
		dynamicLabels:    make(map[string]*labels.Dynamic),
		heartbeats:       make(map[string]*srv.Heartbeat),
		proxiedDatabases: config.Databases.ToMap(),
		monitoredDatabases: monitoredDatabases{
			static: config.Databases,
		},
		reconcileCh: make(chan struct{}),
		middleware: &auth.Middleware{
			AccessPoint:   config.AccessPoint,
			AcceptedUsage: []string{teleport.UsageDatabaseOnly},
		},
	}

	// Update TLS config to require client certificate.
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		server.cfg.TLSConfig, server.cfg.AccessPoint, server.log)

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
		s.log.Warnf("Failed to fetch cloud metadata for %v: %v.", database, err)
	}
	// Attempts to fetch cloud metadata and configure IAM for cloud-hosted
	// databases on a best-effort basis.
	//
	// TODO(r0mant): It may also make sense to auto-configure IAM upon getting
	// access denied error at connection time in case this fails or the policy
	// gets removed off-band.
	if err := s.cfg.CloudIAM.Setup(ctx, database); err != nil {
		s.log.Warnf("Failed to auto-configure IAM for %v: %v.", database, err)
	}
	// Start a goroutine that will be updating database's command labels (if any)
	// on the defined schedule.
	if err := s.startDynamicLabels(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	// Heartbeat will periodically report the presence of this proxied database
	// to the auth server.
	if err := s.startHeartbeat(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	s.log.Debugf("Started %v.", database)
	return nil
}

// stopDatabase uninitializes the database with the specified name.
func (s *Server) stopDatabase(ctx context.Context, name string) error {
	s.stopDynamicLabels(name)
	if err := s.stopHeartbeat(name); err != nil {
		return trace.Wrap(err)
	}
	s.log.Debugf("Stopped database %q.", name)
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
		s.log.Warnf("Failed to teardown IAM for %v: %v.", database, err)
	}
	// Stop heartbeat, labels, etc.
	if err := s.stopProxyingDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// stopProxyingDatabase winds down the proxied database instance by stopping
// its heartbeat and dynamic labels and unregistering it from the list of
// proxied databases.
func (s *Server) stopProxyingDatabase(ctx context.Context, database types.Database) error {
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

// startHeartbeat starts the registration heartbeat to the auth server.
func (s *Server) startHeartbeat(ctx context.Context, database types.Database) error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeContext,
		Component:       teleport.ComponentDatabase,
		Mode:            srv.HeartbeatModeDB,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.getServerInfoFunc(database),
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
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
func (s *Server) getServerInfoFunc(database types.Database) func() (types.Resource, error) {
	if s.cfg.GetServerInfoFn != nil {
		return s.cfg.GetServerInfoFn(database)
	}
	return func() (types.Resource, error) {
		return s.getServerInfo(database)
	}
}

// getServerInfo returns up-to-date database resource e.g. with updated dynamic
// labels.
func (s *Server) getServerInfo(database types.Database) (types.Resource, error) {
	// Make sure to return a new object, because it gets cached by
	// heartbeat and will always compare as equal otherwise.
	s.mu.RLock()
	copy := database.Copy()
	s.mu.RUnlock()
	// Update dynamic labels if the database has them.
	labels := s.getDynamicLabels(copy.GetName())
	if labels != nil {
		copy.SetDynamicLabels(labels.Get())
	}
	expires := s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)
	return types.NewDatabaseServerV3(types.Metadata{
		Name:    copy.GetName(),
		Expires: &expires,
	}, types.DatabaseServerSpecV3{
		Version:  teleport.Version,
		Hostname: s.cfg.Hostname,
		HostID:   s.cfg.HostID,
		Rotation: s.getRotationState(),
		Database: copy,
	})
}

// getRotationState is a helper to return this server's CA rotation state.
func (s *Server) getRotationState() types.Rotation {
	rotation, err := s.cfg.GetRotation(types.RoleDatabase)
	if err != nil && !trace.IsNotFound(err) {
		s.log.WithError(err).Warn("Failed to get rotation state.")
	}
	if rotation != nil {
		return *rotation
	}
	return types.Rotation{}
}

// Start starts proxying all server's registered databases.
func (s *Server) Start(ctx context.Context) (err error) {
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

	return nil
}

// Close stops proxying all server's databases and frees up other resources.
func (s *Server) Close() error {
	var errors []error
	// Stop proxying all databases.
	for _, database := range s.getProxiedDatabases() {
		if err := s.stopProxyingDatabase(s.closeContext, database); err != nil {
			errors = append(errors, trace.WrapWithMessage(
				err, "stopping database %v", database.GetName()))
		}
	}
	// Signal to all goroutines to stop.
	s.closeFunc()
	// Stop the database resource watcher.
	if s.watcher != nil {
		s.watcher.Close()
	}
	// Close all cloud clients.
	errors = append(errors, s.cfg.Auth.Close())
	return trace.NewAggregate(errors...)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	if err := s.closeContext.Err(); err != nil && err != context.Canceled {
		return trace.Wrap(err)
	}
	return nil
}

// ForceHeartbeat is used by tests to force-heartbeat all registered databases.
func (s *Server) ForceHeartbeat() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name, heartbeat := range s.heartbeats {
		s.log.Debugf("Forcing heartbeat for %q.", name)
		if err := heartbeat.ForceSend(time.Second); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// HandleConnection accepts the connection coming over reverse tunnel,
// upgrades it to TLS, extracts identity information from it, performs
// authorization and dispatches to the appropriate database engine.
func (s *Server) HandleConnection(conn net.Conn) {
	log := s.log.WithField("addr", conn.RemoteAddr())
	log.Debug("Accepted connection.")
	// Upgrade the connection to TLS since the other side of the reverse
	// tunnel connection (proxy) will initiate a handshake.
	tlsConn := tls.Server(conn, s.cfg.TLSConfig)
	// Make sure to close the upgraded connection, not "conn", otherwise
	// the other side may not detect that connection has closed.
	defer tlsConn.Close()
	// Perform the handshake explicitly, normally it should be performed
	// on the first read/write but when the connection is passed over
	// reverse tunnel it doesn't happen for some reason.
	err := tlsConn.Handshake()
	if err != nil {
		log.WithError(err).Error("Failed to perform TLS handshake.")
		return
	}
	// Now that the handshake has completed and the client has sent us a
	// certificate, extract identity information from it.
	ctx, err := s.middleware.WrapContextWithUser(s.closeContext, tlsConn)
	if err != nil {
		log.WithError(err).Error("Failed to extract identity from connection.")
		return
	}
	// Dispatch the connection for processing by an appropriate database
	// service.
	err = s.handleConnection(ctx, tlsConn)
	if err != nil && !utils.IsOKNetworkError(err) && !trace.IsAccessDenied(err) {
		log.WithError(err).Error("Failed to handle connection.")
		return
	}
}

func (s *Server) handleConnection(ctx context.Context, clientConn net.Conn) error {
	sessionCtx, err := s.authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	streamWriter, err := s.newStreamWriter(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		// Closing the stream writer is needed to flush all recorded data
		// and trigger upload. Do it in a goroutine since depending on
		// session size it can take a while, and we don't want to block
		// the client.
		go func() {
			// Use the server closing context to make sure that upload
			// continues beyond the session lifetime.
			err := streamWriter.Close(s.closeContext)
			if err != nil {
				sessionCtx.Log.WithError(err).Warn("Failed to close stream writer.")
			}
		}()
	}()
	engine, err := s.dispatch(sessionCtx, streamWriter, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if r := recover(); r != nil {
			s.log.Warnf("Recovered while handling DB connection from %v: %v.", clientConn.RemoteAddr(), r)
			err = trace.BadParameter("failed to handle client connection")
		}
		if err != nil {
			engine.SendError(err)
		}
	}()

	// Wrap a client connection into monitor that auto-terminates
	// idle connection and connection with expired cert.
	clientConn, err = monitorConn(ctx, monitorConnConfig{
		conn:         clientConn,
		lockWatcher:  s.cfg.LockWatcher,
		lockTargets:  sessionCtx.LockTargets,
		identity:     sessionCtx.Identity,
		checker:      sessionCtx.Checker,
		clock:        s.cfg.Clock,
		serverID:     s.cfg.HostID,
		authClient:   s.cfg.AuthClient,
		teleportUser: sessionCtx.Identity.Username,
		emitter:      s.cfg.AuthClient,
		log:          s.log,
		ctx:          s.closeContext,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(jakule): ClientIP should be required starting from 10.0.
	clientIP := sessionCtx.Identity.ClientIP
	if clientIP != "" {
		s.log.Debugf("Real client IP %s", clientIP)

		var release func()
		release, err = s.cfg.Limiter.RegisterRequestAndConnection(clientIP)
		if err != nil {
			return trace.Wrap(err)
		}
		defer release()
	} else {
		s.log.Debug("ClientIP is not set (Proxy Service has to be updated). Rate limiting is disabled.")
	}

	err = engine.HandleConnection(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// dispatch creates and initializes an appropriate database engine for the session.
func (s *Server) dispatch(sessionCtx *common.Session, streamWriter events.StreamWriter, clientConn net.Conn) (common.Engine, error) {
	audit, err := s.cfg.NewAudit(common.AuditConfig{
		Emitter: streamWriter,
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
	return common.GetEngine(sessionCtx.Database.GetProtocol(), common.EngineConfig{
		Auth:         s.cfg.Auth,
		Audit:        audit,
		AuthClient:   s.cfg.AuthClient,
		CloudClients: s.cfg.CloudClients,
		Context:      s.closeContext,
		Clock:        s.cfg.Clock,
		Log:          sessionCtx.Log,
	})
}

func (s *Server) authorize(ctx context.Context) (*common.Session, error) {
	// Only allow local and remote identities to proxy to a database.
	userType := ctx.Value(auth.ContextUser)
	switch userType.(type) {
	case auth.LocalUser, auth.RemoteUser:
	default:
		return nil, trace.BadParameter("invalid identity: %T", userType)
	}
	// Extract authorizing context and identity of the user from the request.
	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()
	s.log.Debugf("Client identity: %#v.", identity)
	// Fetch the requested database server.
	var database types.Database
	registeredDatabases := s.getProxiedDatabases()
	for _, db := range registeredDatabases {
		if db.GetName() == identity.RouteToDatabase.ServiceName {
			database = db
			break
		}
	}
	if database == nil {
		return nil, trace.NotFound("%q not found among registered databases: %v",
			identity.RouteToDatabase.ServiceName, registeredDatabases)
	}
	s.log.Debugf("Will connect to database %q at %v.", database.GetName(),
		database.GetURI())
	id := uuid.New().String()
	return &common.Session{
		ID:                id,
		ClusterName:       identity.RouteToCluster,
		HostID:            s.cfg.HostID,
		Database:          database,
		Identity:          identity,
		DatabaseUser:      identity.RouteToDatabase.Username,
		DatabaseName:      identity.RouteToDatabase.Database,
		Checker:           authContext.Checker,
		StartupParameters: make(map[string]string),
		Log: s.log.WithFields(logrus.Fields{
			"id": id,
			"db": database.GetName(),
		}),
		LockTargets: authContext.LockTargets(),
	}, nil
}
