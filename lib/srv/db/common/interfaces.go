/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/tlsca"
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
	Authorize(ctx context.Context, tlsConn *tls.Conn, params ConnectParams) (*ProxyContext, error)
	// Connect is used to connect to remote database server over reverse tunnel.
	Connect(ctx context.Context, proxyCtx *ProxyContext) (net.Conn, error)
	// Proxy starts proxying between client and service connections.
	Proxy(ctx context.Context, proxyCtx *ProxyContext, clientConn, serviceConn net.Conn) error
}

// ProxyContext contains parameters for a database session being proxied.
type ProxyContext struct {
	// Identity is the authorized client Identity.
	Identity tlsca.Identity
	// Cluster is the remote Cluster running the database server.
	Cluster reversetunnel.RemoteSite
	// Servers is a list of database Servers that proxy the requested database.
	Servers []types.DatabaseServer
	// AuthContext is a context of authenticated user.
	AuthContext *auth.Context
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
