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

package sqlserver

import (
	"context"
	"log/slog"
	"net"

	"github.com/gravitational/trace"

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
	Log *slog.Logger
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
