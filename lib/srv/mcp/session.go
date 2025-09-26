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
	"encoding/json"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
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

	// mcpSessionID is the MCP session ID tracked by remote MCP server.
	mcpSessionID atomicString

	// jwt is the jwt token signed for this identity by Auth server.
	jwt string

	// traitsForRewriteHeaders are user traits used for rewriting headers.
	traitsForRewriteHeaders wrappers.Traits
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
	if c.sessionID == "" {
		if types.MCPTransportHTTP == types.GetMCPServerTransportType(c.App.GetURI()) {
			// A single HTTP request is handled at a time so take session ID
			// from cert.
			c.sessionID = session.ID(c.Identity.RouteToApp.SessionID)
		}
		if c.sessionID == "" {
			c.sessionID = session.NewID()
		}
	}
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

func (c *SessionCtx) generateJWTAndTraits(ctx context.Context, auth AuthClient) (err error) {
	c.jwt, c.traitsForRewriteHeaders, err = appcommon.GenerateJWTAndTraits(ctx, &c.Identity, c.App, auth)
	return trace.Wrap(err)
}

type sessionHandlerConfig struct {
	*SessionCtx
	*sessionAuditor
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

func (s *sessionHandler) processClientNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) {
	s.emitNotificationEvent(ctx, notification, nil)
}

func (s *sessionHandler) onClientNotification(serverRequestWriter mcputils.MessageWriter) mcputils.HandleNotificationFunc {
	return func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
		s.processClientNotification(ctx, notification)
		return trace.Wrap(serverRequestWriter.WriteMessage(ctx, notification))
	}
}

func (s *sessionHandler) onClientRequest(clientResponseWriter, serverRequestWriter mcputils.MessageWriter) mcputils.HandleRequestFunc {
	return func(ctx context.Context, request *mcputils.JSONRPCRequest) error {
		msg, replyDirection := s.processClientRequest(ctx, request)
		if replyDirection == replyToClient {
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
		msgToClient := s.processServerResponse(ctx, response)
		return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msgToClient))
	}
}

type replyDirection bool

const (
	replyToClient replyDirection = true
	replyToServer replyDirection = false
)

func (s *sessionHandler) processClientRequest(ctx context.Context, req *mcputils.JSONRPCRequest) (mcp.JSONRPCMessage, replyDirection) {
	s.idTracker.PushRequest(req)
	reply, authErr := s.processClientRequestNoAudit(ctx, req)
	s.emitRequestEvent(ctx, req, authErr)

	// Not forwarding to server. Just send the auth error to client.
	if authErr != nil {
		return reply, replyToClient
	}
	return reply, replyToServer
}

func (s *sessionHandler) processClientRequestNoAudit(ctx context.Context, req *mcputils.JSONRPCRequest) (mcp.JSONRPCMessage, error) {
	s.idTracker.PushRequest(req)
	switch req.Method {
	case mcp.MethodToolsCall:
		methodName, _ := req.Params.GetName()
		if authErr := s.checkAccessToTool(ctx, methodName); authErr != nil {
			return makeToolAccessDeniedResponse(req, authErr), trace.Wrap(authErr)
		}
	}
	return req, nil
}

func (s *sessionHandler) processServerResponse(ctx context.Context, response *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	method, _ := s.idTracker.PopByID(response.ID)
	switch method {
	case mcp.MethodToolsList:
		return s.makeToolsCallResponse(ctx, response)
	}
	return response
}

func (s *sessionHandler) processServerNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) {
	s.logger.DebugContext(ctx, "Received server notification.", "method", notification.Method)
}

func (s *sessionHandler) makeToolsCallResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	// Nothing to do, likely an error response.
	if resp.Result == nil {
		return resp
	}

	var listResult mcp.ListToolsResult
	if err := json.Unmarshal(resp.Result, &listResult); err != nil {
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

func makeToolAccessDeniedResponse(msg *mcputils.JSONRPCRequest, authErr error) mcp.JSONRPCMessage {
	return mcp.NewJSONRPCError(
		msg.ID,
		mcp.INVALID_PARAMS,
		"RBAC is enforced by your Teleport roles. Contact your Teleport Admin for more details.",
		authErr,
	)
}

type atomicString struct {
	atomic.Pointer[string]
}

// String loads the atomic string value. If the point is nil, empty is returned.
func (s *atomicString) String() string {
	if loaded := s.Load(); loaded != nil {
		return *loaded
	}
	return ""
}
