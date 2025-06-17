/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/services"
)

// AccessPoint defines functions that the MCP server requires from the caching
// client to the Auth Server.
type AccessPoint interface {
	services.AuthPreferenceGetter
	services.ClusterNameGetter
}

// ServerConfig is the config for the MCP forward server.
type ServerConfig struct {
	// Emitter is used for emitting audit events.
	Emitter apievents.Emitter
	// Log is the slog logger.
	Log *slog.Logger
	// ParentContext is parent's context for logging.
	ParentContext context.Context
	// HostID is the host ID of the teleport service.
	HostID string
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint AccessPoint

	clock          clockwork.Clock
	inMemoryServer bool
}

// CheckAndSetDefaults checks values and sets defaults
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.ParentContext == nil {
		return trace.BadParameter("missing ParentContext")
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, teleport.ComponentMCP)
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	c.inMemoryServer = os.Getenv(InMemoryServerEnvVar) == "true"
	return nil
}

// Server handles forwarding client connections to MCP servers.
// TODO(greedy52) add server metrics.
type Server struct {
	cfg ServerConfig
}

// NewServer creates a new Server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Server{
		cfg: cfg,
	}, nil
}

// HandleSession handles an authorized client connection.
func (s *Server) HandleSession(ctx context.Context, sessionCtx SessionCtx) error {
	if err := sessionCtx.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.cfg.inMemoryServer && isInMemoryServerApp(sessionCtx.App) {
		return trace.Wrap(s.handleInMemoryServerSession(ctx, sessionCtx))
	}
	// TODO(greedy52) handle stdio
	return trace.NotImplemented("not implemented")
}

// HandleUnauthorizedConnection handles an unauthorized client connection.
func (s *Server) HandleUnauthorizedConnection(ctx context.Context, clientConn net.Conn, authErr error) error {
	// TODO(greedy52) handle stdio
	return trace.NotImplemented("not implemented")
}
