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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

func (s *Server) handleStdioToSSE(ctx context.Context, sessionCtx SessionCtx) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	session, err := s.makeSessionHandler(ctx, &sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	session.logger.DebugContext(ctx, "Started handling stdio to SSE session")
	defer session.logger.DebugContext(s.cfg.ParentContext, "Completed handling stdio to SSE session")

	baseURI, err := url.Parse(session.App.GetURI())
	if err != nil {
		return trace.Wrap(err, "parsing SSE URI")
	}
	baseURI.Scheme = "http"

	req, err := http.NewRequestWithContext(ctx, "GET", baseURI.String(), nil)
	if err != nil {
		return trace.Wrap(err, "building SSE request")
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// TODO(greedy52) better client? better message?
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return trace.Wrap(err, "sending SSE request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return trace.Errorf(resp.Status)
	}

	bodyReader := bufio.NewReader(resp.Body)
	endpointMessage, err := mcputils.ReadMessageFromSSEBody(ctx, bodyReader)
	if err != nil {
		return trace.Wrap(err, "reading endpoint response")
	}
	if endpointMessage.Event != "endpoint" {
		return trace.BadParameter("expecting endpoint event, got %s", endpointMessage.Event)
	}
	endpointURI, err := baseURI.Parse(endpointMessage.Data)
	if err != nil {
		return trace.Wrap(err, "parsing endpoint response")
	}
	if sessionID := endpointURI.Query().Get("sessionId"); sessionID != "" {
		session.externalSessionID = sessionID
	}
	session.logger.DebugContext(ctx, "Received endpoint from server", "endpoint", logutils.StringerAttr(endpointURI))

	clientResponseWriter := mcputils.NewStdioMessageWriter(utils.NewSyncWriter(sessionCtx.ClientConn))
	go func() {
		for {
			message, err := mcputils.ReadMessageFromSSEBody(ctx, bodyReader)
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					session.logger.WarnContext(session.parentCtx, "Failed to read SSE response from server", "error", err)
				}
				return
			}

			if message.Event != "message" {
				session.logger.WarnContext(session.parentCtx, "Expected message event, got %s", message.Event)
				continue
			}

			// TODO(greedy52) do it properly instead of the assumption below.
			var response mcputils.JSONRPCResponse
			if err := json.Unmarshal([]byte(message.Data), &response); err != nil {
				session.logger.WarnContext(session.parentCtx, "Failed to unmarshal response from server", "error", err)
				continue
			}

			// Assume it's a notification.
			if response.ID.IsNil() {
				var notification mcputils.JSONRPCNotification
				if err := json.Unmarshal([]byte(message.Data), &notification); err != nil {
					session.logger.WarnContext(session.parentCtx, "Failed to unmarshal notification from server", "error", err)
					continue
				}
				if err := clientResponseWriter.WriteMessage(ctx, notification); err != nil {
					session.logger.WarnContext(session.parentCtx, "Failed to write SSE response to client", "error", err)
				}
				continue
			}

			// Assume it's a response.
			reply := session.processServerResponse(ctx, &response)
			if err := clientResponseWriter.WriteMessage(ctx, reply); err != nil {
				session.logger.WarnContext(session.parentCtx, "Failed to write SSE response to client", "error", err)
			}
		}
	}()

	clientRequestReader, err := mcputils.NewStdioMessageReader(mcputils.StdioMessageReaderConfig{
		SourceReadCloser: sessionCtx.ClientConn,
		Logger:           session.logger.With("stdio", "stdin"),
		ParentContext:    s.cfg.ParentContext,
		OnParseError:     mcputils.ReplyParseError(clientResponseWriter),
		OnRequest: func(ctx context.Context, req *mcputils.JSONRPCRequest) error {
			msg, replyDirection := session.processClientRequest(ctx, req)
			if replyDirection == replyToClient {
				return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msg))
			}

			return trace.Wrap(postSSERequest(ctx, endpointURI, msg))
		},
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			session.processClientNotification(ctx, notification)
			return trace.Wrap(postSSERequest(ctx, endpointURI, notification))
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	session.emitStartEvent(session.parentCtx)
	defer session.emitEndEvent(session.parentCtx)
	clientRequestReader.Run(ctx)
	return nil
}

func postSSERequest(ctx context.Context, endpoint *url.URL, msg mcp.JSONRPCMessage) error {
	// error should not happen.
	data, err := json.Marshal(msg)
	if err != nil {
		return trace.Wrap(err, "marshalling message")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err, "building SSE request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return trace.Wrap(err, "sending SSE request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return trace.BadParameter("SSE request returned %s", resp.Status)
	}
	return nil
}
