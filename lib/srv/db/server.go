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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/utils"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

// Config is the configuration for an database proxy server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock
	// DataDir is the path to the data directory for the server.
	DataDir string
	// AuthClient is a client directly connected to the Auth server.
	AuthClient *client.Client
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint auth.ClientAccessPoint
	// StreamEmitter is a non-blocking audit events emitter.
	StreamEmitter events.StreamEmitter
	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config
	// Authorizer is used to authorize requests coming from proxy.
	Authorizer server.Authorizer
	// GetRotation returns the certificate rotation state.
	GetRotation func(role teleport.Role) (*services.Rotation, error)
	// Servers contains a list of database servers this service proxies.
	Servers types.DatabaseServers
	// AWSCredentials are credentials to AWS API.
	AWSCredentials *credentials.Credentials
	// GCPIAM is the GCP IAM client.
	GCPIAM *gcpcredentials.IamCredentialsClient
	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)
}

// CheckAndSetDefaults makes sure the configuration has the minimum required
// to function.
func (c *Config) CheckAndSetDefaults(ctx context.Context) error {
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
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLSConfig")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.GetRotation == nil {
		return trace.BadParameter("missing GetRotation")
	}
	if len(c.Servers) == 0 {
		return trace.BadParameter("missing Servers")
	}
	// Only initialize AWS session if this service is proxying any RDS databases.
	if c.AWSCredentials == nil && c.Servers.HasRDS() {
		session, err := awssession.NewSessionWithOptions(awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		c.AWSCredentials = session.Config.Credentials
	}
	// Only initialize GCP IAM client if this service is proxying any Cloud SQL databases.
	if c.GCPIAM == nil && c.Servers.HasGCP() {
		iamClient, err := gcpcredentials.NewIamCredentialsClient(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		c.GCPIAM = iamClient
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
	middleware *server.Middleware
	// dynamicLabels contains dynamic labels for database servers.
	dynamicLabels map[string]*labels.Dynamic
	// heartbeats holds hearbeats for database servers.
	heartbeats map[string]*srv.Heartbeat
	// rdsCACerts contains loaded RDS root certificates for required regions.
	rdsCACerts map[string][]byte
	// mu protects access to server infos.
	mu sync.RWMutex
	// log is used for logging.
	log *logrus.Entry
}

// New returns a new database server.
func New(ctx context.Context, config Config) (*Server, error) {
	err := config.CheckAndSetDefaults(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	server := &Server{
		cfg:           config,
		log:           logrus.WithField(trace.Component, teleport.ComponentDatabase),
		closeContext:  ctx,
		closeFunc:     cancel,
		dynamicLabels: make(map[string]*labels.Dynamic),
		heartbeats:    make(map[string]*srv.Heartbeat),
		rdsCACerts:    make(map[string][]byte),
		middleware: &server.Middleware{
			AccessPoint:   config.AccessPoint,
			AcceptedUsage: []string{teleport.UsageDatabaseOnly},
		},
	}

	// Update TLS config to require client certificate.
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		server.cfg.TLSConfig, server.cfg.AccessPoint, server.log)

	// Perform various initialization actions on each proxied database, like
	// starting up dynamic labels and loading root certs for RDS dbs.
	for _, db := range server.cfg.Servers {
		if err := server.initDatabaseServer(ctx, db); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return server, nil
}

func (s *Server) initDatabaseServer(ctx context.Context, server types.DatabaseServer) error {
	if err := s.initDynamicLabels(ctx, server); err != nil {
		return trace.Wrap(err)
	}
	if err := s.initHeartbeat(ctx, server); err != nil {
		return trace.Wrap(err)
	}
	if err := s.initRDSRootCert(ctx, server); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) initDynamicLabels(ctx context.Context, server types.DatabaseServer) error {
	if len(server.GetDynamicLabels()) == 0 {
		return nil // Nothing to do.
	}
	dynamic, err := labels.NewDynamic(ctx, &labels.DynamicConfig{
		Labels: server.GetDynamicLabels(),
		Log:    s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	dynamic.Sync()
	s.dynamicLabels[server.GetName()] = dynamic
	return nil
}

func (s *Server) initHeartbeat(ctx context.Context, server types.DatabaseServer) error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeContext,
		Component:       teleport.ComponentDatabase,
		Mode:            srv.HeartbeatModeDB,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.getServerInfoFunc(server),
		KeepAlivePeriod: defaults.ServerKeepAliveTTL,
		AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       defaults.ServerAnnounceTTL,
		OnHeartbeat:     s.cfg.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	s.heartbeats[server.GetName()] = heartbeat
	return nil
}

func (s *Server) getServerInfoFunc(server types.DatabaseServer) func() (services.Resource, error) {
	return func() (services.Resource, error) {
		// Make sure to return a new object, because it gets cached by
		// heartbeat and will always compare as equal otherwise.
		s.mu.RLock()
		server = server.Copy()
		s.mu.RUnlock()
		// Update dynamic labels.
		labels, ok := s.dynamicLabels[server.GetName()]
		if ok {
			server.SetDynamicLabels(labels.Get())
		}
		// Update CA rotation state.
		rotation, err := s.cfg.GetRotation(teleport.RoleDatabase)
		if err != nil && !trace.IsNotFound(err) {
			s.log.WithError(err).Warn("Failed to get rotation state.")
		} else {
			if rotation != nil {
				server.SetRotation(*rotation)
			}
		}
		// Update TTL.
		server.SetExpiry(s.cfg.Clock.Now().UTC().Add(defaults.ServerAnnounceTTL))
		return server, nil
	}
}

// Start starts heartbeating the presence of service.Databases that this
// server is proxying along with any dynamic labels.
func (s *Server) Start() error {
	for _, dynamicLabel := range s.dynamicLabels {
		go dynamicLabel.Start()
	}
	for _, heartbeat := range s.heartbeats {
		go heartbeat.Run()
	}
	return nil
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	// Stop dynamic label updates.
	for _, dynamicLabel := range s.dynamicLabels {
		dynamicLabel.Close()
	}
	// Signal to all goroutines to stop.
	s.closeFunc()
	// Stop the heartbeats.
	var errors []error
	for _, heartbeat := range s.heartbeats {
		errors = append(errors, heartbeat.Close())
	}
	// Close the GCP IAM client if needed.
	if s.cfg.GCPIAM != nil {
		errors = append(errors, s.cfg.GCPIAM.Close())
	}
	return trace.NewAggregate(errors...)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	return s.closeContext.Err()
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
	// Perform the hanshake explicitly, normally it should be performed
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
	if err != nil {
		log.WithError(err).Error("Failed to handle connection.")
		return
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) error {
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
		// session size it can take a while and we don't want to block
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
	engine, err := s.dispatch(sessionCtx, streamWriter)
	if err != nil {
		return trace.Wrap(err)
	}
	err = engine.HandleConnection(ctx, sessionCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// dispatch returns an appropriate database engine for the session.
func (s *Server) dispatch(sessionCtx *common.Session, streamWriter events.StreamWriter) (common.Engine, error) {
	auth, err := common.NewAuth(common.AuthConfig{
		AuthClient:     s.cfg.AuthClient,
		AWSCredentials: s.cfg.AWSCredentials,
		GCPIAM:         s.cfg.GCPIAM,
		RDSCACerts:     s.rdsCACerts,
		Clock:          s.cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audit, err := common.NewAudit(common.AuditConfig{
		StreamWriter: streamWriter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch sessionCtx.Server.GetProtocol() {
	case defaults.ProtocolPostgres:
		return &postgres.Engine{
			Auth:    auth,
			Audit:   audit,
			Context: s.closeContext,
			Clock:   s.cfg.Clock,
			Log:     sessionCtx.Log,
		}, nil
	case defaults.ProtocolMySQL:
		return &mysql.Engine{
			Auth:    auth,
			Audit:   audit,
			Context: s.closeContext,
			Clock:   s.cfg.Clock,
			Log:     sessionCtx.Log,
		}, nil
	}
	return nil, trace.BadParameter("unsupported database protocol %q",
		sessionCtx.Server.GetProtocol())
}

func (s *Server) authorize(ctx context.Context) (*common.Session, error) {
	// Only allow local and remote identities to proxy to a database.
	userType := ctx.Value(server.ContextUser)
	switch userType.(type) {
	case server.LocalUser, server.RemoteUser:
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
	var server types.DatabaseServer
	for _, s := range s.cfg.Servers {
		if s.GetName() == identity.RouteToDatabase.ServiceName {
			server = s
		}
	}
	if server == nil {
		return nil, trace.NotFound("%q not found among registered database servers: %v",
			identity.RouteToDatabase.ServiceName, s.cfg.Servers)
	}
	s.log.Debugf("Will connect to database %q at %v.", server.GetName(),
		server.GetURI())
	id := uuid.New()
	return &common.Session{
		ID:                id,
		ClusterName:       identity.RouteToCluster,
		Server:            server,
		Identity:          identity,
		DatabaseUser:      identity.RouteToDatabase.Username,
		DatabaseName:      identity.RouteToDatabase.Database,
		Checker:           authContext.Checker,
		StartupParameters: make(map[string]string),
		Log: s.log.WithFields(logrus.Fields{
			"id": id,
			"db": server.GetName(),
		}),
	}, nil
}
