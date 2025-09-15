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

package mcputils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerMessageProcessor defines an interface that process JSON RPC responses
// and notifications.
type ServerMessageProcessor interface {
	// ProcessResponse process a response and returns the message for client.
	ProcessResponse(context.Context, *JSONRPCResponse) mcp.JSONRPCMessage
	// ProcessNotification process a notification and returns the message for client.
	ProcessNotification(context.Context, *JSONRPCNotification) mcp.JSONRPCMessage
}

// ReplaceHTTPResponse handles replacing the MCP server response for the
// streamable HTTP transport.
//
// https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http
func ReplaceHTTPResponse(ctx context.Context, resp *http.Response, processor ServerMessageProcessor) error {
	// Nothing to replace.
	if resp.StatusCode != http.StatusOK || resp.ContentLength == 0 {
		return nil
	}

	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return trace.Wrap(err)
	}
	switch mediaType {
	case "application/json":
		// Single response.
		respBody, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPRequestSize)
		if err != nil {
			return trace.Wrap(err)
		}
		respFromServer, err := unmarshalResponse(string(respBody))
		if err != nil {
			return trace.Wrap(err)
		}
		respToClient := processor.ProcessResponse(ctx, respFromServer)
		respToClientAsBody, err := json.Marshal(respToClient)
		if err != nil {
			return trace.Wrap(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(respToClientAsBody))
		resp.ContentLength = int64(len(respToClientAsBody))
		return nil

	case "text/event-stream":
		// Multiple messages (response or notification) can be sent through SSE.
		// Instead of reading all messages then replacing them, here we replace
		// the body with a reader that process the event one a time.
		resp.Body = &httpSSEResponseReplacer{
			ctx:               ctx,
			SSEResponseReader: NewSSEResponseReader(resp.Body),
			processor:         processor,
		}
		return nil
	default:
		return trace.BadParameter("unsupported response type %s", mediaType)
	}
}

type httpSSEResponseReplacer struct {
	*SSEResponseReader
	ctx       context.Context
	processor ServerMessageProcessor
	buf       []byte
}

func (r *httpSSEResponseReplacer) Read(p []byte) (int, error) {
	if len(r.buf) != 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	msg, err := r.ReadMessage(r.ctx)
	if err != nil {
		if utils.IsOKNetworkError(err) {
			return 0, io.EOF
		}
		return 0, trace.Wrap(err)
	}

	var base BaseJSONRPCMessage
	if err := json.Unmarshal([]byte(msg), &base); err != nil {
		return 0, trace.Wrap(err)
	}

	var respToClient mcp.JSONRPCMessage
	switch {
	case base.IsResponse():
		respToClient = r.processor.ProcessResponse(r.ctx, base.MakeResponse())
	case base.IsNotification():
		respToClient = r.processor.ProcessNotification(r.ctx, base.MakeNotification())
	default:
		return 0, trace.BadParameter("message is not a response or a notification")
	}

	respToSendAsBody, err := json.Marshal(respToClient)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Convert to SSE.
	e := event{
		name: sseEventMessage,
		data: respToSendAsBody,
	}
	r.buf = e.marshal()
	return r.Read(p)
}
