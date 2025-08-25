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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// handleStdioToSSE proxies a stdio client connection to an SSE server.
func (s *Server) handleStdioToSSE(ctx context.Context, sessionCtx *SessionCtx) error {
	// The incoming context can be canceled by connection monitor.
	// We create an extra cancel here to force stopping the handling of this
	// connection, when either the client or the server reader finishes reading
	// the stream.
	ctx, stopHandling := context.WithCancel(ctx)
	defer stopHandling()

	baseURL, err := makeSSEBaseURI(sessionCtx.App)
	if err != nil {
		return trace.Wrap(err, "parsing SSE URI")
	}
	httpTransport, err := s.makeHTTPTransport(sessionCtx.App)
	if err != nil {
		return trace.Wrap(err, "creating HTTP transport")
	}
	session, err := s.makeSessionHandler(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "setting up session handler")
	}

	session.logger.InfoContext(ctx, "Started handling stdio to SSE session", "base_url", logutils.StringerAttr(baseURL))
	defer session.logger.InfoContext(ctx, "Completed handling stdio to SSE session")

	// Initialize SSE stream.
	sseResponseReader, sseRequestWriter, err := mcputils.ConnectSSEServer(ctx, baseURL, httpTransport)
	if err != nil {
		return trace.Wrap(err)
	}
	session.logger.DebugContext(ctx, "Received SSE endpoint", "endpoint_url", sseRequestWriter.GetEndpointURL())
	if mcpSessionID := sseRequestWriter.GetSessionID(); mcpSessionID != "" {
		session.mcpSessionID.Store(&mcpSessionID)
	}

	// Setup proxy. The SSE stream and the stdio client connection should
	// maintain the same life cycle from this point.
	stdoutLogger := session.logger.With("sse", "stdout")
	clientResponseWriter := mcputils.NewSyncStdioMessageWriter(sessionCtx.ClientConn)
	serverResponseReader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport:      sseResponseReader,
		Logger:         stdoutLogger,
		OnClose:        stopHandling,
		OnParseError:   mcputils.LogAndIgnoreParseError(stdoutLogger),
		OnNotification: session.onServerNotification(clientResponseWriter),
		OnResponse:     session.onServerResponse(clientResponseWriter),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go serverResponseReader.Run(ctx)

	clientRequestReader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport:      mcputils.NewStdioReader(sessionCtx.ClientConn),
		Logger:         session.logger.With("stdio", "stdin"),
		OnClose:        stopHandling,
		OnParseError:   mcputils.ReplyParseError(clientResponseWriter),
		OnRequest:      session.onClientRequest(clientResponseWriter, sseRequestWriter),
		OnNotification: session.onClientNotification(sseRequestWriter),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) capture client info then emit start event with client
	// information.
	session.emitStartEvent(s.cfg.ParentContext)
	defer session.emitEndEvent(s.cfg.ParentContext)

	// Wait until reader finishes.
	clientRequestReader.Run(ctx)
	return nil
}

func makeSSEBaseURI(app types.Application) (*url.URL, error) {
	baseURL, err := url.Parse(app.GetURI())
	if err != nil {
		return nil, trace.Wrap(err, "parsing SSE URI")
	}
	switch {
	case strings.HasPrefix(app.GetURI(), types.SchemeMCPSSEHTTP):
		baseURL.Scheme = "http"
	case strings.HasPrefix(app.GetURI(), types.SchemeMCPSSEHTTPS):
		baseURL.Scheme = "https"
	default:
		return nil, trace.BadParameter("unknown scheme type: %v", baseURL.Scheme)
	}
	return baseURL, nil
}

func (s *Server) makeHTTPTransport(app types.Application) (http.RoundTripper, error) {
	// Use similar settings from lib/srv/app/transport.go.
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add a timeout to control how long it takes to (start) getting a response
	// from the target server.
	tr.ResponseHeaderTimeout = time.Minute
	tr.TLSClientConfig = utils.TLSConfig(s.cfg.CipherSuites)
	tr.TLSClientConfig.InsecureSkipVerify = lib.IsInsecureDevMode() || app.GetInsecureSkipVerify()
	return tr, nil
}
