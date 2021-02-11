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
	"io"
	"net"
)

// Proxy defines an interface a database proxy should implement.
type Proxy interface {
	// HandleConnection takes the client connection, handles all database
	// specific startup actions and starts proxying to remote server.
	HandleConnection(context.Context, net.Conn) error
}

// Service defines an interface for connecting to a remote database service.
type Service interface {
	// Connect is used to connect to remote database server over reverse tunnel.
	Connect(ctx context.Context, user, database string) (net.Conn, error)
	// Proxy starts proxying between client and service connections.
	Proxy(ctx context.Context, clientConn, serviceConn io.ReadWriteCloser) error
}

// Engine defines an interface for specific database protocol engine such
// as Postgres or MySQL.
type Engine interface {
	// HandleConnection proxies the connection received from the proxy to
	// the particular database instance.
	HandleConnection(context.Context, *Session, net.Conn) error
}
