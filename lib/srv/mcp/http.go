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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

const (
	mcpSessionIDHeader = "Mcp-Session-Id"
)

func (s *Server) serveHTTPConn(ctx context.Context, conn net.Conn, handler http.Handler) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	waitConn := utils.NewWaitConn(conn)
	context.AfterFunc(ctx, func() { waitConn.Close() })

	httpServer := &http.Server{
		Handler:     handler,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	listener := listenerutils.NewSingleUseListener(waitConn)
	if err := httpServer.Serve(listener); err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	waitConn.Wait()
	return nil
}

func (s *Server) handleAuthErrHTTP(ctx context.Context, clientConn net.Conn, authErr error) error {
	return trace.Wrap(s.serveHTTPConn(ctx, clientConn, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		trace.WriteError(w, authErr)
	})))
}

func (s *Server) handleStreamableHTTP(ctx context.Context, sessionCtx *SessionCtx) error {
	session, err := s.getSessionHandlerWithJWT(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "setting up session handler")
	}

	transport, err := s.makeStreamableHTTPTransport(session)
	if err != nil {
		return trace.Wrap(err, "setting up streamable http transport")
	}

	session.logger.DebugContext(ctx, "Started handling HTTP request")
	defer session.logger.DebugContext(ctx, "Completed handling HTTP request")

	delegate := reverseproxy.NewHeaderRewriter()
	reverseProxy, err := reverseproxy.New(
		reverseproxy.WithFlushInterval(100*time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(session.logger),
		reverseproxy.WithRewriter(appcommon.NewHeaderRewriter(delegate)),
		reverseproxy.WithResponseModifier(func(resp *http.Response) error {
			if resp.Request != nil && resp.Request.Method == http.MethodDelete {
				// Nothing to modify here.
				return nil
			}
			return trace.Wrap(mcputils.ReplaceHTTPResponse(ctx, resp, newHTTPResponseReplacer(session)))
		}),
	)
	if err != nil {
		return trace.Wrap(err, "creating reverse proxy")
	}

	return trace.Wrap(s.serveHTTPConn(ctx, sessionCtx.ClientConn, reverseProxy))
}

func (s *Server) makeStreamableHTTPTransport(session *sessionHandler) (http.RoundTripper, error) {
	targetURI, err := url.Parse(session.App.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	targetURI.Scheme = strings.TrimPrefix(targetURI.Scheme, "mcp+")

	targetTransport, err := s.makeHTTPTransport(session.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &streamableHTTPTransport{
		sessionHandler:  session,
		targetURI:       targetURI,
		targetTransport: targetTransport,
	}, nil
}

type streamableHTTPTransport struct {
	*sessionHandler
	targetURI       *url.URL
	targetTransport http.RoundTripper
}

func (t *streamableHTTPTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.setExternalSessionID(r.Header)

	switch r.Method {
	case http.MethodDelete:
		return t.handleSessionEndRequest(r)
	case http.MethodGet:
		return t.handleListenSSEStreamRequest(r)
	case http.MethodPost:
		return t.handleMCPMessage(r)

	default:
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return &http.Response{
			Request:    r,
			StatusCode: http.StatusMethodNotAllowed,
		}, nil
	}
}

func (t *streamableHTTPTransport) setExternalSessionID(header http.Header) {
	if id := header.Get(mcpSessionIDHeader); id != "" {
		t.mcpSessionID.Store(&id)
	}
}

func (t *streamableHTTPTransport) rewriteRequest(r *http.Request) *http.Request {
	r = r.Clone(r.Context())
	r.URL.Scheme = t.targetURI.Scheme
	r.URL.Host = t.targetURI.Host

	// Defaults to the endpoint defined in the app if client is not providing it.
	// By spec, streamable HTTP should use a single endpoint except the
	// ".well-known" used for OAuth.
	// https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http
	if t.targetURI.Path != "" && (r.URL.Path == "" || r.URL.Path == "/") {
		r.URL.Path = t.targetURI.Path
	}

	// Add in JWT headers. By default, JWT is not put into "Authorization"
	// headers since the auth token can also come from the client and Teleport
	// just pass it through. If the remote MCP server does verify the auth token
	// signed by Teleport, the server can take the token from the
	// "teleport-jwt-assertion" header or use a rewrite setting to set the JWT
	// as "Bearer" in "Authorization".
	r.Header.Set(teleport.AppJWTHeader, t.jwt)
	// Add headers from rewrite configuration.
	rewriteHeaders := appcommon.AppRewriteHeaders(r.Context(), t.App.GetRewrite(), t.logger)
	services.RewriteHeadersAndApplyValueTraits(r, rewriteHeaders, t.traitsForRewriteHeaders, t.logger)
	return r
}

func (t *streamableHTTPTransport) rewriteAndSendRequest(r *http.Request) (*http.Response, error) {
	rCopy := t.rewriteRequest(r)
	return t.targetTransport.RoundTrip(rCopy)
}

func (t *streamableHTTPTransport) handleSessionEndRequest(r *http.Request) (*http.Response, error) {
	resp, err := t.rewriteAndSendRequest(r)
	t.emitEndEvent(t.parentCtx, convertHTTPResponseErrorForAudit(resp, err))
	return resp, trace.Wrap(err)
}

func (t *streamableHTTPTransport) handleListenSSEStreamRequest(r *http.Request) (*http.Response, error) {
	resp, err := t.rewriteAndSendRequest(r)
	t.emitListenSSEStreamEvent(t.parentCtx, convertHTTPResponseErrorForAudit(resp, err))
	return resp, trace.Wrap(err)
}

func (t *streamableHTTPTransport) handleMCPMessage(r *http.Request) (*http.Response, error) {
	var baseMessage mcputils.BaseJSONRPCMessage
	if reqBody, err := utils.GetAndReplaceRequestBody(r); err != nil {
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return nil, trace.BadParameter("invalid request body %v", err)
	} else if err := json.Unmarshal(reqBody, &baseMessage); err != nil {
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return nil, trace.BadParameter("invalid request body %v", err)
	}

	switch {
	case baseMessage.IsRequest():
		mcpRequest := baseMessage.MakeRequest()
		if errResp, authErr := t.sessionHandler.processClientRequestNoAudit(r.Context(), mcpRequest); authErr != nil {
			return t.handleRequestAuthError(r, mcpRequest, errResp, authErr)
		}
	case baseMessage.IsNotification():
		// nothing to do, yet.
	default:
		// Not sending it to the server if we don't understand it.
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return nil, trace.BadParameter("not a MCP request or notification")
	}

	resp, err := t.rewriteAndSendRequest(r)
	// Prefer session ID from server response if present. For example,
	// "initialize" request does not have an ID but the server response may have
	// it.
	if resp != nil {
		t.setExternalSessionID(resp.Header)
	}

	// Take care of audit events after round trip.
	respErrForAudit := convertHTTPResponseErrorForAudit(resp, err)
	switch {
	case baseMessage.IsRequest():
		mcpRequest := baseMessage.MakeRequest()
		// Only emit session start if "initialize" succeeded.
		if mcpRequest.Method == mcp.MethodInitialize && respErrForAudit == nil {
			t.emitStartEvent(t.parentCtx)
		}
		t.emitRequestEvent(t.parentCtx, mcpRequest, respErrForAudit)
	case baseMessage.IsNotification():
		t.emitNotificationEvent(t.parentCtx, baseMessage.MakeNotification(), respErrForAudit)
	}
	return resp, trace.Wrap(err)
}

func (t *streamableHTTPTransport) handleRequestAuthError(r *http.Request, mcpRequest *mcputils.JSONRPCRequest, errResp mcp.JSONRPCMessage, authErr error) (*http.Response, error) {
	t.emitRequestEvent(t.parentCtx, mcpRequest, authErr)

	errRespAsBody, err := json.Marshal(errResp)
	if err != nil {
		// Should not happen. If it does, we are failing the request either way.
		return nil, trace.Wrap(err)
	}

	httpResp := &http.Response{
		Request:    r,
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(errRespAsBody)),
		Header:     make(http.Header),
	}
	httpResp.Header.Set("Content-Type", "application/json")
	return httpResp, nil
}

func convertHTTPResponseErrorForAudit(resp *http.Response, err error) error {
	if err != nil {
		return trace.Wrap(err)
	}
	if resp == nil {
		// Should not happen.
		return trace.BadParameter("missing response")
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
		return nil
	}
	if resp.Status == "" {
		return trace.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return trace.Errorf("HTTP %s", resp.Status)
}

// streamableHTTPResponseReplacer is a wrapper of sessionHandler to satisfy
// mcputils.ServerMessageProcessor.
type streamableHTTPResponseReplacer struct {
	*sessionHandler
}

func newHTTPResponseReplacer(sessionHandler *sessionHandler) *streamableHTTPResponseReplacer {
	return &streamableHTTPResponseReplacer{
		sessionHandler: sessionHandler,
	}
}

func (p *streamableHTTPResponseReplacer) ProcessResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	return p.processServerResponse(ctx, resp)
}
func (p *streamableHTTPResponseReplacer) ProcessNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) mcp.JSONRPCMessage {
	p.processServerNotification(ctx, notification)
	return notification
}
