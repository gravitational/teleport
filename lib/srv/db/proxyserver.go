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
	"crypto/x509"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// ProxyServer runs inside Teleport proxy and is responsible to accepting
// connections coming from the database clients (via a multiplexer) and
// dispatching them to appropriate database services over reverse tunnel.
type ProxyServer struct {
	// cfg is the proxy server configuration.
	cfg ProxyServerConfig
	// middleware extracts identity information from client certificates.
	middleware *auth.Middleware
	// closeCtx is closed when the process shuts down.
	closeCtx context.Context
	// log is used for logging.
	log logrus.FieldLogger
}

// ProxyServerConfig is the proxy configuration.
type ProxyServerConfig struct {
	// AuthClient is the authenticated client to the auth server.
	AuthClient *auth.Client
	// AccessPoint is the caching client connected to the auth server.
	AccessPoint auth.ReadDatabaseAccessPoint
	// Authorizer is responsible for authorizing user identities.
	Authorizer auth.Authorizer
	// Tunnel is the reverse tunnel server.
	Tunnel reversetunnel.Server
	// TLSConfig is the proxy server TLS configuration.
	TLSConfig *tls.Config
	// Limiter is the connection/rate limiter.
	Limiter *limiter.Limiter
	// Emitter is used to emit audit events.
	Emitter events.Emitter
	// Clock to override clock in tests.
	Clock clockwork.Clock
	// ServerID is the ID of the audit log server.
	ServerID string
	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
}

// ShuffleFunc defines a function that shuffles a list of database servers.
type ShuffleFunc func([]types.DatabaseServer) []types.DatabaseServer

// ShuffleRandom is a ShuffleFunc that randomizes the order of database servers.
// Used to provide load balancing behavior when proxying to multiple agents.
func ShuffleRandom(servers []types.DatabaseServer) []types.DatabaseServer {
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(
		len(servers), func(i, j int) {
			servers[i], servers[j] = servers[j], servers[i]
		})
	return servers
}

// ShuffleSort is a ShuffleFunc that sorts database servers by name and host ID.
// Used to provide predictable behavior in tests.
func ShuffleSort(servers []types.DatabaseServer) []types.DatabaseServer {
	sort.Sort(types.DatabaseServers(servers))
	return servers
}

var (
	// mu protects the shuffleFunc global access.
	mu sync.RWMutex
	// shuffleFunc provides shuffle behavior for multiple database agents.
	shuffleFunc ShuffleFunc = ShuffleRandom
)

// SetShuffleFunc sets the shuffle behavior when proxying to multiple agents.
func SetShuffleFunc(fn ShuffleFunc) {
	mu.Lock()
	defer mu.Unlock()
	shuffleFunc = fn
}

// getShuffleFunc returns the configured function used to shuffle agents.
func getShuffleFunc() ShuffleFunc {
	mu.RLock()
	defer mu.RUnlock()
	return shuffleFunc
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *ProxyServerConfig) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.Tunnel == nil {
		return trace.BadParameter("missing Tunnel")
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("missing TLSConfig")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.ServerID == "" {
		return trace.BadParameter("missing ServerID")
	}
	if c.LockWatcher == nil {
		return trace.BadParameter("missing LockWatcher")
	}
	if c.Limiter == nil {
		// Empty config means no connection limit.
		connLimiter, err := limiter.NewLimiter(limiter.Config{})
		if err != nil {
			return trace.Wrap(err)
		}

		c.Limiter = connLimiter
	}
	return nil
}

// NewProxyServer creates a new instance of the database proxy server.
func NewProxyServer(ctx context.Context, config ProxyServerConfig) (*ProxyServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	server := &ProxyServer{
		cfg: config,
		middleware: &auth.Middleware{
			AccessPoint:   config.AccessPoint,
			AcceptedUsage: []string{teleport.UsageDatabaseOnly},
		},
		closeCtx: ctx,
		log:      logrus.WithField(trace.Component, "db:proxy"),
	}
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		server.cfg.TLSConfig, server.cfg.AccessPoint, server.log, types.UserCA)
	return server, nil
}

// ServePostgres starts accepting Postgres connections from the provided listener.
func (s *ProxyServer) ServePostgres(listener net.Listener) error {
	s.log.Debug("Started database proxy.")
	defer s.log.Debug("Database proxy exited.")
	for {
		// Accept the connection from the database client, such as psql.
		// The connection is expected to come through via multiplexer.
		clientConn, err := listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		// Let the appropriate proxy handle the connection and go back
		// to listening.
		go func() {
			defer clientConn.Close()
			err := s.PostgresProxy().HandleConnection(s.closeCtx, clientConn)
			if err != nil && !utils.IsOKNetworkError(err) {
				s.log.WithError(err).Warn("Failed to handle Postgres client connection.")
			}
		}()
	}
}

// ServeMySQL starts accepting MySQL client connections.
func (s *ProxyServer) ServeMySQL(listener net.Listener) error {
	s.log.Debug("Started MySQL proxy.")
	defer s.log.Debug("MySQL proxy exited.")
	for {
		// Accept the connection from a MySQL client.
		clientConn, err := listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		// Pass over to the MySQL proxy handler.
		go func() {
			defer clientConn.Close()
			err := s.MySQLProxy().HandleConnection(s.closeCtx, clientConn)
			if err != nil && !utils.IsOKNetworkError(err) {
				s.log.WithError(err).Error("Failed to handle MySQL client connection.")
			}
		}()
	}
}

// ServeMongo starts accepting Mongo client connections.
func (s *ProxyServer) ServeMongo(listener net.Listener, tlsConfig *tls.Config) error {
	return s.serveGenericTLS(listener, tlsConfig, defaults.ProtocolMongoDB)
}

// serveGenericTLS starts accepting a plain TLS database client connection.
// dbName is used only for logging purposes.
func (s *ProxyServer) serveGenericTLS(listener net.Listener, tlsConfig *tls.Config, dbName string) error {
	s.log.Debugf("Started %s proxy.", dbName)
	defer s.log.Debugf("%s proxy exited.", dbName)
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}

		go func() {
			defer clientConn.Close()
			tlsConn := tls.Server(clientConn, tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				if !utils.IsOKNetworkError(err) {
					s.log.WithError(err).Errorf("%s TLS handshake failed.", dbName)
				}
				return
			}
			err := s.handleConnection(tlsConn)
			if err != nil {
				s.log.WithError(err).Errorf("Failed to handle %s client connection.", dbName)
			}
		}()
	}
}

// ServeTLS starts accepting database connections that use plain TLS connection.
func (s *ProxyServer) ServeTLS(listener net.Listener) error {
	s.log.Debug("Started database TLS proxy.")
	defer s.log.Debug("Database TLS proxy exited.")
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		go func() {
			defer clientConn.Close()
			err := s.handleConnection(clientConn)
			if err != nil {
				s.log.WithError(err).Error("Failed to handle database TLS connection.")
			}
		}()
	}
}

func (s *ProxyServer) handleConnection(conn net.Conn) error {
	s.log.Debugf("Accepted TLS database connection from %v.", conn.RemoteAddr())
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return trace.BadParameter("expected *tls.Conn, got %T", conn)
	}
	clientIP, err := utils.ClientIPFromConn(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Apply connection and rate limiting.
	release, err := s.cfg.Limiter.RegisterRequestAndConnection(clientIP)
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	proxyCtx, err := s.Authorize(s.closeCtx, tlsConn, common.ConnectParams{
		ClientIP: clientIP,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	switch proxyCtx.Identity.RouteToDatabase.Protocol {
	case defaults.ProtocolPostgres, defaults.ProtocolCockroachDB:
		return s.PostgresProxyNoTLS().HandleConnection(s.closeCtx, tlsConn)
	case defaults.ProtocolMySQL:
		version := getMySQLVersionFromServer(proxyCtx.Servers)
		// Set the version in the context to match a behavior in other handlers.
		ctx := context.WithValue(s.closeCtx, dbutils.ContextMySQLServerVersion, version)
		return s.MySQLProxyNoTLS().HandleConnection(ctx, tlsConn)
	case defaults.ProtocolSQLServer:
		return s.SQLServerProxy().HandleConnection(s.closeCtx, proxyCtx, tlsConn)
	}
	serviceConn, err := s.Connect(s.closeCtx, proxyCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()
	err = s.Proxy(s.closeCtx, proxyCtx, tlsConn, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getMySQLVersionFromServer returns the MySQL version returned by an instance on last connection or
// the MySQL.ServerVersion set in configuration if the first one is not available.
// Function picks a random server each time if more than one are available.
func getMySQLVersionFromServer(servers []types.DatabaseServer) string {
	count := len(servers)
	db := servers[rand.Intn(count)].GetDatabase()
	return db.GetMySQLServerVersion()
}

// PostgresProxy returns a new instance of the Postgres protocol aware proxy.
func (s *ProxyServer) PostgresProxy() *postgres.Proxy {
	return &postgres.Proxy{
		TLSConfig:  s.cfg.TLSConfig,
		Middleware: s.middleware,
		Service:    s,
		Limiter:    s.cfg.Limiter,
		Log:        s.log,
	}
}

// PostgresProxyNoTLS returns a new instance of the non-TLS Postgres proxy.
func (s *ProxyServer) PostgresProxyNoTLS() *postgres.Proxy {
	return &postgres.Proxy{
		Middleware: s.middleware,
		Service:    s,
		Limiter:    s.cfg.Limiter,
		Log:        s.log,
	}
}

// MySQLProxy returns a new instance of the MySQL protocol aware proxy.
func (s *ProxyServer) MySQLProxy() *mysql.Proxy {
	return &mysql.Proxy{
		TLSConfig:  s.cfg.TLSConfig,
		Middleware: s.middleware,
		Service:    s,
		Limiter:    s.cfg.Limiter,
		Log:        s.log,
	}
}

// MySQLProxyNoTLS returns a new instance of the non-TLS MySQL proxy.
func (s *ProxyServer) MySQLProxyNoTLS() *mysql.Proxy {
	return &mysql.Proxy{
		Middleware: s.middleware,
		Service:    s,
		Limiter:    s.cfg.Limiter,
		Log:        s.log,
	}
}

// SQLServerProxy returns a new instance of the SQL Server protocol aware proxy.
func (s *ProxyServer) SQLServerProxy() *sqlserver.Proxy {
	return &sqlserver.Proxy{
		Middleware: s.middleware,
		Service:    s,
		Log:        s.log,
	}
}

// Connect connects to the database server running on a remote cluster
// over reverse tunnel and upgrades this end of the connection to TLS so
// the identity can be passed over it.
//
// The passed in context is expected to contain the identity information
// decoded from the client certificate by auth.Middleware.
//
// Implements common.Service.
func (s *ProxyServer) Connect(ctx context.Context, proxyCtx *common.ProxyContext) (net.Conn, error) {
	// There may be multiple database servers proxying the same database. If
	// we get a connection problem error trying to dial one of them, likely
	// the database server is down so try the next one.
	for _, server := range getShuffleFunc()(proxyCtx.Servers) {
		s.log.Debugf("Dialing to %v.", server)
		tlsConfig, err := s.getConfigForServer(ctx, proxyCtx.Identity, server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serviceConn, err := proxyCtx.Cluster.Dial(reversetunnel.DialParams{
			From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: "@db-proxy"},
			To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnel.LocalNode},
			ServerID: fmt.Sprintf("%v.%v", server.GetHostID(), proxyCtx.Cluster.GetName()),
			ConnType: types.DatabaseTunnel,
			ProxyIDs: server.GetProxyIDs(),
		})
		if err != nil {
			// If an agent is down, we'll retry on the next one (if available).
			if isReverseTunnelDownError(err) {
				s.log.WithError(err).Warnf("Failed to dial database %v.", server)
				continue
			}
			return nil, trace.Wrap(err)
		}
		// Upgrade the connection so the client identity can be passed to the
		// remote server during TLS handshake. On the remote side, the connection
		// received from the reverse tunnel will be handled by tls.Server.
		serviceConn = tls.Client(serviceConn, tlsConfig)
		return serviceConn, nil
	}
	return nil, trace.BadParameter("failed to connect to any of the database servers")
}

// isReverseTunnelDownError returns true if the provided error indicates that
// the reverse tunnel connection is down e.g. because the agent is down.
func isReverseTunnelDownError(err error) bool {
	return trace.IsConnectionProblem(err) ||
		strings.Contains(err.Error(), reversetunnel.NoDatabaseTunnel)
}

// Proxy starts proxying all traffic received from database client between
// this proxy and Teleport database service over reverse tunnel.
//
// Implements common.Service.
func (s *ProxyServer) Proxy(ctx context.Context, proxyCtx *common.ProxyContext, clientConn, serviceConn net.Conn) error {
	// Wrap a client connection into monitor that auto-terminates
	// idle connection and connection with expired cert.
	tc, err := monitorConn(ctx, monitorConnConfig{
		conn:         clientConn,
		lockWatcher:  s.cfg.LockWatcher,
		lockTargets:  proxyCtx.AuthContext.LockTargets(),
		identity:     proxyCtx.AuthContext.Identity.GetIdentity(),
		checker:      proxyCtx.AuthContext.Checker,
		clock:        s.cfg.Clock,
		serverID:     s.cfg.ServerID,
		authClient:   s.cfg.AuthClient,
		teleportUser: proxyCtx.AuthContext.Identity.GetIdentity().Username,
		emitter:      s.cfg.Emitter,
		log:          s.log,
		ctx:          s.closeCtx,
	})
	if err != nil {
		clientConn.Close()
		serviceConn.Close()
		return trace.Wrap(err)
	}
	errCh := make(chan error, 2)
	go func() {
		defer s.log.Debug("Stop proxying from client to service.")
		defer serviceConn.Close()
		defer tc.Close()
		_, err := io.Copy(serviceConn, tc)
		errCh <- err
	}()
	go func() {
		defer s.log.Debug("Stop proxying from service to client.")
		defer serviceConn.Close()
		defer tc.Close()
		_, err := io.Copy(tc, serviceConn)
		errCh <- err
	}()
	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && !utils.IsOKNetworkError(err) {
				s.log.WithError(err).Warn("Connection problem.")
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return trace.ConnectionProblem(nil, "context is closing")
		}
	}
	return trace.NewAggregate(errs...)
}

// monitorConnConfig is a monitorConn configuration.
type monitorConnConfig struct {
	conn         net.Conn
	lockWatcher  *services.LockWatcher
	lockTargets  []types.LockTarget
	checker      services.AccessChecker
	identity     tlsca.Identity
	clock        clockwork.Clock
	serverID     string
	authClient   *auth.Client
	teleportUser string
	emitter      events.Emitter
	log          logrus.FieldLogger
	ctx          context.Context
}

// monitorConn wraps a client connection with TrackingReadConn, starts a connection monitor and
// returns a tracking connection that will be auto-terminated in case disconnect_expired_cert or idle timeout is
// configured, and unmodified client connection otherwise.
func monitorConn(ctx context.Context, cfg monitorConnConfig) (net.Conn, error) {
	authPref, err := cfg.authClient.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfig, err := cfg.authClient.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certExpires := cfg.identity.Expires
	var disconnectCertExpired time.Time
	if !certExpires.IsZero() && cfg.checker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert()) {
		disconnectCertExpired = certExpires
	}
	idleTimeout := cfg.checker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout())
	ctx, cancel := context.WithCancel(ctx)
	tc, err := srv.NewTrackingReadConn(srv.TrackingReadConnConfig{
		Conn:    cfg.conn,
		Clock:   cfg.clock,
		Context: ctx,
		Cancel:  cancel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start monitoring client connection. When client connection is closed the monitor goroutine exits.
	err = srv.StartMonitor(srv.MonitorConfig{
		LockWatcher:           cfg.lockWatcher,
		LockTargets:           cfg.lockTargets,
		DisconnectExpiredCert: disconnectCertExpired,
		ClientIdleTimeout:     idleTimeout,
		Conn:                  cfg.conn,
		Tracker:               tc,
		Context:               cfg.ctx,
		Clock:                 cfg.clock,
		ServerID:              cfg.serverID,
		TeleportUser:          cfg.teleportUser,
		Emitter:               cfg.emitter,
		Entry:                 cfg.log,
	})
	if err != nil {
		tc.Close()
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

// Authorize authorizes the provided client TLS connection.
func (s *ProxyServer) Authorize(ctx context.Context, tlsConn *tls.Conn, params common.ConnectParams) (*common.ProxyContext, error) {
	ctx, err := s.middleware.WrapContextWithUser(ctx, tlsConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()
	if params.User != "" {
		identity.RouteToDatabase.Username = params.User
	}
	if params.Database != "" {
		identity.RouteToDatabase.Database = params.Database
	}
	if params.ClientIP != "" {
		identity.ClientIP = params.ClientIP
	}
	cluster, servers, err := s.getDatabaseServers(ctx, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &common.ProxyContext{
		Identity:    identity,
		Cluster:     cluster,
		Servers:     servers,
		AuthContext: authContext,
	}, nil
}

// getDatabaseServers finds database servers that proxy the database instance
// encoded in the provided identity.
func (s *ProxyServer) getDatabaseServers(ctx context.Context, identity tlsca.Identity) (reversetunnel.RemoteSite, []types.DatabaseServer, error) {
	cluster, err := s.cfg.Tunnel.GetSite(identity.RouteToCluster)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessPoint, err := cluster.CachingAccessPoint()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	servers, err := accessPoint.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s.log.Debugf("Available databases in %v: %s.", cluster.GetName(), servers)
	// Find out which database servers proxy the database a user is
	// connecting to using routing information from identity.
	var result []types.DatabaseServer
	for _, server := range servers {
		if server.GetDatabase().GetName() == identity.RouteToDatabase.ServiceName {
			result = append(result, server)
		}
	}
	if len(result) != 0 {
		return cluster, result, nil
	}
	return nil, nil, trace.NotFound("database %q not found among registered databases in cluster %q",
		identity.RouteToDatabase.ServiceName,
		identity.RouteToCluster)
}

// getConfigForServer returns TLS config used for establishing connection
// to a remote database server over reverse tunnel.
func (s *ProxyServer) getConfigForServer(ctx context.Context, identity tlsca.Identity, server types.DatabaseServer) (*tls.Config, error) {
	privateKeyBytes, _, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DatabaseCA was introduced in Teleport 10. Older versions require database certificate signed
	// with UserCA where Teleport 10+ uses DatabaseCA.
	ver10orAbove, err := utils.MinVerWithoutPreRelease(server.GetTeleportVersion(), constants.DatabaseCAMinVersion)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse Teleport version: %q", server.GetTeleportVersion())
	}

	response, err := s.cfg.AuthClient.SignDatabaseCSR(ctx, &proto.DatabaseCSRRequest{
		CSR:         csr,
		ClusterName: identity.RouteToCluster,
		// TODO: Remove in Teleport 11.
		SignWithDatabaseCA: ver10orAbove,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(response.Cert, privateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, caCert := range response.CACerts {
		ok := pool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, trace.BadParameter("failed to append CA certificate")
		}
	}
	return &tls.Config{
		ServerName:   server.GetHostname(),
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

func getConfigForClient(conf *tls.Config, ap auth.ReadDatabaseAccessPoint, log logrus.FieldLogger, caTypes ...types.CertAuthType) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil && !trace.IsNotFound(err) {
				log.Debugf("Ignoring unsupported cluster name %q.", info.ServerName)
			}
		}
		pool, _, err := auth.ClientCertPool(ap, clusterName, caTypes...)
		if err != nil {
			log.WithError(err).Error("Failed to retrieve client CA pool.")
			return nil, nil // Fall back to the default config.
		}
		tlsCopy := conf.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}
