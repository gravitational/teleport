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

package postgres

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"github.com/jackc/pgproto3/v2"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/utils"
)

// Proxy proxies connections from Postgres clients to database services
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
	// Limiter limits database connections.
	Limiter common.Limiter
	// IngressReporter reports new and active connections.
	IngressReporter *ingress.Reporter
}

// HandleConnection accepts connection from a Postgres client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
	if p.IngressReporter != nil {
		p.IngressReporter.ConnectionAccepted(ingress.Postgres, clientConn)
		defer p.IngressReporter.ConnectionClosed(ingress.Postgres, clientConn)
	}

	startupMessage, tlsConn, backend, err := p.handleStartup(ctx, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := p.handleConnection(ctx, tlsConn, startupMessage); err != nil {
		if serr := backend.Send(toErrorResponse(err)); serr != nil {
			p.Log.WarnContext(ctx, "Failed to send error to backend.", "error", serr)
		}
		return trace.Wrap(err)
	}
	return nil
}

// handleConnection dials database service, sends the postgres startup
// message, and begins proxying the connection.
func (p *Proxy) handleConnection(ctx context.Context, clientConn utils.TLSConn, startupMessage pgproto3.FrontendMessage) error {
	// Apply connection and rate limiting.
	releaseConn, clientIP, err := p.Limiter.RegisterClientIP(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseConn()

	proxyCtx, err := p.Service.Authorize(ctx, clientConn, common.ConnectParams{
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
		p.IngressReporter.ConnectionAuthenticated(ingress.Postgres, clientConn)
		defer p.IngressReporter.AuthenticatedConnectionClosed(ingress.Postgres, clientConn)
	}

	serviceConn, err := p.Service.Connect(ctx, proxyCtx, clientConn.RemoteAddr(), clientConn.LocalAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()
	// Frontend acts as a client for the Postgres wire protocol.
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(serviceConn), serviceConn)
	// Pass the startup message along to the Teleport database server.
	err = frontend.Send(startupMessage)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.Service.Proxy(ctx, proxyCtx, clientConn, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// handleStartup handles the initial protocol exchange between the Postgres
// client (e.g. psql) and this proxy.
//
// Returns the startup message that contains initial connect parameters and
// the upgraded TLS connection.
func (p *Proxy) handleStartup(ctx context.Context, clientConn net.Conn) (pgproto3.FrontendMessage, utils.TLSConn, *pgproto3.Backend, error) {
	receivedSSLRequest := false
	receivedGSSEncRequest := false
	for {
		// Backend acts as a server for the Postgres wire protocol.
		backend := pgproto3.NewBackend(pgproto3.NewChunkReader(clientConn), clientConn)
		startupMessage, err := backend.ReceiveStartupMessage()
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}

		// We don't want to log the cancel request secret key, so we handle
		// this case separately.
		if m, ok := startupMessage.(*pgproto3.CancelRequest); ok {
			p.Log.DebugContext(ctx, "Received cancel request for pid.", "pid", m.ProcessID)
			tlsConn, ok := clientConn.(utils.TLSConn)
			if !ok {
				return nil, nil, nil, trace.BadParameter(
					"expected tls connection, got %T", clientConn)
			}
			return m, tlsConn, backend, nil
		}

		// When initiating an encrypted connection, psql will first check with
		// the server whether it supports TLS by sending an SSLRequest message.
		//
		// Once the server has indicated the support (by sending 'S' in reply),
		// it will send a StartupMessage with the connection parameters such as
		// user name, database name, etc.
		//
		// https://www.postgresql.org/docs/13/protocol-flow.html#id-1.10.5.7.11
		p.Log.DebugContext(ctx, "Received startup message.", "message", startupMessage)
		switch m := startupMessage.(type) {
		case *pgproto3.SSLRequest:
			if receivedSSLRequest {
				// avoid receiving the same request over and over.
				return nil, nil, nil, trace.BadParameter("received more than one SSLRequest")
			}
			receivedSSLRequest = true
			if p.TLSConfig == nil {
				// Send 'N' back to make the client connect without TLS. Happens
				// when client connects through the local TLS proxy.
				_, err := clientConn.Write([]byte("N"))
				if err != nil {
					return nil, nil, nil, trace.Wrap(err)
				}
			} else {
				// Send 'S' back to indicate TLS support to the client.
				_, err := clientConn.Write([]byte("S"))
				if err != nil {
					return nil, nil, nil, trace.Wrap(err)
				}
				// Upgrade the connection to TLS and wait for the next message
				// which should be of the StartupMessage type.
				clientConn = tls.Server(clientConn, p.TLSConfig)
			}
			// The client may then choose to either close the connection or issue
			// another request.
			// We loop again to receive another message.
			continue
		case *pgproto3.GSSEncRequest:
			if receivedGSSEncRequest {
				// avoid receiving the same request over and over.
				return nil, nil, nil, trace.BadParameter("received more than one GSSEncRequest")
			}
			receivedGSSEncRequest = true
			// Send 'N' back to make the client connect without GSS encryption.
			_, err := clientConn.Write([]byte("N"))
			if err != nil {
				return nil, nil, nil, trace.Wrap(err)
			}
			// The client may then choose to either close the connection or issue
			// another request.
			// We loop again to receive another message.
			continue
		case *pgproto3.StartupMessage:
			// TLS connection between the client and this proxy has been
			// established, just return the startup message.
			tlsConn, ok := clientConn.(utils.TLSConn)
			if !ok {
				return nil, nil, nil, trace.BadParameter(
					"expected tls connection, got %T", clientConn)
			}
			return m, tlsConn, backend, nil
		}
		return nil, nil, nil, trace.BadParameter(
			"unsupported startup message: %#v", startupMessage)
	}
}
