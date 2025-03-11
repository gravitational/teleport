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

package mysql

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/utils"
)

// Proxy proxies connections from MySQL clients to database services
// over reverse tunnel. It runs inside Teleport proxy service.
//
// Implements common.Proxy.
type Proxy struct {
	// TLSConfig is the proxy TLS configuration.
	TLSConfig *tls.Config
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// Service is used to connect to a remote database service.
	Service common.Service
	// Log is used for logging.
	Log *slog.Logger
	// Limiter applies limits for database connections.
	Limiter common.Limiter
	// IngressReporter reports new and active connections.
	IngressReporter common.IngressReporter
	// ServerVersion allows to overwrite the default Proxy MySQL Engine Version. Note that for TLS Routing connection
	// the dynamic service version propagation by ALPN extension will take precedes over Proxy ServerVersion.
	ServerVersion string
}

// HandleConnection accepts connection from a MySQL client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
	if p.IngressReporter != nil {
		p.IngressReporter.ConnectionAccepted(ingress.MySQL, clientConn)
		defer p.IngressReporter.ConnectionClosed(ingress.MySQL, clientConn)
	}

	// Wrap the client connection in the connection that can detect the protocol
	// by peeking into the first few bytes. This is needed to be able to detect
	// proxy protocol which otherwise would interfere with MySQL protocol.
	conn := multiplexer.NewConn(clientConn)
	mysqlServerVersion := getServerVersionFromCtx(ctx, p.ServerVersion)

	mysqlServer := p.makeServer(conn, mysqlServerVersion)
	// If any error happens, make sure to send it back to the client, so it
	// has a chance to close the connection from its side.
	defer func() {
		if r := recover(); r != nil {
			p.Log.WarnContext(ctx, "Recovered in MySQL proxy while handling connectionv.", "from", clientConn.RemoteAddr(), "to", r)
			err = trace.BadParameter("failed to handle MySQL client connection")
		}
		if err != nil {
			if writeErr := mysqlServer.WriteError(err); writeErr != nil {
				p.Log.DebugContext(ctx, "Failed to send error to MySQL client.", "original_err", err.Error(), "error", writeErr)
			}
		}
	}()
	// Perform first part of the handshake, up to the point where client sends
	// us certificate and connection upgrades to TLS.
	tlsConn, err := p.performHandshake(ctx, conn, mysqlServer)
	if err != nil {
		return trace.Wrap(err)
	}

	// Apply connection and rate limiting.
	releaseConn, clientIP, err := p.Limiter.RegisterClientIP(tlsConn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseConn()

	proxyCtx, err := p.Service.Authorize(ctx, tlsConn, common.ConnectParams{
		User:     mysqlServer.GetUser(),
		Database: mysqlServer.GetDatabase(),
		ClientIP: clientIP,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Apply per user max connections.
	releaseIdentity, err := p.Limiter.RegisterIdentity(ctx, proxyCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseIdentity()

	if p.IngressReporter != nil {
		p.IngressReporter.ConnectionAuthenticated(ingress.MySQL, clientConn)
		defer p.IngressReporter.AuthenticatedConnectionClosed(ingress.MySQL, clientConn)
	}

	serviceConn, err := p.Service.Connect(ctx, proxyCtx, clientConn.RemoteAddr(), clientConn.LocalAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()
	// Before replying OK to the client which would make the client consider
	// auth completed, wait for OK packet from db service indicating auth
	// success.
	err = p.waitForOK(mysqlServer, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Auth has completed, the client enters command phase, start proxying
	// all messages back-and-forth.
	err = p.Service.Proxy(ctx, proxyCtx, tlsConn, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getServerVersionFromCtx tries to extract MySQL server version from the passed context.
// The default version is returned if context doesn't have it.
func getServerVersionFromCtx(ctx context.Context, configEngineVersion string) string {
	// Set default server version or use the Proxy MySQL Engine Version if it was provided.
	mysqlServerVersion := DefaultServerVersion
	if configEngineVersion != "" {
		mysqlServerVersion = configEngineVersion
	}

	if mysqlVerCtx := ctx.Value(dbutils.ContextMySQLServerVersion); mysqlVerCtx != nil {
		version, ok := mysqlVerCtx.(string)
		if ok {
			mysqlServerVersion = version
		}
	}
	return mysqlServerVersion
}

// credentialProvider is used by MySQL server created below.
//
// It's a no-op because authentication is done via mTLS.
type credentialProvider struct{}

func (p *credentialProvider) CheckUsername(_ string) (bool, error)         { return true, nil }
func (p *credentialProvider) GetCredential(_ string) (string, bool, error) { return "", true, nil }

// makeServer creates a MySQL server from the accepted client connection that
// provides access to various parts of the handshake.
func (p *Proxy) makeServer(clientConn net.Conn, serverVersion string) *server.Conn {
	return server.MakeConn(
		clientConn,
		server.NewServer(
			serverVersion,
			mysql.DEFAULT_COLLATION_ID,
			mysql.AUTH_NATIVE_PASSWORD,
			nil,
			// TLS config can actually be nil if the client is connecting
			// through local TLS proxy without TLS.
			p.TLSConfig),
		&credentialProvider{},
		server.EmptyHandler{})
}

// performHandshake performs the initial handshake between MySQL client and
// this server, up to the point where the client sends us a certificate for
// authentication, and returns the upgraded connection.
func (p *Proxy) performHandshake(ctx context.Context, conn *multiplexer.Conn, server *server.Conn) (utils.TLSConn, error) {
	// MySQL protocol is server-initiated which means the client will expect
	// server to send initial handshake message.
	err := server.WriteInitialHandshake()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// See if we need to read the proxy-line which could happen if Teleport
	// is running behind a load balancer with proxy protocol enabled.
	err = p.maybeReadProxyLine(ctx, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Then proceed normally to MySQL handshake.
	err = server.ReadHandshakeResponse()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// First part of the handshake completed and the connection has been
	// upgraded to TLS, so now we can look at the client certificate and
	// see which database service to route the connection to.
	switch c := server.Conn.Conn.(type) {
	case *tls.Conn:
		return c, nil
	case *multiplexer.Conn:
		tlsConn, ok := c.Conn.(utils.TLSConn)
		if !ok {
			return nil, trace.BadParameter("expected TLS connection, got: %T", c.Conn)
		}

		return tlsConn, nil
	}
	return nil, trace.BadParameter("expected *tls.Conn or *multiplexer.Conn, got: %T",
		server.Conn.Conn)
}

// maybeReadProxyLine peeks into the connection to see if instead of regular
// MySQL protocol we were sent a proxy-line. This usually happens when Teleport
// is running behind a load balancer with proxy protocol enabled.
func (p *Proxy) maybeReadProxyLine(ctx context.Context, conn *multiplexer.Conn) error {
	proto, err := conn.Detect()
	if err != nil {
		return trace.Wrap(err)
	}
	if proto != multiplexer.ProtoProxy && proto != multiplexer.ProtoProxyV2 {
		return nil
	}
	proxyLine, err := conn.ReadProxyLine()
	if err != nil {
		return trace.Wrap(err)
	}
	p.Log.DebugContext(ctx, "MySQL listener proxy-line.", "proxy_line", proxyLine)
	return nil
}

// waitForOK waits for OK_PACKET from the database service which indicates
// that auth on the other side completed successfully.
func (p *Proxy) waitForOK(server *server.Conn, serviceConn net.Conn) error {
	err := serviceConn.SetReadDeadline(time.Now().Add(2 * defaults.DatabaseConnectTimeout))
	if err != nil {
		return trace.Wrap(err)
	}
	packet, err := protocol.ParsePacket(serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = serviceConn.SetReadDeadline(time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}
	switch p := packet.(type) {
	case *protocol.OK:
		err = server.WriteOK(nil)
		if err != nil {
			return trace.Wrap(err)
		}
	case *protocol.Error:
		// There may be a difference in capabilities between client <--> proxy
		// than there is between proxy <--> agent, most notably,
		// CLIENT_PROTOCOL_41.
		// So rather than forwarding packet bytes directly, convert the error
		// packet into MyError and write with respect to caps between
		// client <--> proxy.
		err = server.WriteError(mysql.NewError(p.Code, p.Message))
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("expected OK or ERR packet, got %s", packet)
	}
	return nil
}

const (
	// DefaultServerVersion is advertised to MySQL clients during handshake.
	//
	// Some clients may refuse to work with older servers (e.g. MySQL
	// Workbench requires > 5.5).
	DefaultServerVersion = "8.0.0-Teleport"
)
