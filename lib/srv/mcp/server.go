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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessPoint defines functions that the MCP server requires from the caching
// client to the Auth Server.
type AccessPoint interface {
	services.AuthPreferenceGetter
	services.ClusterNameGetter
}

// AuthClient defines functions that the MCP server requires from the auth
// client.
type AuthClient interface {
	appcommon.AppTokenGenerator
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
	// AuthClient is a client directly connected to the Auth server.
	AuthClient AuthClient
	// EnableDemoServer enables the "Teleport Demo" MCP server.
	EnableDemoServer bool
	// CipherSuites is the list of TLS cipher suites that have been configured
	// for this process.
	CipherSuites []uint16

	clock clockwork.Clock
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
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if len(c.CipherSuites) == 0 {
		return trace.BadParameter("missing CipherSuites")
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, teleport.ComponentMCP)
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	return nil
}

// Server handles forwarding client connections to MCP servers.
// TODO(greedy52) add server metrics.
type Server struct {
	cfg ServerConfig

	sessionCache *utils.FnCache
}

// NewServer creates a new Server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         10 * time.Minute,
		Context:     cfg.ParentContext,
		Clock:       cfg.clock,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Server{
		cfg:          cfg,
		sessionCache: cache,
	}, nil
}

// HandleSession handles an authorized client connection.
func (s *Server) HandleSession(ctx context.Context, sessionCtx *SessionCtx) error {
	if err := sessionCtx.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.cfg.EnableDemoServer && isDemoServerApp(sessionCtx.App) {
		return trace.Wrap(s.handleStdio(ctx, sessionCtx, makeDemoServerRunner))
	}
	transportType := types.GetMCPServerTransportType(sessionCtx.App.GetURI())
	switch transportType {
	case types.MCPTransportStdio:
		return trace.Wrap(s.handleStdio(ctx, sessionCtx, makeExecServerRunner))
	case types.MCPTransportSSE:
		return trace.Wrap(s.handleStdioToSSE(ctx, sessionCtx))
	case types.MCPTransportHTTP:
		return trace.Wrap(s.handleStreamableHTTP(ctx, sessionCtx))
	default:
		return trace.BadParameter("unknown transport type: %v", transportType)
	}
}

// HandleUnauthorizedConnection handles an unauthorized client connection.
// This function has a hardcoded 30 seconds timeout in case the proper error
// message cannot be delivered to the client.
func (s *Server) HandleUnauthorizedConnection(ctx context.Context, clientConn net.Conn, app types.Application, authErr error) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	transportType := types.GetMCPServerTransportType(app.GetURI())
	switch transportType {
	case types.MCPTransportHTTP:
		return trace.Wrap(s.handleAuthErrHTTP(ctx, clientConn, authErr))
	default:
		return trace.Wrap(s.handleAuthErrStdio(ctx, clientConn, authErr))
	}
}

func (s *Server) makeSessionAuditor(ctx context.Context, sessionCtx *SessionCtx, logger *slog.Logger) (*sessionAuditor, error) {
	clusterName, err := s.cfg.AccessPoint.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	name := clusterName.GetClusterName()

	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sessionCtx.sessionID,
		ServerID:    s.cfg.HostID,
		Namespace:   apidefaults.Namespace,
		Clock:       s.cfg.clock,
		ClusterName: name,
		StartTime:   s.cfg.clock.Now(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newSessionAuditor(sessionAuditorConfig{
		emitter:    s.cfg.Emitter,
		logger:     logger,
		hostID:     s.cfg.HostID,
		preparer:   preparer,
		sessionCtx: sessionCtx,
	})
}

func (s *Server) makeSessionHandler(ctx context.Context, sessionCtx *SessionCtx) (*sessionHandler, error) {
	// Some extra info for debugging purpose.
	logger := s.cfg.Log.With(
		"client_ip", sessionCtx.ClientConn.RemoteAddr(),
		"app", sessionCtx.App.GetName(),
		"app_uri", sessionCtx.App.GetURI(),
		"user", sessionCtx.AuthCtx.User.GetName(),
		"session_id", sessionCtx.sessionID,
	)

	sessionAuditor, err := s.makeSessionAuditor(ctx, sessionCtx, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newSessionHandler(sessionHandlerConfig{
		SessionCtx:     sessionCtx,
		sessionAuditor: sessionAuditor,
		accessPoint:    s.cfg.AccessPoint,
		logger:         logger,
		parentCtx:      s.cfg.ParentContext,
		clock:          s.cfg.clock,
	})
}

func (s *Server) makeSessionHandlerWithJWT(ctx context.Context, sessionCtx *SessionCtx) (*sessionHandler, error) {
	if err := sessionCtx.generateJWTAndTraits(ctx, s.cfg.AuthClient); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.makeSessionHandler(ctx, sessionCtx)
}

func (s *Server) getSessionHandlerWithJWT(ctx context.Context, sessionCtx *SessionCtx) (*sessionHandler, error) {
	ttl := min(sessionCtx.Identity.Expires.Sub(s.cfg.clock.Now()), 10*time.Minute)
	return utils.FnCacheGetWithTTL(ctx, s.sessionCache, sessionCtx.sessionID, ttl, func(ctx context.Context) (*sessionHandler, error) {
		return s.makeSessionHandlerWithJWT(ctx, sessionCtx)
	})
}
