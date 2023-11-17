/*
Copyright 2022 Gravitational, Inc.

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

package sqlserver

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
)

// Proxy accepts connections from SQL Server clients, performs a Pre-Login
// handshake and then forwards the connection to the database service agent.
type Proxy struct {
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// Service is used to connect to a remote database service.
	Service common.Service
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection accepts connection from a SQL Server client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, proxyCtx *common.ProxyContext, conn net.Conn) error {
	conn, err := p.handlePreLogin(ctx, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	serviceConn, err := p.Service.Connect(ctx, proxyCtx, conn.RemoteAddr(), conn.LocalAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()

	err = p.Service.Proxy(ctx, proxyCtx, conn, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (p *Proxy) handlePreLogin(ctx context.Context, conn net.Conn) (net.Conn, error) {
	_, err := protocol.ReadPreLoginPacket(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = protocol.WritePreLoginResponse(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pre-Login is done, Login7 is handled by the agent.
	return conn, nil
}
