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

package web

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
)

// ServerConfig provides dependencies required to create a [Server].
type ServerConfig struct {
	// Server serves the web api
	Server *http.Server
	// Handler web handler
	Handler *APIHandler
	// Log to write log messages
	Log *slog.Logger
	// ShutdownPollPeriod sets polling period for shutdown
	ShutdownPollPeriod time.Duration
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.Server == nil {
		return trace.BadParameter("missing required parameter Server")
	}

	if c.Handler == nil {
		return trace.BadParameter("missing required parameter Handler")
	}

	if c.ShutdownPollPeriod <= 0 {
		c.ShutdownPollPeriod = defaults.ShutdownPollPeriod
	}

	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, teleport.ComponentProxy)
	}

	return nil
}

// Server serves the web api.
type Server struct {
	cfg ServerConfig

	mu     sync.Mutex
	ln     net.Listener
	closed bool
}

// NewServer constructs a [Server] from the provided [ServerConfig].
func NewServer(cfg ServerConfig) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Server{
		cfg: cfg,
	}, nil
}

// Serve launches the configured [http.Server].
func (s *Server) Serve(l net.Listener) error {
	s.mu.Lock()
	s.ln = l
	closed := s.closed
	if closed {
		s.ln.Close()
	}
	s.mu.Unlock()
	if closed {
		return trace.Errorf("serve called on previously closed server")
	}
	return trace.Wrap(s.cfg.Server.Serve(l))
}

// Close immediately closes the [http.Server].
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	if s.ln != nil {
		s.ln.Close()
	}
	s.mu.Unlock()
	return trace.NewAggregate(s.cfg.Handler.Close(), s.cfg.Server.Close())
}

// HandleConnection handles connections from plain TCP applications.
func (s *Server) HandleConnection(ctx context.Context, conn net.Conn) error {
	return s.cfg.Handler.appHandler.HandleConnection(ctx, conn)
}

// Shutdown initiates graceful shutdown. The underlying [http.Server]
// is not shutdown until all active connections are terminated or
// the context times out. This is required because the [http.Server]
// does not attempt to close nor wait for hijacked connections such as
// WebSockets during Shutdown; which means that any open sessions in the
// web UI will not prevent the [http.Server] from shutting down.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	var err error
	s.closed = true
	if s.ln != nil {
		err = s.ln.Close()
	}
	s.mu.Unlock()

	activeConnections := s.cfg.Handler.handler.userConns.Load()
	if activeConnections == 0 {
		err := s.cfg.Server.Shutdown(ctx)
		return trace.NewAggregate(err, s.cfg.Handler.Close())
	}

	s.cfg.Log.InfoContext(ctx, "Shutdown: waiting for active connections to finish", "active_connection_count", activeConnections)
	lastReport := time.Time{}
	ticker := time.NewTicker(s.cfg.ShutdownPollPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			activeConnections = s.cfg.Handler.handler.userConns.Load()
			if activeConnections == 0 {
				err := s.cfg.Server.Shutdown(ctx)
				return trace.NewAggregate(err, s.cfg.Handler.Close())
			}
			if time.Since(lastReport) > 10*s.cfg.ShutdownPollPeriod {
				s.cfg.Log.InfoContext(ctx, "Shutdown: waiting for active connections to finish", "active_connection_count", activeConnections)
				lastReport = time.Now()
			}
		case <-ctx.Done():
			s.cfg.Log.InfoContext(ctx, "Context canceled wait, returning")
			return trace.ConnectionProblem(trace.NewAggregate(err, s.cfg.Handler.Close()), "context canceled")
		}
	}
}
