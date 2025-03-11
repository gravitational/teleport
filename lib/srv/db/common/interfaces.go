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

package common

import (
	"context"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Proxy defines an interface a database proxy should implement.
type Proxy interface {
	// HandleConnection takes the client connection, handles all database
	// specific startup actions and starts proxying to remote server.
	HandleConnection(context.Context, net.Conn) error
}

// ConnectParams keeps parameters used when connecting to Service.
type ConnectParams struct {
	// User is a database username.
	User string
	// Database is a database name/schema.
	Database string
	// ClientIP is a client real IP. Currently, used for rate limiting.
	ClientIP string
}

// Service defines an interface for connecting to a remote database service.
type Service interface {
	// Authorize authorizes the provided client TLS connection.
	Authorize(ctx context.Context, tlsConn utils.TLSConn, params ConnectParams) (*ProxyContext, error)
	// Connect is used to connect to remote database server over reverse tunnel.
	Connect(ctx context.Context, proxyCtx *ProxyContext, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error)
	// Proxy starts proxying between client and service connections.
	Proxy(ctx context.Context, proxyCtx *ProxyContext, clientConn, serviceConn net.Conn) error
}

// ProxyContext contains parameters for a database session being proxied.
type ProxyContext struct {
	// Identity is the authorized client Identity.
	Identity tlsca.Identity
	// Cluster is the remote Cluster running the database server.
	Cluster reversetunnelclient.RemoteSite
	// Servers is a list of database Servers that proxy the requested database.
	Servers []types.DatabaseServer
	// AuthContext is a context of authenticated user.
	AuthContext *authz.Context
}

// Engine defines an interface for specific database protocol engine such
// as Postgres or MySQL.
type Engine interface {
	// InitializeConnection initializes the client connection. No DB connection is made at this point, but a message
	// can be sent to a client in a database format.
	InitializeConnection(clientConn net.Conn, sessionCtx *Session) error
	// SendError sends an error to a client in database encoded format.
	// NOTE: Client connection must be initialized before this function is called.
	SendError(error)
	// HandleConnection proxies the connection received from the proxy to
	// the particular database instance.
	HandleConnection(context.Context, *Session) error
}

// Users defines an interface for managing database users.
type Users interface {
	GetPassword(ctx context.Context, database types.Database, userName string) (string, error)
}

// AutoUsers defines an interface for automatic user provisioning
// a particular database engine should implement.
type AutoUsers interface {
	// ActivateUser creates or enables a database user.
	ActivateUser(context.Context, *Session) error
	// DeactivateUser disables a database user.
	DeactivateUser(context.Context, *Session) error
	// DeleteUser deletes the database user.
	DeleteUser(context.Context, *Session) error
}

// IngressReporter provides an interface for ingress.Report that tracks
// connection ingress metrics.
type IngressReporter interface {
	// ConnectionAccepted reports a new connection, ConnectionClosed must be
	// called when the connection closes.
	ConnectionAccepted(service string, conn net.Conn)
	// ConnectionClosed reports a closed connection. This should only be called
	// after ConnectionAccepted.
	ConnectionClosed(service string, conn net.Conn)
	// ConnectionAuthenticated reports a new authenticated connection,
	// AuthenticatedConnectionClosed must be called when the connection is
	// closed.
	ConnectionAuthenticated(service string, conn net.Conn)
	// AuthenticatedConnectionClosed reports a closed authenticated connection,
	// this should only be called after ConnectionAuthenticated.
	AuthenticatedConnectionClosed(service string, conn net.Conn)
}

// Limiter defines an interface for limiting database connections.
type Limiter interface {
	// RegisterClientIP applies connection and rate limiting by the client IP.
	// Returned release func must be called when the connection handling is
	// done.
	RegisterClientIP(conn net.Conn) (release func(), clientIP string, err error)
	// RegisterIdentity applies per user max connection based on role options.
	// Returned release func must be called when the connection handling is
	// done.
	RegisterIdentity(ctx context.Context, proxyCtx *ProxyContext) (release func(), err error)
}
