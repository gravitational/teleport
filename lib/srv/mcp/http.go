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
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type httpResponseProcessor struct {
	*sessionHandler
}

func newHTTPResponseProcessor(sessionHandler *sessionHandler) *httpResponseProcessor {
	return &httpResponseProcessor{
		sessionHandler: sessionHandler,
	}
}

func (p *httpResponseProcessor) ProcessResponse(ctx context.Context, resp *mcputils.JSONRPCResponse) mcp.JSONRPCMessage {
	return p.processServerResponse(ctx, resp)
}
func (p *httpResponseProcessor) ProcessNotification(ctx context.Context, notification *mcputils.JSONRPCNotification) mcp.JSONRPCMessage {
	p.logger.DebugContext(ctx, "Processing server notification.", "method", notification.Method)
	return notification
}

func (s *Server) handleStreamableHTTP(ctx context.Context, sessionCtx *SessionCtx) error {
	s.cfg.Log.InfoContext(ctx, "Handle streamable HTTP request")
	defer s.cfg.Log.InfoContext(ctx, "Handle streamable HTTP request completed")

	// TODO(greedy52) cache session similar to how app access handles chunks for
	// recording purpose.
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
			return trace.Wrap(mcputils.ReplaceHTTPResponse(ctx, resp, newHTTPResponseProcessor(session)))
		}),
	)
	if err != nil {
		return trace.Wrap(err, "creating reverse proxy")
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			proxy.ServeHTTP(w, req)
		}),
	}
	// Single use listener returns io.EOF on the second Accept so Serve here
	// returns once the connection is passed to the handler in a go-routine.
	if err = server.Serve(
		listenerutils.NewSingleUseListener(sessionCtx.ClientConn),
	); err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	// Graceful shutdown to wait for the request to be processed.
	if err := server.Shutdown(ctx); err != nil && !utils.IsOKNetworkError(err) && !errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}
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

	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO configure TLS
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

func (t *sessionTransport) setExternalSessionID(id string) {
	if id == "" {
		return
	}

	// Store the external session ID.
	t.sessionCtx.mcpSessionID.Store(&id)

	// Use the external session ID for our session ID, if it's already a UUID.
	// If not, do a UUID hash.
	if parsedID, err := uuid.Parse(id); err == nil {
		t.sessionID = session.ID(parsedID.String())
	} else {
		t.sessionID = session.ID(uuid.NewSHA1(uuid.Nil, []byte(id)).String())

	}
}

func (t *sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.setExternalSessionID(r.Header.Get("Mcp-Session-Id"))

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

	reqBody, err := utils.GetAndReplaceRequestBody(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var initReq *mcputils.JSONRPCRequest
	switch {
	case r.Method == http.MethodDelete:
		t.emitEndEvent(t.parentCtx)
		return t.tr.RoundTrip(r)
	case len(reqBody) > 0:
		var baseMessage mcputils.BaseJSONRPCMessage
		if err := json.Unmarshal(reqBody, &baseMessage); err != nil {
			return nil, trace.Wrap(err)
		}
		switch {
		case baseMessage.IsRequest():
			req := baseMessage.MakeRequest()
			switch req.Method {
			case "initialize":
				// TODO(greedy52) handle this in a more automatic way
				initReq = req
			default:
				errResp, replyDir := t.sessionHandler.processClientRequest(r.Context(), req)
				errRespAsBody, err := json.Marshal(errResp)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				if replyDir == replyToClient {
					t.logger.WarnContext(r.Context(), "=== blocking request", "accept", r.Header.Get("accept"))
					httpResp := &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(errRespAsBody)),
						Header:     make(http.Header),
					}
					httpResp.Header.Set("Content-Type", "application/json")
					httpResp.Header.Set("Mcp-Session-Id", r.Header.Get("Mcp-Session-Id"))
					return httpResp, nil
				}
			}
		case baseMessage.IsNotification():
			t.sessionHandler.processClientNotification(r.Context(), baseMessage.MakeNotification())
		default:
			return nil, trace.BadParameter("todo something went wrong")
		}
	}

	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		// TODO(greedy52) emit start failure?
		return nil, trace.Wrap(err)
	}
	if initReq != nil {
		t.setExternalSessionID(resp.Header.Get("Mcp-Session-Id"))
		t.emitStartEvent(t.parentCtx)
		t.emitRequestEvent(t.parentCtx, initReq, nil)
	}
	return resp, nil
}
