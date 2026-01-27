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
	"net/http"
	"sync"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/events"
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

	// pendingSessionStartEvent is used to delay sending the session start event
	// until more metadata is received.
	pendingSessionStartEvent *apievents.MCPSessionStart
	mu                       sync.Mutex
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

func (a *sessionAuditor) appendStartEvent(ctx context.Context, options ...eventOptionFunc) {
	opts := newEventOptions(options...)

	// Prepare it to have the correct index.
	preparedEvent, err := a.preparer.PrepareSessionEvent(&apievents.MCPSessionStart{
		Metadata: a.makeEventMetadata(
			events.MCPSessionStartEvent,
			events.MCPSessionStartCode,
		),
		ServerMetadata:     a.makeServerMetadata(),
		SessionMetadata:    a.makeSessionMetadata(),
		UserMetadata:       a.makeUserMetadata(),
		ConnectionMetadata: a.makeConnectionMetadata(),
		AppMetadata:        a.makeAppMetadata(),
		EgressAuthType:     guessEgressAuthType(opts.header, a.sessionCtx.rewriteAuthDetails),
	})
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to prepare session start event", "error", err)
		return
	}
	event, ok := preparedEvent.GetAuditEvent().(*apievents.MCPSessionStart)
	if !ok {
		a.logger.ErrorContext(ctx, "failed to get session start event from prepared event")
		return
	}
	a.mu.Lock()
	a.pendingSessionStartEvent = event
	a.mu.Unlock()
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
	a.flushAndEmitEvent(ctx, event)
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
	a.flushAndEmitEvent(ctx, event)
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

	// Initialize should be the first request. Let's not flush session start
	// event yet but wait for the initialize result. Flush if request is
	// anything else to avoid delaying.
	if msg.Method == mcputils.MethodInitialize {
		a.updatePendingSessionStartEventWithInitializeRequest(msg)
		a.emitEvent(ctx, event)
	} else {
		a.flushAndEmitEvent(ctx, event)
	}
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
	a.flushAndEmitEvent(ctx, event)
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
	a.flushAndEmitEvent(ctx, event)
}

func (a *sessionAuditor) flush(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingSessionStartEvent != nil {
		if err := a.emitter.EmitAuditEvent(ctx, a.pendingSessionStartEvent); err != nil {
			a.logger.ErrorContext(ctx, "failed to emit session start event", "error", err)
		}
		a.pendingSessionStartEvent = nil
	}
}

func (a *sessionAuditor) flushAndEmitEvent(ctx context.Context, event apievents.AuditEvent) {
	a.flush(ctx)
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

func (a *sessionAuditor) updatePendingSessionStartEvent(fn func(*apievents.MCPSessionStart)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingSessionStartEvent != nil {
		fn(a.pendingSessionStartEvent)
	}
}

func (a *sessionAuditor) updatePendingSessionStartEventWithInitializeRequest(msg *mcputils.JSONRPCRequest) {
	// TODO(greedy52) avoid the Marshal when migrating to official SDK.
	paramsData, err := json.Marshal(msg.Params)
	if err != nil {
		return
	}
	var params mcp.InitializeParams
	if err := mcputils.UnmarshalJSONRPCMessage(paramsData, &params); err != nil {
		return
	}
	a.updatePendingSessionStartEvent(func(sessionStartEvent *apievents.MCPSessionStart) {
		sessionStartEvent.ProtocolVersion = params.ProtocolVersion
		sessionStartEvent.ClientInfo = fmt.Sprintf("%s/%s", params.ClientInfo.Name, params.ClientInfo.Version)
	})
}

func (a *sessionAuditor) updatePendingSessionStartEventWithInitializeResult(ctx context.Context, resp *mcputils.JSONRPCResponse) {
	if initResult, err := resp.GetInitializeResult(); err == nil && initResult != nil {
		a.updatePendingSessionStartEvent(func(sessionStartEvent *apievents.MCPSessionStart) {
			sessionStartEvent.ServerInfo = fmt.Sprintf("%s/%s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)
		})
	}

	// We can flush now as we receive the result.
	a.flush(ctx)
}

func (a *sessionAuditor) updatePendingSessionStartEventWithExternalSessionID(sessionID string) {
	a.updatePendingSessionStartEvent(func(event *apievents.MCPSessionStart) {
		event.McpSessionId = sessionID
	})
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
func guessEgressAuthType(headerWithoutRewrite http.Header, rewriteDetails rewriteAuthDetails) string {
	switch {
	case rewriteDetails.hasIDTokenTrait:
		return egressAuthTypeAppIDToken
	case rewriteDetails.hasJWTTrait:
		return egressAuthTypeAppJWT
	case rewriteDetails.rewriteAuthHeader:
		return egressAuthTypeAppDefined
	}

	// Reach here when app.Rewrite not overwriting auth. Check if Auth header is
	// defined by the user.
	if headerWithoutRewrite.Get("Authorization") != "" {
		return egressAuthTypeUserDefined
	}

	return egressAuthTypeUnknown
}

const (
	// egressAuthTypeUnknown is reported when no egress auth method can be
	// determined.
	egressAuthTypeUnknown = "unknown"
	// egressAuthTypeUserDefined is reported when the auth header is set by the
	// user and forwarded from the client.
	egressAuthTypeUserDefined = "user-defined"
	// egressAuthTypeAppJWT is reported when "{{internal.jwt}}" is defined in app
	// rewrite rules.
	egressAuthTypeAppJWT = "app-jwt"
	// egressAuthTypeAppIDToken is reported when "{{internal.id_token}}" is
	// defined in app rewrite rules.
	egressAuthTypeAppIDToken = "app-id-token"
	// egressAuthTypeAppDefined is reported when static credentials are defined in
	// app rewrite rules.
	egressAuthTypeAppDefined = "app-defined"
)
