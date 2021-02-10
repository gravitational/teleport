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
	"net"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/native"
	auth "github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
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
	AuthClient *client.Client
	// AccessPoint is the caching client connected to the auth server.
	AccessPoint libauth.AccessPoint
	// Authorizer is responsible for authorizing user identities.
	Authorizer auth.Authorizer
	// Tunnel is the reverse tunnel server.
	Tunnel reversetunnel.Server
	// TLSConfig is the proxy server TLS configuration.
	TLSConfig *tls.Config
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
			AccessPoint: config.AccessPoint,
		},
		closeCtx: ctx,
		log:      logrus.WithField(trace.Component, "db:proxy"),
	}
	server.cfg.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	server.cfg.TLSConfig.GetConfigForClient = getConfigForClient(
		server.cfg.TLSConfig, server.cfg.AccessPoint, server.log)
	return server, nil
}

// Serve starts accepting database connections from the provided listener.
func (s *ProxyServer) Serve(listener net.Listener) error {
	s.log.Debug("Started database proxy.")
	defer s.log.Debug("Database proxy exited.")
	for {
		// Accept the connection from the database client, such as psql.
		// The connection is expected to come through via multiplexer.
		clientConn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), teleport.UseOfClosedNetworkConnection) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		// The multiplexed connection contains information about detected
		// protocol so dispatch to the appropriate proxy.
		proxy, err := s.dispatch(clientConn)
		if err != nil {
			s.log.WithError(err).Error("Failed to dispatch client connection.")
			continue
		}
		// Let the appropriate proxy handle the connection and go back
		// to listening.
		go func() {
			defer clientConn.Close()
			err := proxy.HandleConnection(s.closeCtx, clientConn)
			if err != nil {
				s.log.Errorf("Failed to handle client connection: %v.",
					trace.DebugReport(err))
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
			if strings.Contains(err.Error(), teleport.UseOfClosedNetworkConnection) {
				return nil
			}
			return trace.Wrap(err)
		}
		// Pass over to the MySQL proxy handler.
		go func() {
			defer clientConn.Close()
			err := s.mysqlProxy().HandleConnection(s.closeCtx, clientConn)
			if err != nil {
				s.log.WithError(err).Error("Failed to handle MySQL client connection.")
			}
		}()
	}
}

// dispatch dispatches the connection to appropriate database proxy.
func (s *ProxyServer) dispatch(clientConn net.Conn) (common.Proxy, error) {
	muxConn, ok := clientConn.(*multiplexer.Conn)
	if !ok {
		return nil, trace.BadParameter("expected multiplexer connection, got %T", clientConn)
	}
	switch muxConn.Protocol() {
	case multiplexer.ProtoPostgres:
		s.log.Debugf("Accepted Postgres connection from %v.", muxConn.RemoteAddr())
		return s.postgresProxy(), nil
	}
	return nil, trace.BadParameter("unsupported database protocol %q",
		muxConn.Protocol())
}

// postgresProxy returns a new instance of the Postgres protocol aware proxy.
func (s *ProxyServer) postgresProxy() *postgres.Proxy {
	return &postgres.Proxy{
		TLSConfig:  s.cfg.TLSConfig,
		Middleware: s.middleware,
		Service:    s,
		Log:        s.log,
	}
}

// mysqlProxy returns a new instance of the MySQL protocol aware proxy.
func (s *ProxyServer) mysqlProxy() *mysql.Proxy {
	return &mysql.Proxy{
		TLSConfig:  s.cfg.TLSConfig,
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
func (s *ProxyServer) Connect(ctx context.Context, user, database string) (net.Conn, error) {
	authContext, err := s.authorize(ctx, user, database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := s.getConfigForServer(ctx, authContext.identity, authContext.server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceConn, err := authContext.cluster.Dial(reversetunnel.DialParams{
		From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: "@db-proxy"},
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnel.LocalNode},
		ServerID: fmt.Sprintf("%v.%v", authContext.server.GetHostID(), authContext.cluster.GetName()),
		ConnType: types.DatabaseTunnel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Upgrade the connection so the client identity can be passed to the
	// remote server during TLS handshake. On the remote side, the connection
	// received from the reverse tunnel will be handled by tls.Server.
	serviceConn = tls.Client(serviceConn, tlsConfig)
	return serviceConn, nil
}

// Proxy starts proxying all traffic received from database client between
// this proxy and Teleport database service over reverse tunnel.
//
// Implements common.Service.
func (s *ProxyServer) Proxy(ctx context.Context, clientConn, serviceConn io.ReadWriteCloser) error {
	errCh := make(chan error, 2)
	go func() {
		defer s.log.Debug("Stop proxying from client to service.")
		defer serviceConn.Close()
		defer clientConn.Close()
		_, err := io.Copy(serviceConn, clientConn)
		errCh <- err
	}()
	go func() {
		defer s.log.Debug("Stop proxying from service to client.")
		defer serviceConn.Close()
		defer clientConn.Close()
		_, err := io.Copy(clientConn, serviceConn)
		errCh <- err
	}()
	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && err != io.EOF && !strings.Contains(err.Error(), teleport.UseOfClosedNetworkConnection) {
				s.log.WithError(err).Warn("Connection problem.")
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return trace.ConnectionProblem(nil, "context is closing")
		}
	}
	return trace.NewAggregate(errs...)
}

// proxyContext contains parameters for a database session being proxied.
type proxyContext struct {
	// identity is the authorized client identity.
	identity tlsca.Identity
	// cluster is the remote cluster running the database server.
	cluster reversetunnel.RemoteSite
	// server is a database server that has the requested database.
	server types.DatabaseServer
}

func (s *ProxyServer) authorize(ctx context.Context, user, database string) (*proxyContext, error) {
	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()
	identity.RouteToDatabase.Username = user
	identity.RouteToDatabase.Database = database
	cluster, server, err := s.pickDatabaseServer(ctx, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Will proxy to database %q on server %s.", server.GetName(), server)
	return &proxyContext{
		identity: identity,
		cluster:  cluster,
		server:   server,
	}, nil
}

// pickDatabaseServer finds a database server instance to proxy requests
// to based on the routing information from the provided identity.
func (s *ProxyServer) pickDatabaseServer(ctx context.Context, identity tlsca.Identity) (reversetunnel.RemoteSite, types.DatabaseServer, error) {
	cluster, err := s.cfg.Tunnel.GetSite(identity.RouteToCluster)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessPoint, err := cluster.CachingAccessPoint()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	servers, err := accessPoint.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	s.log.Debugf("Available database servers on %v: %s.", cluster.GetName(), servers)
	// Find out which database servers proxy the database a user is
	// connecting to using routing information from identity.
	for _, server := range servers {
		if server.GetName() == identity.RouteToDatabase.ServiceName {
			// TODO(r0mant): Return all matching servers and round-robin
			// between them.
			return cluster, server, nil
		}
	}
	return nil, nil, trace.NotFound("database %q not found among registered database servers on cluster %q",
		identity.RouteToDatabase.ServiceName,
		identity.RouteToCluster)
}

// getConfigForServer returns TLS config used for establishing connection
// to a remote database server over reverse tunnel.
func (s *ProxyServer) getConfigForServer(ctx context.Context, identity tlsca.Identity, server types.DatabaseServer) (*tls.Config, error) {
	privateKeyBytes, _, err := native.GenerateKeyPair("")
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
	response, err := s.cfg.AuthClient.SignDatabaseCSR(ctx, &proto.DatabaseCSRRequest{
		CSR:         csr,
		ClusterName: identity.RouteToCluster,
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

func getConfigForClient(conf *tls.Config, ap libauth.AccessPoint, log logrus.FieldLogger) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			clusterName, err = libauth.DecodeClusterName(info.ServerName)
			if err != nil && !trace.IsNotFound(err) {
				log.Debugf("Ignoring unsupported cluster name %q.", info.ServerName)
			}
		}
		pool, err := auth.ClientCertPool(ap, clusterName)
		if err != nil {
			log.WithError(err).Error("Failed to retrieve client CA pool.")
			return nil, nil // Fall back to the default config.
		}
		tlsCopy := conf.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}
