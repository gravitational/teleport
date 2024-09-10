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
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/enterprise"
	"github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
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
	log *slog.Logger
}

// ConnMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnMonitor interface {
	MonitorConn(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
}

// ProxyServerConfig is the proxy configuration.
type ProxyServerConfig struct {
	// AuthClient is the authenticated client to the auth server.
	AuthClient *authclient.Client
	// AccessPoint is the caching client connected to the auth server.
	AccessPoint authclient.ReadDatabaseAccessPoint
	// Authorizer is responsible for authorizing user identities.
	Authorizer authz.Authorizer
	// Tunnel is the reverse tunnel server.
	Tunnel reversetunnelclient.Server
	// TLSConfig is the proxy server TLS configuration.
	TLSConfig *tls.Config
	// Limiter is the connection/rate limiter.
	Limiter *limiter.Limiter
	// IngressReporter reports new and active connections.
	IngressReporter *ingress.Reporter
	// ConnectionMonitor monitors and closes connections if session controls
	// prevent the connections.
	ConnectionMonitor ConnMonitor
	// MySQLServerVersion  allows to override the default MySQL Engine Version propagated by Teleport Proxy.
	MySQLServerVersion string
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
	if c.ConnectionMonitor == nil {
		return trace.BadParameter("missing ConnectionMonitor")
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

const proxyServerComponent = "db:proxy"

// NewProxyServer creates a new instance of the database proxy server.
func NewProxyServer(ctx context.Context, config ProxyServerConfig) (*ProxyServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clustername, err := config.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server := &ProxyServer{
		cfg: config,
		middleware: &auth.Middleware{
			ClusterName:   clustername.GetClusterName(),
			AcceptedUsage: []string{teleport.UsageDatabaseOnly},
		},
		closeCtx: ctx,
		log:      slog.With(teleport.ComponentKey, proxyServerComponent),
	}
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		ctx, server.cfg.TLSConfig, server.cfg.AccessPoint, server.log, types.UserCA,
	)
	return server, nil
}

// ServePostgres starts accepting Postgres connections from the provided listener.
func (s *ProxyServer) ServePostgres(listener net.Listener) error {
	s.log.DebugContext(s.closeCtx, "Started database proxy.")
	defer s.log.DebugContext(s.closeCtx, "Database proxy exited.")
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
				s.log.WarnContext(s.closeCtx, "Failed to handle Postgres client connection.", "error", err)
			}
		}()
	}
}

// ServeMySQL starts accepting MySQL client connections.
func (s *ProxyServer) ServeMySQL(listener net.Listener) error {
	s.log.DebugContext(s.closeCtx, "Started MySQL proxy.")
	defer s.log.DebugContext(s.closeCtx, "MySQL proxy exited.")
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
				s.log.ErrorContext(s.closeCtx, "Failed to handle MySQL client connection.", "error", err)
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
	s.log.DebugContext(s.closeCtx, "Started DB proxy.", "db_type", dbName)
	defer s.log.DebugContext(s.closeCtx, "DB proxy exited.", "db_type", dbName)
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
			if err := tlsConn.HandshakeContext(s.closeCtx); err != nil {
				if !utils.IsOKNetworkError(err) {
					s.log.ErrorContext(s.closeCtx, "TLS handshake failed.", "db_type", dbName)
				}
				return
			}
			err := s.handleConnection(tlsConn)
			if err != nil {
				s.log.ErrorContext(s.closeCtx, "Failed to handle client connection.", "db_type", dbName)
			}
		}()
	}
}

// ServeTLS starts accepting database connections that use plain TLS connection.
func (s *ProxyServer) ServeTLS(listener net.Listener) error {
	s.log.DebugContext(s.closeCtx, "Started database TLS proxy.")
	defer s.log.DebugContext(s.closeCtx, "Database TLS proxy exited.")
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
				s.log.ErrorContext(s.closeCtx, "Failed to handle database TLS connection.", "error", err)
			}
		}()
	}
}

func (s *ProxyServer) handleConnection(conn net.Conn) error {
	if s.cfg.IngressReporter != nil {
		s.cfg.IngressReporter.ConnectionAccepted(ingress.DatabaseTLS, conn)
		defer s.cfg.IngressReporter.ConnectionClosed(ingress.DatabaseTLS, conn)
	}

	s.log.DebugContext(s.closeCtx, "Accepted TLS database connection.", "from", conn.RemoteAddr())
	tlsConn, ok := conn.(utils.TLSConn)
	if !ok {
		return trace.BadParameter("expected utils.TLSConn, got %T", conn)
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

	if s.cfg.IngressReporter != nil {
		s.cfg.IngressReporter.ConnectionAuthenticated(ingress.DatabaseTLS, conn)
		defer s.cfg.IngressReporter.AuthenticatedConnectionClosed(ingress.DatabaseTLS, conn)
	}
	if err = enterprise.ProtocolValidation(proxyCtx.Identity.RouteToDatabase.Protocol); err != nil {
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

	serviceConn, err := s.Connect(s.closeCtx, proxyCtx, conn.RemoteAddr(), conn.LocalAddr())
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
		TLSConfig:       s.cfg.TLSConfig,
		Middleware:      s.middleware,
		Service:         s,
		Limiter:         s.cfg.Limiter,
		Log:             s.log,
		IngressReporter: s.cfg.IngressReporter,
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
		TLSConfig:       s.cfg.TLSConfig,
		Middleware:      s.middleware,
		Service:         s,
		Limiter:         s.cfg.Limiter,
		Log:             s.log,
		IngressReporter: s.cfg.IngressReporter,
		ServerVersion:   s.cfg.MySQLServerVersion,
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
func (s *ProxyServer) Connect(ctx context.Context, proxyCtx *common.ProxyContext, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error) {
	var labels prometheus.Labels
	if len(proxyCtx.Servers) > 0 {
		labels = getLabelsFromDB(proxyCtx.Servers[0].GetDatabase())
	} else {
		labels = getLabelsFromDB(nil)
	}

	labels["available_db_servers"] = strconv.Itoa(len(proxyCtx.Servers))

	defer observeLatency(connectionSetupTime.With(labels))()

	var attemptedServers int
	defer func() {
		dialAttemptedServers.With(labels).Observe(float64(attemptedServers))
	}()

	// There may be multiple database servers proxying the same database. If
	// we get a connection problem error trying to dial one of them, likely
	// the database server is down so try the next one.
	for _, server := range getShuffleFunc()(proxyCtx.Servers) {
		attemptedServers++
		s.log.DebugContext(ctx, "Dialing to database service.", "server", server)
		tlsConfig, err := s.getConfigForServer(ctx, proxyCtx.Identity, server)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialAttempts.With(labels).Inc()
		serviceConn, err := proxyCtx.Cluster.Dial(reversetunnelclient.DialParams{
			From:                  clientSrcAddr,
			To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnelclient.LocalNode},
			OriginalClientDstAddr: clientDstAddr,
			ServerID:              fmt.Sprintf("%v.%v", server.GetHostID(), proxyCtx.Cluster.GetName()),
			ConnType:              types.DatabaseTunnel,
			ProxyIDs:              server.GetProxyIDs(),
		})
		if err != nil {
			dialFailures.With(labels).Inc()
			// If an agent is down, we'll retry on the next one (if available).
			if isReverseTunnelDownError(err) {
				s.log.WarnContext(ctx, "Failed to dial database service.", "server", server, "error", err)
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
		strings.Contains(err.Error(), reversetunnelclient.NoDatabaseTunnel)
}

// Proxy starts proxying all traffic received from database client between
// this proxy and Teleport database service over reverse tunnel.
//
// Implements common.Service.
func (s *ProxyServer) Proxy(ctx context.Context, proxyCtx *common.ProxyContext, clientConn, serviceConn net.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Wrap a client connection with a monitor that auto-terminates
	// idle connection and connection with expired cert.
	var err error
	ctx, clientConn, err = s.cfg.ConnectionMonitor.MonitorConn(ctx, proxyCtx.AuthContext, clientConn)
	if err != nil {
		clientConn.Close()
		serviceConn.Close()
		return trace.Wrap(err)
	}

	var labels prometheus.Labels
	if len(proxyCtx.Servers) > 0 {
		labels = getLabelsFromDB(proxyCtx.Servers[0].GetDatabase())
	} else {
		labels = getLabelsFromDB(nil)
	}

	activeConnections.With(labels).Inc()
	defer activeConnections.With(labels).Dec()

	err = utils.ProxyConn(ctx, clientConn, serviceConn)

	// The clientConn is closed by utils.ProxyConn on successful io.Copy thus
	// possibly causing utils.ProxyConn to return io.EOF from
	// context.Cause(ctx), as monitor context is closed when
	// TrackingReadConn.Close() is called.
	if errors.Is(err, io.EOF) {
		return nil
	}
	return trace.Wrap(err)
}

// Authorize authorizes the provided client TLS connection.
func (s *ProxyServer) Authorize(ctx context.Context, tlsConn utils.TLSConn, params common.ConnectParams) (*common.ProxyContext, error) {
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
		identity.LoginIP = params.ClientIP
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
func (s *ProxyServer) getDatabaseServers(ctx context.Context, identity tlsca.Identity) (reversetunnelclient.RemoteSite, []types.DatabaseServer, error) {
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
	s.log.DebugContext(ctx, "Available database servers.", "cluster", cluster.GetName(), "servers", servers)
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
	defer observeLatency(tlsConfigTime.With(getLabelsFromDB(server.GetDatabase())))()

	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.cfg.AccessPoint),
		cryptosuites.ProxyToDatabaseAgent)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := s.cfg.AuthClient.SignDatabaseCSR(ctx, &proto.DatabaseCSRRequest{
		CSR:         csr,
		ClusterName: identity.RouteToCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := keys.TLSCertificateForSigner(privateKey, response.Cert)
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

func getConfigForClient(ctx context.Context, conf *tls.Config, ap authclient.ReadDatabaseAccessPoint, log *slog.Logger, caTypes ...types.CertAuthType) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil && !trace.IsNotFound(err) {
				log.DebugContext(ctx, "Ignoring unsupported cluster name.", "cluster_name", info.ServerName)
			}
		}
		pool, _, err := authclient.ClientCertPool(info.Context(), ap, clusterName, caTypes...)
		if err != nil {
			log.ErrorContext(ctx, "Failed to retrieve client CA pool.", "error", err)
			return nil, nil // Fall back to the default config.
		}
		tlsCopy := conf.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}

func init() {
	_ = metrics.RegisterPrometheusCollectors(prometheusCollectors...)
}

func observeLatency(o prometheus.Observer) func() {
	start := time.Now()
	return func() {
		o.Observe(time.Since(start).Seconds())
	}
}

var commonLabels = []string{teleport.ComponentLabel, "db_protocol", "db_type"}

func getLabelsFromDB(db types.Database) prometheus.Labels {
	if db != nil {
		return map[string]string{
			teleport.ComponentLabel: proxyServerComponent,
			"db_protocol":           db.GetProtocol(),
			"db_type":               db.GetType(),
		}
	}

	return map[string]string{
		teleport.ComponentLabel: proxyServerComponent,
		"db_protocol":           "unknown",
		"db_type":               "unknown",
	}
}

var (
	connectionSetupTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_setup_time_seconds",
			Subsystem: "proxy_db",
			Help:      "Time to establish connection to DB service from Proxy service.",
			// 1ms ... 14.5h
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 20),
		},
		append([]string{"available_db_servers"}, commonLabels...),
	)

	dialAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_dial_attempts_total",
			Subsystem: "proxy_db",
			Help:      "Number of dial attempts from Proxy to DB service made",
		},
		append([]string{"available_db_servers"}, commonLabels...),
	)

	dialFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_dial_failures_total",
			Subsystem: "proxy_db",
			Help:      "Number of failed dial attempts from Proxy to DB service made",
		},
		append([]string{"available_db_servers"}, commonLabels...),
	)

	dialAttemptedServers = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "attempted_servers_total",
			Subsystem: "proxy_db",
			Help:      "Number of servers processed during connection attempt to the DB service from Proxy service.",
			Buckets:   prometheus.LinearBuckets(1, 1, 16),
		},
		append([]string{"available_db_servers"}, commonLabels...),
	)

	tlsConfigTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_tls_config_time_seconds",
			Subsystem: "proxy_db",
			Help:      "Time to fetch TLS configuration for the connection to DB service from Proxy service.",
			// 1ms ... 14.5h
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 20),
		},
		commonLabels,
	)

	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_db",
			Name:      "active_connections_total",
			Help:      "Number of currently active connections to DB service from Proxy service.",
		},
		commonLabels,
	)

	prometheusCollectors = []prometheus.Collector{
		connectionSetupTime, tlsConfigTime, dialAttempts, dialFailures, dialAttemptedServers, activeConnections,
	}
)
