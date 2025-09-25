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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

func (s *Server) handleStreamableHTTP(ctx context.Context, sessionCtx *SessionCtx) error {
	s.cfg.Log.InfoContext(ctx, "Handle streamable HTTP request")
	defer s.cfg.Log.InfoContext(ctx, "Handle streamable HTTP request completed")

	session, err := s.makeSessionHandler(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "setting up session handler")
	}

	transport, err := s.makeStreamableHTTPTransport(ctx, session)
	if err != nil {
		return trace.Wrap(err, "setting up streamable http transport")
	}

	delegate := reverseproxy.NewHeaderRewriter()
	proxy, err := reverseproxy.New(
		reverseproxy.WithFlushInterval(100*time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(session.logger),
		reverseproxy.WithRewriter(appcommon.NewHeaderRewriter(delegate)),
		reverseproxy.WithResponseModifier(func(resp *http.Response) error {
			if resp.Request != nil && resp.Request.Method == http.MethodDelete {
				// Nothing to modify here. we are exiting the session.
				return nil
			}
			return trace.Wrap(mcputils.ReplaceHTTPResponse(ctx, resp, newHTTPResponseReplacer(session)))
		}),
	)
	if err != nil {
		return trace.Wrap(err, "creating reverse proxy")
	}

	// Serve a single request.
	waitConn := utils.NewCloserConn(sessionCtx.ClientConn)
	listener := listenerutils.NewSingleUseListener(waitConn)
	if err = http.Serve(listener, proxy); err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	waitConn.Wait()
	return nil
}

func (s *Server) makeStreamableHTTPTransport(ctx context.Context, session *sessionHandler) (http.RoundTripper, error) {
	jwt, traits, err := appcommon.GenerateJWTAndTraits(ctx, &session.Identity, session.App, s.cfg.AuthClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetURI, err := url.Parse(session.App.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	targetURI.Scheme = strings.TrimPrefix(targetURI.Scheme, "mcp+")

	tr, err := s.makeHTTPTransport(session.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sessionTransport{
		sessionHandler: session,
		targetURI:      targetURI,
		tr:             tr,
		jwt:            jwt,
		traits:         traits,
	}, nil
}

type sessionTransport struct {
	*sessionHandler
	targetURI *url.URL
	tr        http.RoundTripper
	jwt       string
	traits    wrappers.Traits
}

func (t *sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.setExternalSessionID(r.Header)
	t.rewriteRequest(r)

	switch r.Method {
	case http.MethodDelete:
		return t.handleSessionEndRequest(r)
	case http.MethodGet:
		return t.handleListenSSEStreamRequest(r)
	case http.MethodPost:
		return t.handleMCPMessage(r)

	default:
		// Some clients like MCP inspector may send OPTIONS requests which are
		// not documented in the MCP spec ¯\_(ツ)_/¯.
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return &http.Response{
			Request:    r,
			StatusCode: http.StatusMethodNotAllowed,
		}, nil
	}
}

func (t *sessionTransport) handleSessionEndRequest(r *http.Request) (*http.Response, error) {
	resp, err := t.tr.RoundTrip(r)
	t.emitEndEvent(t.parentCtx, err)
	return resp, trace.Wrap(err)
}

func (t *sessionTransport) handleListenSSEStreamRequest(r *http.Request) (*http.Response, error) {
	resp, err := t.tr.RoundTrip(r)
	t.emitListenSSEStreamEvent(t.parentCtx, err)
	return resp, trace.Wrap(err)
}

func (t *sessionTransport) handleMCPMessage(r *http.Request) (*http.Response, error) {
	var baseMessage mcputils.BaseJSONRPCMessage
	if reqBody, err := utils.GetAndReplaceRequestBody(r); err != nil {
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return nil, trace.BadParameter(err.Error())
	} else if err := json.Unmarshal(reqBody, &baseMessage); err != nil {
		t.emitInvalidHTTPRequest(t.parentCtx, r)
		return nil, trace.BadParameter(err.Error())
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

	resp, err := t.tr.RoundTrip(r)
	if resp != nil {
		// Prefer session ID from server response if present. For example,
		// "initialize" request does not have an ID but the server response to it
		// will.
		t.setExternalSessionID(resp.Header)
	}

	// Take of audit events.
	switch {
	case baseMessage.IsRequest():
		mcpRequest := baseMessage.MakeRequest()
		// Only emit session start if "initialize" succeeded.
		if mcpRequest.Method == "initialize" && err == nil {
			t.emitStartEvent(t.parentCtx)
		}
		t.emitRequestEvent(t.parentCtx, mcpRequest, err)
	case baseMessage.IsNotification():
		t.emitNotificationEvent(t.parentCtx, baseMessage.MakeNotification(), err)
	}
	return resp, trace.Wrap(err)
}

func (t *sessionTransport) setExternalSessionID(header http.Header) {
	if id := header.Get("Mcp-Session-Id"); id != "" {
		t.sessionCtx.mcpSessionID.Store(&id)
	}
}

func (t *sessionTransport) rewriteRequest(r *http.Request) {
	r.URL.Scheme = t.targetURI.Scheme
	r.URL.Host = t.targetURI.Host

	// Defaults to the endpoint defined in the app if client is not providing it.
	if t.targetURI.Path != "" {
		r.URL.Path = t.targetURI.Path
	}

	// Add in JWT headers.
	r.Header.Set(teleport.AppJWTHeader, t.jwt)
	// Add headers from rewrite configuration.
	rewriteHeaders := appcommon.AppRewriteHeaders(r.Context(), t.App.GetRewrite(), t.logger)
	services.RewriteHeadersAndApplyValueTraits(r, rewriteHeaders, t.traits, t.logger)
}

func (t *sessionTransport) handleRequestAuthError(r *http.Request, mcpRequest *mcputils.JSONRPCRequest, errResp mcp.JSONRPCMessage, authErr error) (*http.Response, error) {
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

type httpResponseReplacer struct {
	*sessionHandler
}

func newHTTPResponseReplacer(sessionHandler *sessionHandler) *httpResponseReplacer {
	return &httpResponseReplacer{
		sessionHandler: sessionHandler,
	}
}

func (p *httpResponseReplacer) ProcessResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	return p.processServerResponse(ctx, resp)
}
func (p *httpResponseReplacer) ProcessNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) mcp.JSONRPCMessage {
	p.processServerNotification(ctx, notification)
	return notification
}
