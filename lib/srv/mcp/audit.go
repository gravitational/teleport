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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
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

type eventOptions struct {
	err    error
	header http.Header
}

type eventOptionFunc func(*eventOptions)

func newEventOptions(options ...eventOptionFunc) (opt eventOptions) {
	for _, fn := range options {
		if fn != nil {
			fn(&opt)
		}
	}
	return
}

func eventWithError(err error) eventOptionFunc {
	return func(o *eventOptions) {
		o.err = err
	}
}

func eventWithHTTPResponseError(resp *http.Response, err error) eventOptionFunc {
	return eventWithError(convertHTTPResponseErrorForAudit(resp, err))
}

func eventWithHeader(header http.Header) eventOptionFunc {
	return func(o *eventOptions) {
		o.header = headersForAudit(header)
	}
}

func (a *sessionAuditor) shouldEmitEvent(method string) bool {
	// Do not record discovery, ping calls.
	switch method {
	case mcputils.MethodPing,
		mcputils.MethodResourcesList,
		mcputils.MethodResourcesTemplatesList,
		mcputils.MethodPromptsList,
		mcputils.MethodToolsList:
		return false
	default:
		return true
	}
}

func (a *sessionAuditor) emitStartEvent(ctx context.Context, options ...eventOptionFunc) {
	opts := newEventOptions(options...)

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
		EgressAuthType:     guessEgressAuthType(opts.header, a.sessionCtx.App.GetRewrite()),
	})
}

func (a *sessionAuditor) emitEndEvent(ctx context.Context, options ...eventOptionFunc) {
	opts := newEventOptions(options...)

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
		Headers: wrappers.Traits(opts.header),
	}

	if opts.err != nil {
		event.Metadata.Code = events.MCPSessionEndFailureCode
		event.Status.Success = false
		event.Status.Error = opts.err.Error()
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitNotificationEvent(ctx context.Context, msg *mcputils.JSONRPCNotification, options ...eventOptionFunc) {
	opts := newEventOptions(options...)
	if opts.err == nil && !a.shouldEmitEvent(msg.Method) {
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
			Method:  msg.Method,
			Params:  msg.Params.GetEventParams(),
		},
		Status: apievents.Status{
			Success: true,
		},
		Headers: wrappers.Traits(opts.header),
	}
	if opts.err != nil {
		event.Metadata.Code = events.MCPSessionNotificationFailureCode
		event.Status.Success = false
		event.Status.Error = opts.err.Error()
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitRequestEvent(ctx context.Context, msg *mcputils.JSONRPCRequest, options ...eventOptionFunc) {
	opts := newEventOptions(options...)
	if opts.err == nil && !a.shouldEmitEvent(msg.Method) {
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
			Method:  msg.Method,
			ID:      msg.ID.String(),
			Params:  msg.Params.GetEventParams(),
		},
		Headers: wrappers.Traits(opts.header),
	}

	if opts.err != nil {
		event.Metadata.Code = events.MCPSessionRequestFailureCode
		event.Status.Success = false
		event.Status.Error = opts.err.Error()
	}
	a.emitEvent(ctx, event)
}

func (a *sessionAuditor) emitListenSSEStreamEvent(ctx context.Context, options ...eventOptionFunc) {
	opts := newEventOptions(options...)
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
		Headers: wrappers.Traits(opts.header),
	}
	if opts.err != nil {
		event.Metadata.Code = events.MCPSessionListenSSEStreamFailureCode
		event.Status.Success = false
		event.Status.Error = opts.err.Error()
	}
	a.emitEvent(ctx, event)
}

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
		Headers:         wrappers.Traits(r.Header),
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

var headersWithSecret = []string{
	"Authorization",
	"X-API-Key",
}

func headersForAudit(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	ret := h.Clone()
	for _, key := range appcommon.ReservedHeaders {
		ret.Del(key)
	}
	for _, key := range headersWithSecret {
		if len(ret.Values(key)) > 0 {
			ret.Set(key, "<REDACTED>")
		}
	}
	return ret
}

// guessEgressAuthType makes an educated guess on what kind of auth is used to
// for the remote MCP server.
func guessEgressAuthType(headerWithoutRewrite http.Header, rewrite *types.Rewrite) string {
	if rewrite != nil {
		testJWTTraits := map[string][]string{
			"jwt": {"test", "jwt"},
		}

		var rewriteAuth bool
		for _, rewrite := range rewrite.Headers {
			if strings.EqualFold(rewrite.Name, "Authorization") {
				rewriteAuth = true
			}

			// Check if any header value includes "{{internal.jwt}}".
			if strings.Contains(rewrite.Value, "internal.jwt") {
				// Apply fake traits just to be sure. The fake traits will
				// result two values if applied successfully.
				if interpolated, _ := services.ApplyValueTraits(rewrite.Value, testJWTTraits); len(interpolated) > 1 {
					return "app-jwt"
				}
			}
		}

		// Auth header has be defined in the app definition but not using
		// "{{internal.jwt}}".
		if rewriteAuth {
			return "app-defined"
		}
	}

	// Reach here when app.Rewrite not overwriting auth. Check if Auth header is
	// defined by the user.
	if headerWithoutRewrite.Get("Authorization") != "" {
		return "user-defined"
	}

	// No auth required for the remote MCP server or something we don't
	// understand yet.
	return "unknown"
}
