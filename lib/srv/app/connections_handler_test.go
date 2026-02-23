/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	c := &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			ServiceComponent: teleport.ComponentApp,
		},
	}

	srv := c.newHTTPServer("test-cluster")

	// The HTTP server no longer wraps a limiter (limiting is applied at
	// the connection level in handleConnection). Verify that requests
	// are never rejected with 429 by the HTTP handler itself.
	for i := range 5 {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
			"request %d should not be rate-limited", i+1)
	}
}

// newTestHandler creates a ConnectionsHandler with a limiter for testing
// handleConnection. The handler is minimal - only the limiter wiring is
// functional; TLS and auth are not configured.
func newTestHandler(t *testing.T, cfg limiter.Config) *ConnectionsHandler {
	t.Helper()
	lim, err := limiter.NewLimiter(cfg)
	require.NoError(t, err)

	return &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			Clock:            clockwork.NewFakeClock(),
			ServiceComponent: teleport.ComponentApp,
		},
		closeContext: t.Context(),
		limiter:      lim,
		log:          slog.Default(),
	}
}

// newConnWithIP creates a net.Pipe-backed connection whose RemoteAddr
// returns the given IP and port. Close the returned client side when
// done; the server side is used by handleConnection.
func newConnWithIP(t *testing.T, ip string, port int) (server, client net.Conn) {
	t.Helper()
	server, client = net.Pipe()
	t.Cleanup(func() {
		server.Close()
		client.Close()
	})
	addr := &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
	server = utils.NewConnWithSrcAddr(server, addr)
	return server, client
}

func TestHandleConnection_MaxConnections(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	const clientIP = "10.0.0.1"

	// Pre-fill the limiter to capacity for this IP so the next
	// handleConnection call from the same IP is rejected.
	release, err := c.limiter.RegisterRequestAndConnection(clientIP)
	require.NoError(t, err)
	defer release()

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close() // prevent blocking on TLS handshake

	_, err = c.handleConnection(conn)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded error, got: %v", err)
}

func TestHandleConnection_MaxConnectionsRelease(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	const clientIP = "10.0.0.1"

	// Acquire and immediately release the slot so the limiter
	// has capacity when handleConnection runs.
	release, err := c.limiter.RegisterRequestAndConnection(clientIP)
	require.NoError(t, err)
	release()

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close()

	// handleConnection should pass the limiter and fail later
	// (at TLS handshake). The error must not be about limits.
	_, err = c.handleConnection(conn)
	require.Error(t, err)
	require.False(t, trace.IsLimitExceeded(err), "unexpected LimitExceeded error: %v", err)
}

func TestHandleConnection_RateLimiting(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{
		Rates: []limiter.Rate{{
			Period:  time.Minute,
			Average: 1,
			Burst:   1,
		}},
	})

	const clientIP = "10.0.0.1"

	// Consume the one allowed request so the next is rate-limited.
	require.NoError(t, c.limiter.RegisterRequest(clientIP))

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close()

	_, err := c.handleConnection(conn)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded error, got: %v", err)
}

func TestHandleConnection_PipeSkipsLimiter(t *testing.T) {
	t.Parallel()

	// net.Pipe connections (whose RemoteAddr cannot be parsed as
	// host:port) skip limiting entirely. Configure a real limit and
	// pre-fill it so that a normal connection would be rejected.
	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	release, err := c.limiter.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)
	defer release()

	server, client := net.Pipe()
	t.Cleanup(func() { server.Close() })
	client.Close()

	// handleConnection should bypass the limiter (which is full)
	// and fail later at TLS instead.
	_, err = c.handleConnection(server)
	require.Error(t, err)
	require.False(t, trace.IsLimitExceeded(err), "unexpected LimitExceeded error: %v", err)
}
