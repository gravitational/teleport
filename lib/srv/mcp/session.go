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
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type SessionCtx struct {
	ClientConn net.Conn
	AuthCtx    *authz.Context
	App        types.Application
	Identity   tlsca.Identity
	sessionID  session.ID
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
		// Do not use web session ID from the app route.
		c.sessionID = session.NewID()
	}
	return nil
}

type sessionHandler struct {
	SessionCtx
	*auditor

	accessPoint AccessPoint
	idTracker   *mcputils.IDTracker
	accessCache *utils.FnCache
	logger      *slog.Logger
}

func (s *Server) newSessionHandler(ctx context.Context, sessionCtx SessionCtx) (*sessionHandler, error) {
	// Usually we wouldn't need more than a few but let's do 50 just in case.
	// Also, it's ok to lose the ID. The tracked ID is currently used for
	// tools/list filtering. In rare cases we couldn't apply this filtering,
	// tools/call will still be blocked.
	idTracker, err := mcputils.NewIDTracker(50)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   time.Minute * 5,
		Clock: s.cfg.clock,
	})

	auditor, err := s.newAuditor(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sessionHandler{
		SessionCtx:  sessionCtx,
		auditor:     auditor,
		idTracker:   idTracker,
		accessPoint: s.cfg.AccessPoint,
		accessCache: accessCache,
		// Some extra info for debugging purpose.
		logger: s.cfg.Log.With(
			"client_ip", sessionCtx.ClientConn.RemoteAddr(),
			"app", sessionCtx.App.GetName(),
			"user", sessionCtx.AuthCtx.User.GetName(),
		),
	}, nil
}

func (s *sessionHandler) checkAccessToTool(ctx context.Context, toolName string) error {
	authErr, err := utils.FnCacheGet(ctx, s.accessCache, toolName, func(ctx context.Context) (error, error) {
		accessState, err := s.getAccessState(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		matcher := &services.MCPToolMatcher{
			Name: toolName,
		}
		authErr := s.AuthCtx.Checker.CheckAccess(s.App, accessState, matcher)
		return trace.Wrap(authErr), nil
	})
	return trace.NewAggregate(authErr, err)
}

func (s *sessionHandler) getAccessState(ctx context.Context) (services.AccessState, error) {
	authPref, err := s.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		return services.AccessState{}, trace.Wrap(err)
	}
	state := s.AuthCtx.Checker.GetAccessState(authPref)
	state.MFAVerified = s.Identity.IsMFAVerified()
	state.EnableDeviceVerification = true
	state.DeviceVerified = dtauthz.IsTLSDeviceVerified(&s.Identity.DeviceExtensions)
	return state, nil
}

type replyDirection bool

const (
	replyToClient replyDirection = true
	replyToServer replyDirection = false
)

func (s *sessionHandler) processClientNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) {
	s.auditor.emitNotificationEvent(ctx, &s.SessionCtx, notification)
}

func (s *sessionHandler) processClientRequest(ctx context.Context, req *mcputils.JSONRPCRequest) (mcp.JSONRPCMessage, replyDirection) {
	s.idTracker.PushRequest(req)
	switch {
	case req.Method == mcp.MethodToolsCall:
		methodName, _ := req.Params.GetName()
		if authErr := s.checkAccessToTool(ctx, methodName); authErr != nil {
			s.auditor.emitRequestEvent(ctx, &s.SessionCtx, req, authErr)
			return makeToolAccessDeniedResponse(req, authErr), replyToClient
		}
	}
	s.auditor.emitRequestEvent(ctx, &s.SessionCtx, req, nil)
	return req, replyToServer
}

func (s *sessionHandler) processServerResponse(ctx context.Context, response *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	method, _ := s.idTracker.PopByID(response.ID)
	switch method {
	case mcp.MethodToolsList:
		return s.makeToolsCallResponse(ctx, response)
	}
	return response
}

func (s *sessionHandler) makeToolsCallResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	// Likely an error response.
	if resp.Result == nil {
		return resp
	}

	var listResult mcp.ListToolsResult
	if err := json.Unmarshal(resp.Result, &listResult); err != nil {
		return mcp.NewJSONRPCError(resp.ID, mcp.PARSE_ERROR, "failed to unmarshal tools/list response", err)
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

func makeToolAccessDeniedResponse(msg *mcputils.JSONRPCRequest, authErr error) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      msg.ID,
		Result: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error: %v. RBAC is enforced by your Teleport roles. Contact your Teleport Adminstrators for more details.", authErr),
			}},
			IsError: false,
		},
	}
}
