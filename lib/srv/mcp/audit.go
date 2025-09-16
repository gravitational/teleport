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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type sessionAuditorConfig struct {
	emitter    apievents.Emitter
	logger     *slog.Logger
	hostID     string
	sessionCtx *SessionCtx
	// TODO(greedy52) add recording support.
	preparer events.SessionEventPreparer
}

func (c *sessionAuditorConfig) checkAndSetDefaults() error {
	if c.emitter == nil {
		return trace.BadParameter("missing emitter")
	}
	if c.hostID == "" {
		return trace.BadParameter("missing hostID")
	}
	if c.sessionCtx == nil {
		return trace.BadParameter("missing sessionCtx")
	}
	if c.preparer == nil {
		return trace.BadParameter("missing preparer")
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	return nil
}

// sessionAuditor handles audit events for a session.
type sessionAuditor struct {
	sessionAuditorConfig
}

func newSessionAuditor(cfg sessionAuditorConfig) (*sessionAuditor, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &sessionAuditor{
		sessionAuditorConfig: cfg,
	}, nil
}

func (a *sessionAuditor) shouldEmitEvent(method mcp.MCPMethod) bool {
	// Do not record discovery, ping calls.
	switch method {
	case mcp.MethodPing,
		mcp.MethodResourcesList,
		mcp.MethodResourcesTemplatesList,
		mcp.MethodPromptsList,
		mcp.MethodToolsList:
		return false
	default:
		return true
	}
}

func (a *sessionAuditor) emitStartEvent(ctx context.Context) {
	a.emitEvent(ctx, &apievents.MCPSessionStart{
		Metadata: a.makeEventMetadata(
			events.MCPSessionStartEvent,
			events.MCPSessionStartCode,
		),
		ServerMetadata:     a.makeServerMetadata(),
		SessionMetadata:    a.makeSessionMetadata(),
		UserMetadata:       a.makeUserMetadata(),
		ConnectionMetadata: a.makeConnectionMetadata(),
		AppMetadata:        a.makeAppMetadata(),
		McpSessionId:       a.sessionCtx.mcpSessionID.String(),
	})
}

func (a *sessionAuditor) emitEndEvent(ctx context.Context, err error) {
	event := &apievents.MCPSessionEnd{
		Metadata: a.makeEventMetadata(
			events.MCPSessionEndEvent,
			events.MCPSessionEndCode,
		),
		ServerMetadata:     a.makeServerMetadata(),
		SessionMetadata:    a.makeSessionMetadata(),
		UserMetadata:       a.makeUserMetadata(),
		ConnectionMetadata: a.makeConnectionMetadata(),
		AppMetadata:        a.makeAppMetadata(),
		Status: apievents.Status{
			Success: true,
		},
	}
	if err != nil {
		event.Metadata.Code = events.MCPSessionEndFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitNotificationEvent(ctx context.Context, msg *mcputils.JSONRPCNotification, err error) {
	if err == nil && !a.shouldEmitEvent(msg.Method) {
		return
	}
	event := &apievents.MCPSessionNotification{
		Metadata: a.makeEventMetadata(
			events.MCPSessionNotificationEvent,
			events.MCPSessionNotificationCode,
		),
		SessionMetadata: a.makeSessionMetadata(),
		UserMetadata:    a.makeUserMetadata(),
		AppMetadata:     a.makeAppMetadata(),
		Message: apievents.MCPJSONRPCMessage{
			JSONRPC: msg.JSONRPC,
			Method:  string(msg.Method),
			Params:  msg.Params.GetEventParams(),
		},
		Status: apievents.Status{
			Success: true,
		},
	}
	if err != nil {
		event.Metadata.Code = events.MCPSessionNotificationFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitRequestEvent(ctx context.Context, msg *mcputils.JSONRPCRequest, err error) {
	if err == nil && !a.shouldEmitEvent(msg.Method) {
		return
	}
	event := &apievents.MCPSessionRequest{
		Metadata: a.makeEventMetadata(
			events.MCPSessionRequestEvent,
			events.MCPSessionRequestCode,
		),
		SessionMetadata: a.makeSessionMetadata(),
		UserMetadata:    a.makeUserMetadata(),
		AppMetadata:     a.makeAppMetadata(),
		Status: apievents.Status{
			Success: true,
		},
		Message: apievents.MCPJSONRPCMessage{
			JSONRPC: msg.JSONRPC,
			Method:  string(msg.Method),
			ID:      msg.ID.String(),
			Params:  msg.Params.GetEventParams(),
		},
	}

	if err != nil {
		event.Metadata.Code = events.MCPSessionRequestFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	}
	a.emitEvent(ctx, event)
}

//nolint:unused //TODO(greedy52) remove nolint
func (a *sessionAuditor) emitListenSSEStreamEvent(ctx context.Context, err error) {
	event := &apievents.MCPSessionListenSSEStream{
		Metadata: a.makeEventMetadata(
			events.MCPSessionListenSSEStream,
			events.MCPSessionListenSSEStreamCode,
		),
		SessionMetadata: a.makeSessionMetadata(),
		UserMetadata:    a.makeUserMetadata(),
		AppMetadata:     a.makeAppMetadata(),
		Status: apievents.Status{
			Success: true,
		},
	}
	if err != nil {
		event.Metadata.Code = events.MCPSessionListenSSEStreamFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	}
	a.emitEvent(ctx, event)
}

//nolint:unused //TODO(greedy52) remove nolint
func (a *sessionAuditor) emitInvalidHTTPRequest(ctx context.Context, r *http.Request) {
	body, _ := utils.GetAndReplaceRequestBody(r)
	event := &apievents.MCPSessionInvalidHTTPRequest{
		Metadata: a.makeEventMetadata(
			events.MCPSessionInvalidHTTPRequest,
			events.MCPSessionInvalidHTTPRequestCode,
		),
		SessionMetadata: a.makeSessionMetadata(),
		UserMetadata:    a.makeUserMetadata(),
		AppMetadata:     a.makeAppMetadata(),
		Path:            r.URL.Path,
		Method:          r.Method,
		Body:            body,
		RawQuery:        r.URL.RawQuery,
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitEvent(ctx context.Context, event apievents.AuditEvent) {
	preparedEvent, err := a.preparer.PrepareSessionEvent(event)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to prepare event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
		return
	}
	if err := a.emitter.EmitAuditEvent(ctx, preparedEvent.GetAuditEvent()); err != nil {
		a.logger.ErrorContext(ctx, "Failed to emit audit event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
	}
}

func (a *sessionAuditor) makeEventMetadata(eventType, eventCode string) apievents.Metadata {
	return apievents.Metadata{
		Type:        eventType,
		Code:        eventCode,
		ClusterName: a.sessionCtx.Identity.RouteToApp.ClusterName,
	}
}

func (a *sessionAuditor) makeServerMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerID:        a.hostID,
		ServerNamespace: apidefaults.Namespace,
	}
}

func (a *sessionAuditor) makeConnectionMetadata() apievents.ConnectionMetadata {
	return apievents.ConnectionMetadata{
		RemoteAddr: a.sessionCtx.Identity.LoginIP,
	}
}

func (a *sessionAuditor) makeAppMetadata() apievents.AppMetadata {
	return apievents.AppMetadata{
		AppURI:  a.sessionCtx.App.GetURI(),
		AppName: a.sessionCtx.App.GetName(),
	}
}

func (a *sessionAuditor) makeSessionMetadata() apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        a.sessionCtx.sessionID.String(),
		WithMFA:          a.sessionCtx.Identity.MFAVerified,
		PrivateKeyPolicy: string(a.sessionCtx.Identity.PrivateKeyPolicy),
	}
}

func (a *sessionAuditor) makeUserMetadata() apievents.UserMetadata {
	return a.sessionCtx.Identity.GetUserMetadata()
}
