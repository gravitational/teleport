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
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// SessionCtx contains basic information of an MCP session.
type SessionCtx struct {
	// ClientConn is the incoming client connection.
	ClientConn net.Conn
	// AuthCtx is the authorization context.
	AuthCtx *authz.Context
	// App is the MCP server application being accessed.
	App types.Application
	// Identity is the user identity.
	Identity tlsca.Identity

	// sessionID is the Teleport session ID.
	//
	// Note that for stdio-based MCP server, a new session ID is generated per
	// connection instead of using the web session ID from the app route.
	sessionID session.ID

	// transport is the transport type of the MCP server.
	transport string

	rewriteAuthDetails rewriteAuthDetails
}

func (c *SessionCtx) checkAndSetDefaults() error {
	if c.ClientConn == nil {
		return trace.BadParameter("missing ClientConn")
	}
	if c.AuthCtx == nil {
		return trace.BadParameter("missing AuthCtx")
	}
	if c.App == nil {
		return trace.BadParameter("missing App")
	}
	if c.Identity.Username == "" {
		c.Identity = c.AuthCtx.Identity.GetIdentity()
	}
	if c.transport == "" {
		c.transport = types.GetMCPServerTransportType(c.App.GetURI())
	}
	if c.sessionID == "" {
		if types.MCPTransportHTTP == c.transport {
			// A single HTTP request is handled at a time so take session ID
			// from cert.
			c.sessionID = session.ID(c.Identity.RouteToApp.SessionID)
		}
		if c.sessionID == "" {
			c.sessionID = session.NewID()
		}
	}
	c.rewriteAuthDetails = newRewriteAuthDetails(c.App.GetRewrite())
	return nil
}

func (c *SessionCtx) getAccessState(authPref types.AuthPreference) services.AccessState {
	state := c.AuthCtx.Checker.GetAccessState(authPref)
	state.MFAVerified = c.Identity.IsMFAVerified()
	state.EnableDeviceVerification = true
	state.DeviceVerified = dtauthz.IsTLSDeviceVerified(&c.Identity.DeviceExtensions)
	state.IsBot = c.Identity.IsBot()
	return state
}

type sessionHandlerConfig struct {
	*SessionCtx
	*sessionAuditor
	*sessionAuth
	accessPoint AccessPoint
	logger      *slog.Logger
	clock       clockwork.Clock
	parentCtx   context.Context
}

func (c *sessionHandlerConfig) checkAndSetDefaults() error {
	if c.SessionCtx == nil {
		return trace.BadParameter("missing session")
	}
	if c.sessionAuditor == nil {
		return trace.BadParameter("missing auditor")
	}
	if c.sessionAuth == nil {
		return trace.BadParameter("missing session auth")
	}
	if c.accessPoint == nil {
		return trace.BadParameter("missing accessPoint")
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.parentCtx == nil {
		c.parentCtx = context.Background()
	}
	return nil
}

// sessionHandler provides common functions for handling an MCP session,
// irrespective the transport type.
type sessionHandler struct {
	sessionHandlerConfig

	idTracker   *mcputils.IDTracker
	accessCache *utils.FnCache
}

func newSessionHandler(cfg sessionHandlerConfig) (*sessionHandler, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Usually we won't have more than a couple in flight messages but let's do
	// 50 just in case. Also, it's ok to lose the ID. The tracked ID is
	// currently used for tools/list filtering. In worst case we couldn't apply
	// this filtering, tools/call will still be blocked.
	idTracker, err := mcputils.NewIDTracker(50)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache access check like tool name for a small period of time.
	accessCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   time.Minute * 10,
		Clock: cfg.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Always begins with session start event for single-connection sessions.
	if cfg.sessionCtx.transport != types.MCPTransportHTTP {
		cfg.sessionAuditor.appendStartEvent(cfg.parentCtx)
	}

	return &sessionHandler{
		sessionHandlerConfig: cfg,
		idTracker:            idTracker,
		accessCache:          accessCache,
	}, nil
}

func (s *sessionHandler) checkAccessToTool(ctx context.Context, toolName string) error {
	authErr, err := utils.FnCacheGet(ctx, s.accessCache, toolName, func(ctx context.Context) (error, error) {
		authPref, err := s.accessPoint.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		matcher := &services.MCPToolMatcher{
			Name: toolName,
		}
		authErr := s.AuthCtx.Checker.CheckAccess(s.App, s.getAccessState(authPref), matcher)
		return trace.Wrap(authErr), nil
	})
	// Fails the check on either authErr or internal error.
	return trace.NewAggregate(authErr, err)
}

// NOTE: the onClientXXX/onServerXXX functions and the close function below
// provides helper to single-connection sessions and handles audit events.
// The streamable-HTTP server handler does not use these function but uses
// processClientXXX/processServerXXX functions directly instead and handles
// audit events outside the sessionHandler. Ideally we should do some
// refactoring to separate handler types.

func (s *sessionHandler) close() {
	s.sessionAuditor.flush(s.parentCtx)
	if s.sessionCtx.transport != types.MCPTransportHTTP {
		s.sessionAuditor.emitEndEvent(s.parentCtx)
	}
}

func (s *sessionHandler) onClientNotification(serverRequestWriter mcputils.MessageWriter) mcputils.HandleNotificationFunc {
	return func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
		s.emitNotificationEvent(ctx, notification)
		s.processClientNotification(ctx, notification)
		return trace.Wrap(serverRequestWriter.WriteMessage(ctx, notification))
	}
}

func (s *sessionHandler) onClientRequest(clientResponseWriter, serverRequestWriter mcputils.MessageWriter) mcputils.HandleRequestFunc {
	return func(ctx context.Context, request *mcputils.JSONRPCRequest) error {
		msg, authErr := s.processClientRequest(ctx, request)
		s.emitRequestEvent(ctx, request, eventWithError(authErr))
		if authErr != nil {
			return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msg))
		}
		return trace.Wrap(serverRequestWriter.WriteMessage(ctx, msg))
	}
}

func (s *sessionHandler) onServerNotification(clientResponseWriter mcputils.MessageWriter) mcputils.HandleNotificationFunc {
	return func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
		s.processServerNotification(ctx, notification)
		return trace.Wrap(clientResponseWriter.WriteMessage(ctx, notification))
	}
}

func (s *sessionHandler) onServerResponse(clientResponseWriter mcputils.MessageWriter) mcputils.HandleResponseFunc {
	return func(ctx context.Context, response *mcputils.JSONRPCResponse) error {
		method, msgToClient := s.processServerResponse(ctx, response)
		if method == mcputils.MethodInitialize {
			s.sessionAuditor.updatePendingSessionStartEventWithInitializeResult(ctx, response)
		}
		return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msgToClient))
	}
}

func (s *sessionHandler) processClientRequest(ctx context.Context, req *mcputils.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
	messagesFromClient.WithLabelValues(s.transport, "request", reportRequestMethod(req.Method)).Inc()

	// TODO(greedy52) add checks to ensure that the initialize request is the
	// first request coming in (with the exception of the ping).
	s.idTracker.PushRequest(req)
	switch req.Method {
	case mcputils.MethodToolsCall:
		methodName, _ := req.Params.GetName()
		if authErr := s.checkAccessToTool(ctx, methodName); authErr != nil {
			return makeToolAccessDeniedResponse(req, authErr), trace.Wrap(authErr)
		}
	}
	return req, nil
}

func (s *sessionHandler) processClientNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) {
	messagesFromClient.WithLabelValues(s.transport, "notification", reportNotificationMethod(notification.Method)).Inc()
}

func (s *sessionHandler) processServerResponse(ctx context.Context, response *mcputils.JSONRPCResponse) (string, mcp.JSONRPCMessage) {
	method, _ := s.idTracker.PopByID(response.ID)
	messagesFromServer.WithLabelValues(s.transport, "response", reportRequestMethod(method)).Inc()

	switch method {
	case mcputils.MethodToolsList:
		return method, s.makeToolsCallResponse(ctx, response)
	}
	return method, response
}

func (s *sessionHandler) processServerNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) {
	s.logger.DebugContext(ctx, "Received server notification.", "method", notification.Method)
	messagesFromServer.WithLabelValues(s.transport, "notification", reportNotificationMethod(notification.Method)).Inc()
}

func (s *sessionHandler) makeToolsCallResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	// Nothing to do, likely an error response.
	if resp.Result == nil {
		return resp
	}

	listResult, err := resp.GetListToolResult()
	if err != nil {
		return mcp.NewJSONRPCError(resp.ID, mcp.INTERNAL_ERROR, "failed to unmarshal tools/list response", err)
	}

	var allowed []mcp.Tool
	for _, tool := range listResult.Tools {
		if s.checkAccessToTool(ctx, tool.Name) == nil {
			allowed = append(allowed, tool)
		}
	}

	s.logger.DebugContext(ctx, "Received tools/list result", "received", len(listResult.Tools), "allowed", len(allowed))
	listResult.Tools = allowed

	return mcp.JSONRPCResponse{
		JSONRPC: resp.JSONRPC,
		ID:      resp.ID,
		Result:  listResult,
	}
}

func (s *sessionHandler) rewriteHTTPRequestHeaders(r *http.Request) error {
	jwt, traits, err := s.generateJWTAndTraits(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	// Add in JWT headers. By default, JWT is not put into "Authorization"
	// headers since the auth token can also come from the client and Teleport
	// just pass it through. If the remote MCP server does verify the auth token
	// signed by Teleport, the server can take the token from the
	// "teleport-jwt-assertion" header or use a rewrite setting to set the JWT
	// as "Bearer" in "Authorization".
	r.Header.Set(teleport.AppJWTHeader, jwt)
	// Add headers from rewrite configuration.
	rewriteHeaders := appcommon.AppRewriteHeaders(r.Context(), s.App.GetRewrite(), s.logger)
	services.RewriteHeadersAndApplyValueTraits(r, rewriteHeaders, traits, s.logger)
	return nil
}

func makeToolAccessDeniedResponse(msg *mcputils.JSONRPCRequest, authErr error) mcp.JSONRPCMessage {
	return mcp.NewJSONRPCError(
		msg.ID,
		mcp.INVALID_PARAMS,
		"RBAC is enforced by your Teleport roles. Contact your Teleport Admin for more details.",
		authErr,
	)
}
